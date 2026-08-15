package containerlab

import (
	"embed"
	"fmt"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

const ModuleName = "containerlab"

const ClabOutputFile = "topo.yaml"
const ScriptFile = "containerlab.sh"

const ClabNetworkNameParamName = "_clab_networkName"
const ClabImageParamName = "image"
const ClabKindParamName = "kind"
const ClabBindMountsParamName = "_clab_bindMounts"
const ClabSrcEndpointParamName = "_clab_src_endpoint"
const ClabDstEndpointParamName = "_clab_dst_endpoint"

const ClabYamlFormatName = "_clabYaml"
const ClabCmdFormatName = "clabCmd"

// ClabCopyFormatName decorates the copy commands like any other exec entry. Both
// blocks carry their separator in front rather than behind, because either one
// can be the only one there: a block that ended with a newline would leave a
// blank line behind whenever it came last.
const ClabCopyFormatName = "clabCopy"
const ClabLinkFormatName = "_clabLink"

const NetworkClassName = "_clabNetwork"
const NodeClassName = "_clabNode"
const SwitchNodeClassName = "_clabSwitchNode"

// WorkerGroupClassName carries the topology file when the topology declares
// placement units, so that each machine gets one it can deploy on its own.
const WorkerGroupClassName = "_clabWorkerGroup"

// Ready-made classes a topology opts into with use:. containerlab refuses to
// deploy when a bridge node's bridge does not exist, and its deployment check
// runs before any stage, so nothing inside the lab can create it. These spell
// out the usual command for each kind of bridge.
//
// They are not applied automatically. Which command creates a bridge is not
// decided by the kind - the same ovs-bridge may be provisioned by Ansible, need
// sudo, or live in another OVS database - so the choice belongs to the
// topology. A topology that provisions its bridges some other way names neither
// class and writes its own clab_bridge_setup template, or none at all.
//
// The names carry no underscore because they are meant to be written by users.
const OvsBridgeSetupClassName = "clabOvsBridgeSetup"
const LinuxBridgeSetupClassName = "clabLinuxBridgeSetup"

// BridgeSetupConfigName is the config template name both classes above define,
// and the one a topology overrides or supplies itself. Aggregate it with
// {{ .nodes_clab_bridge_setup }}.
const BridgeSetupConfigName = "clab_bridge_setup"

// BridgeSetupFile is where those blocks end up. The module writes the script
// itself rather than leaving a topology to assemble it: what goes in it is the
// module's own doing, and a topology that says use: [clabOvsBridgeSetup] has
// said everything it needs to.
const BridgeSetupFile = "setup-bridges.sh"

// BridgeCleanupConfigName is the same block the other way round: what the entry
// script runs once the lab is down.
const BridgeCleanupConfigName = "clab_bridge_cleanup"
const InterfaceClassName = "_clabInterface"
const ConnectionClassName = "_clabConnection"

// Options are the containerlab module's own settings, written under
// module_config.containerlab.
type Options struct {
	// ManagementNetwork attaches containerlab's management network to every
	// node, the way containerlab does on its own. It is off by default: that
	// network is a second path between every pair of nodes, so a reachability
	// test that should have failed can pass through it. It also takes eth0,
	// which a topology may want for a data interface.
	//
	// Turn it on for clab exec and the clab-* names.
	ManagementNetwork bool `yaml:"management_network"`
	// GenerateScripts writes an entry point script beside the lab, so that it
	// can be brought up without knowing this platform's invocation. Off by
	// default: the commands are short enough to type, and a topology that does
	// not want the extra file should not get one.
	GenerateScripts bool `yaml:"generate_scripts"`
}

//go:embed templates/*
var templates embed.FS

type ClabModule struct {
	*types.StandardModule
}

// Capabilities provided by this module.
var (
	_ types.Module             = (*ClabModule)(nil)
	_ types.ObjectClassifier   = (*ClabModule)(nil)
	_ types.ParameterProvider  = (*ClabModule)(nil)
	_ types.RequirementChecker = (*ClabModule)(nil)
	_ types.ParameterGenerator = (*ClabModule)(nil)
)

func NewModule() types.Module {
	return &ClabModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *ClabModule) UpdateConfig(cfg *types.Config) error {
	var opts Options
	if _, err := cfg.DecodeModuleConfig("containerlab", &opts); err != nil {
		return err
	}

	// add file format
	formatStyle := &types.FormatStyle{
		Name:                ClabYamlFormatName,
		MergeBlockSeparator: ", ",
	}
	cfg.AddFormatStyle(formatStyle)
	formatStyle = &types.FormatStyle{
		Name:                ClabCmdFormatName,
		FormatLinePrefix:    "      - ",
		FormatBlockPrefix:   "\n",
		MergeBlockSeparator: "\n",
	}
	cfg.AddFormatStyle(formatStyle)
	formatStyle = &types.FormatStyle{
		Name:                ClabCopyFormatName,
		FormatLinePrefix:    "      - ",
		FormatBlockPrefix:   "\n",
		MergeBlockSeparator: "\n",
	}
	cfg.AddFormatStyle(formatStyle)

	// Links are joined without a separator: each connection block already ends with
	// a newline, so the default merge separator would insert a blank line between
	// entries.
	formatStyle = &types.FormatStyle{
		Name:                ClabLinkFormatName,
		MergeBlockSeparator: types.EmptySeparator,
	}
	cfg.AddFormatStyle(formatStyle)

	// One topology file, or one per machine. A lab is deployed to a single
	// machine, so a topology spread over several of them needs a file each;
	// with no placement units declared there is one machine and one file.
	//
	// The choice is made here because a FileDefinition carries a fixed scope
	// and the model does not exist yet. What can be read at this point is the
	// topology's own configuration, which is loaded before the modules are.
	_, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName)

	scope := types.ClassTypeNetwork
	if perWorker {
		scope = types.ClassTypeGroup
	}
	// With the output split per module the topology file moves into a directory
	// of its own. containerlab resolves a relative bind against the directory
	// holding the topology file, so what has to change with it is the bind
	// paths - see generateBindMountParams.
	subdir := ""
	if cfg.GlobalSettings.SplitModuleOutput {
		subdir = ModuleName
	}
	cfg.AddFileDefinition(&types.FileDefinition{
		Name:   ClabOutputFile,
		Path:   "",
		Scope:  scope,
		Subdir: subdir,
	})

	// The topology file is owned by whichever object it is scoped to: the
	// network as a whole, or one worker group. Only the group knows which nodes
	// and which links belong to it, and it already answers that - a connection
	// with one end outside the group is not one of its children, which is
	// exactly the set a machine can wire with veth.
	ct1 := &types.ConfigTemplate{File: ClabOutputFile}
	topoTemplate := "templates/topo.yaml.network_clab_topo"
	if perWorker {
		topoTemplate = "templates/topo.yaml.group_clab_topo"
	}
	bytes, err := templates.ReadFile(topoTemplate)
	if err != nil {
		return err
	}
	ct1.Template = []string{string(bytes)}

	// The bridges a topology names have to exist before containerlab will
	// deploy it, so a topology that asks for one gets a script that makes them.
	// The module writes it rather than leaving a topology to assemble it, and
	// only when some node class pulls in one of the bridge setup classes -
	// which is knowable here, from the topology's own configuration.
	owns := []*types.ConfigTemplate{ct1}
	if usesBridgeSetup(cfg) {
		cfg.AddFileDefinition(&types.FileDefinition{
			Name:       BridgeSetupFile,
			Path:       "",
			Scope:      scope,
			Subdir:     subdir,
			Executable: true,
		})
		bridgeScript, err := templates.ReadFile("templates/setup-bridges.sh.clab_bridge_script")
		if err != nil {
			return err
		}
		owns = append(owns, &types.ConfigTemplate{
			File:     BridgeSetupFile,
			Template: []string{string(bridgeScript)},
		})
	}

	// The entry script joins the same class, rather than getting one of its
	// own: a class of its own is a class no group carries a label for, and the
	// script would silently not be written for a multi-machine lab.
	if opts.GenerateScripts {
		entry, err := entryScriptTemplate(cfg, scope, subdir)
		if err != nil {
			return err
		}
		owns = append(owns, entry)
	}

	if perWorker {
		// Not AddModuleGroupClassLabel: that would give the topology file to
		// every group, including the ones that only share parameters.
		// ClassifyObjects picks the worker groups out.
		cfg.AddGroupClass(&types.GroupClass{
			Name:            WorkerGroupClassName,
			ConfigTemplates: owns,
		})
	} else {
		cfg.AddNetworkClass(&types.NetworkClass{
			Name:            NetworkClassName,
			ConfigTemplates: owns,
		})
	}

	// add node class
	// Both blocks are named clab_cmds and merge into one exec section, each
	// appearing only when it has something to say - which is what keeps an empty
	// startup from leaving an empty command behind. The copies come first: a
	// command the author wrote may use a file that is only there once it has
	// been copied.
	ctCopies := &types.ConfigTemplate{
		Name:           "clab_copies",
		Format:         ClabCopyFormatName,
		RequiredParams: []string{"values_clab_copy_entry"},
	}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_copies")
	if err != nil {
		return err
	}
	ctCopies.Template = []string{string(bytes)}

	ct1 = &types.ConfigTemplate{
		Name:           "clab_cmds",
		Format:         ClabCmdFormatName,
		Depends:        []string{"startup"},
		RequiredParams: []string{"self_startup"},
	}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_cmd")
	if err != nil {
		return err
	}
	ct1.Template = []string{string(bytes)}

	// binds section - only output if values_clab_bind_entry exists
	ct2 := &types.ConfigTemplate{
		Name:           "clab_topo_binds",
		RequiredParams: []string{"values_clab_bind_entry"},
	}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_topo_binds")
	if err != nil {
		return err
	}
	ct2.Template = []string{string(bytes)}

	// exec section - only output if startup exists (matches original {{ if .self_startup }})
	// The two blocks that can fill the section, joined into one. Either alone is
	// reason enough to write the section, and whether either has anything to say
	// is known only once both are rendered - so the section asks whether this
	// came out empty rather than trying to work it out beforehand.
	ctExecBody := &types.ConfigTemplate{
		Name:    "clab_exec_body",
		Depends: []string{"clab_copies", "clab_cmds"},
	}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_exec_body")
	if err != nil {
		return err
	}
	ctExecBody.Template = []string{string(bytes)}

	// The section appears when there is something to run, which is not the same
	// as the topology having written startup commands: a file provided by copy
	// puts its own command here.
	ct3 := &types.ConfigTemplate{
		Name:           "clab_topo_exec",
		RequiredParams: []string{"self_clab_exec_body"},
		Depends:        []string{"clab_exec_body"},
	}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_topo_exec")
	if err != nil {
		return err
	}
	ct3.Template = []string{string(bytes)}

	// main topo entry - references binds and exec sections
	ct4 := &types.ConfigTemplate{
		Name:    "clab_topo",
		Depends: []string{"clab_topo_binds", "clab_topo_exec"},
	}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_topo")
	if err != nil {
		return err
	}
	// The template carries network-mode: none. Asking for the management
	// network means taking that line out again, which is why it sits on a line
	// of its own.
	nodeTopo := string(bytes)
	if opts.ManagementNetwork {
		nodeTopo = strings.ReplaceAll(nodeTopo, "      network-mode: none\n", "")
	}
	ct4.Template = []string{nodeTopo}

	// What the entry script needs from each node: the lab's own teardown
	// commands, and the files to copy out. Both are aggregated by the script,
	// which is the module's own file - a topology never names these blocks.
	ctTeardown, err := readTemplate("templates/teardown.node_clab_teardown", &types.ConfigTemplate{
		Name:           "clab_teardown",
		Depends:        []string{"teardown"},
		RequiredParams: []string{"self_teardown"},
	})
	if err != nil {
		return err
	}
	ctCollect, err := readTemplate("templates/collect.node_clab_collect", &types.ConfigTemplate{
		Name:           "clab_collect",
		RequiredParams: []string{"values_clab_collect_entry"},
	})
	if err != nil {
		return err
	}

	nodeClass := &types.NodeClass{
		Name:            NodeClassName,
		Parameters:      []string{"clab_binds", "clab_copies", "clab_collects"},
		ConfigTemplates: []*types.ConfigTemplate{ct1, ct2, ct3, ct4, ctCopies, ctExecBody, ctTeardown, ctCollect},
	}
	cfg.AddNodeClass(nodeClass)
	// Not AddModuleNodeClassLabel: which of the two node classes a node gets is
	// decided per node in ClassifyObjects.

	// A switch node is a shared L2 domain the platform realizes itself, so it
	// carries no image, no bind mounts and no commands - only the kind, whose
	// value comes from the user untouched.
	ct6 := &types.ConfigTemplate{Name: "clab_topo"}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_switch")
	if err != nil {
		return err
	}
	ct6.Template = []string{string(bytes)}

	cfg.AddNodeClass(&types.NodeClass{
		Name:            SwitchNodeClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct6},
	})

	// A bridge the module made is a bridge the module takes away: the setup
	// script had no counterpart, so a lab that was destroyed left its bridges
	// behind.
	for _, setup := range []struct{ className, file, cleanupFile string }{
		{OvsBridgeSetupClassName, "templates/setup.node_clab_ovs_bridge", "templates/setup.node_clab_ovs_bridge_cleanup"},
		{LinuxBridgeSetupClassName, "templates/setup.node_clab_linux_bridge", "templates/setup.node_clab_linux_bridge_cleanup"},
	} {
		bytes, err = templates.ReadFile(setup.file)
		if err != nil {
			return err
		}
		cleanup, err := templates.ReadFile(setup.cleanupFile)
		if err != nil {
			return err
		}
		cfg.AddNodeClass(&types.NodeClass{
			Name: setup.className,
			ConfigTemplates: []*types.ConfigTemplate{
				{Name: BridgeSetupConfigName, Template: []string{string(bytes)}},
				{Name: BridgeCleanupConfigName, Template: []string{string(cleanup)}},
			},
		})
	}

	// add connection class emitting one "links:" entry per connection.
	// The endpoint names come from GenerateParameters; rendering lives in the
	// template so that the list can be aggregated by whichever object owns the
	// output file (see the network class below).
	// RequiredLink: the entry asks containerlab to lay a wire, so it is written
	// only where the platform lays one. A connection the generated configuration
	// builds instead - a tunnel between two ends that already exist - reaches the
	// same pair of nodes without any wiring for containerlab to do.
	ct5 := &types.ConfigTemplate{Name: "clab_link", NamespaceFormat: ClabLinkFormatName, RequiredLink: true}
	bytes, err = templates.ReadFile("templates/topo.yaml.connection_clab_link")
	if err != nil {
		return err
	}
	ct5.Template = []string{string(bytes)}

	connectionClass := &types.ConnectionClass{
		Name:            ConnectionClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct5},
	}
	cfg.AddConnectionClass(connectionClass)
	m.AddModuleConnectionClassLabel(ConnectionClassName)

	// add param_rule for bind mounts using Value class
	bindsParamRule := &types.ParameterRule{
		Name:      "clab_binds",
		Mode:      types.ParameterRuleModeAttach,
		Generator: "clab.filemounts",
		ConfigTemplates: []*types.ConfigTemplate{
			{
				Name:     "clab_bind_entry",
				Template: []string{"      - {{ .source }}:{{ .target }}{{ .mode }}"},
			},
		},
	}
	cfg.AddParameterRule(bindsParamRule)

	collectEntry, err := templates.ReadFile("templates/collect.value_clab_collect_entry")
	if err != nil {
		return err
	}
	cfg.AddParameterRule(&types.ParameterRule{
		Name:      "clab_collects",
		Mode:      types.ParameterRuleModeAttach,
		Generator: "clab.collectfiles",
		ConfigTemplates: []*types.ConfigTemplate{
			{Name: "clab_collect_entry", Template: []string{string(collectEntry)}},
		},
	})

	copiesEntry, err := templates.ReadFile("templates/topo.yaml.value_clab_copy_entry")
	if err != nil {
		return err
	}
	cfg.AddParameterRule(&types.ParameterRule{
		Name:      "clab_copies",
		Mode:      types.ParameterRuleModeAttach,
		Generator: "clab.copyfiles",
		ConfigTemplates: []*types.ConfigTemplate{
			{Name: "clab_copy_entry", Template: []string{string(copiesEntry)}},
		},
	})

	return nil
}

// ClassifyObjects gives every node one of the module's two node classes. The
// choice cannot be made through AddModuleNodeClassLabel, which applies one class
// to all nodes before this hook runs.
func (m *ClabModule) ClassifyObjects(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			node.AddModuleClassLabels(SwitchNodeClassName)
		} else {
			node.AddModuleClassLabels(NodeClassName)
		}
	}
	// Only the groups standing for a machine get a topology file. The others
	// group nodes for some purpose of the topology's own - an AS, an area - and
	// nothing is deployed to them.
	for _, group := range nm.Groups {
		if cfg.IsWorkerGroup(group) {
			group.AddModuleClassLabels(WorkerGroupClassName)
		}
	}
	return nil
}

func (m *ClabModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {

	// set network name
	nm.AddParam(ClabNetworkNameParamName, cfg.Name)

	// Supply the endpoint names of every connection. The "links:" list itself is
	// rendered by the connection config template, not assembled here: a single
	// network-wide string could not be split per output file, which host-scoped
	// topologies need. Connections to virtual nodes are dropped by the template
	// condition, so they are not special-cased here.
	for _, conn := range nm.Connections {
		conn.AddParam(ClabSrcEndpointParamName, fmt.Sprintf("%s:%s", conn.Src.Node.Name, conn.Src.Name))
		conn.AddParam(ClabDstEndpointParamName, fmt.Sprintf("%s:%s", conn.Dst.Node.Name, conn.Dst.Name))
	}

	// Note: bind mounts are now generated through Value class mechanism
	// (param_rule "clab_binds" with generator "clab.filemounts")

	return nil
}

// GenerateValueParameters implements ParameterGenerator interface
// Generates parameter sets for Value objects based on generator name
func (m *ClabModule) GenerateValueParameters(
	generatorName string,
	target types.ValueOwner,
	cfg *types.Config,
	nm *types.NetworkModel,
) ([]map[string]string, error) {
	switch generatorName {
	case "filemounts":
		return m.generateFilemountParams(target, cfg, nm)
	case "copyfiles":
		return copyFileParams(target, cfg)
	case "collectfiles":
		return generateCollectParams(target, cfg, nm)
	default:
		return nil, fmt.Errorf("unknown generator: %s", generatorName)
	}
}

// generateFilemountParams generates source/target pairs for container bind mounts
func (m *ClabModule) generateFilemountParams(
	target types.ValueOwner,
	cfg *types.Config,
	nm *types.NetworkModel,
) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("filemounts generator requires Node target, got %T", target)
	}

	// Skip virtual nodes
	if !node.IsMaterialised() {
		return nil, nil
	}

	// Get list of files this node will generate
	nodeFiles := node.FilesToGenerate(cfg)
	fileSet := make(map[string]bool)
	for _, file := range nodeFiles {
		fileSet[file] = true
	}

	// containerlab resolves a relative bind against the directory holding the
	// topology file, so every source path is stated from there rather than from
	// the output root.
	adjust := func(srcPath string) (string, error) {
		// With one topology per machine that file sits in the machine's
		// directory - otherwise the machine's own name appears twice.
		if _, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName); perWorker {
			dir, err := node.OutputDir(cfg)
			if err != nil {
				return "", err
			}
			if dir != "" {
				srcPath = strings.TrimPrefix(srcPath, dir+"/")
			}
		}
		// The topology file sits one level down when the output is split.
		if cfg.GlobalSettings.SplitModuleOutput {
			srcPath = "../" + srcPath
		}
		return srcPath, nil
	}

	var results []map[string]string
	for _, fileDef := range cfg.FileDefinitions {
		if fileDef.Path == "" {
			continue
		}

		// Check if this node actually generates this file
		if !fileSet[fileDef.Name] {
			continue
		}

		// A file provided by copy is not bound: it waits in the staging
		// directory, which is bound as a whole below, and is copied to its own
		// path once the container is up.
		if fileDef.GetProvide() == types.ProvideCopy {
			continue
		}

		srcPath, err := node.OutputPath(cfg, fileDef)
		if err != nil {
			return nil, err
		}
		srcPath, err = adjust(srcPath)
		if err != nil {
			return nil, err
		}

		params := map[string]string{
			"source": srcPath,
			"target": fileDef.Path,
			"mode":   "",
		}
		results = append(results, params)
	}

	// The staging directory is read only: it holds what was generated, and the
	// copies the container works with are made from it.
	stagingDir, staged, err := node.StagingDir(cfg)
	if err != nil {
		return nil, err
	}
	if staged {
		srcPath, err := adjust(stagingDir)
		if err != nil {
			return nil, err
		}
		results = append(results, map[string]string{
			"source": srcPath,
			"target": "/" + types.StagingDirName,
			"mode":   ":ro",
		})
	}

	return results, nil
}

func (m *ClabModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	// containerlab keeps eth0 for the management network and refuses a data
	// interface by that name. It says so at deploy time; saying it here means
	// the topology hears about it while it can still be changed.
	var opts Options
	if _, err := cfg.DecodeModuleConfig("containerlab", &opts); err != nil {
		return err
	}
	if opts.ManagementNetwork {
		for _, node := range nm.Nodes {
			if !node.IsMaterialised() || cfg.IsSwitchNode(node) {
				continue
			}
			for _, iface := range node.Interfaces {
				if iface.Name == "eth0" {
					return fmt.Errorf(
						"node %s has an interface named eth0, which containerlab keeps for the "+
							"management network that module_config.containerlab.management_network "+
							"turns on; name the interfaces something else (interfaceclass prefix) "+
							"or leave the management network off", node.Name)
				}
			}
		}
	}

	// A machine is deployed from its own directory, so everything its topology
	// file refers to has to live under that directory. Splitting the output by
	// some other grouping would scatter the node files elsewhere and leave no
	// relative path from the topology to them.
	if _, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName); perWorker {
		if got := cfg.GlobalSettings.OutputGroupClass; got != types.WorkerGroupClassName {
			return fmt.Errorf(
				"a topology split across %s groups needs global.output_group_class: %s so that "+
					"each machine's files sit beside its topo.yaml (it is %q)",
				types.WorkerGroupClassName, types.WorkerGroupClassName, got)
		}
	}

	// node config templates named startup
	flag := false
	for _, nc := range cfg.NodeClasses {
		for _, ct := range nc.ConfigTemplates {
			if ct.Name == "startup" {
				flag = true
			}
		}
	}
	if !flag {
		return fmt.Errorf("node config templates named startup is required")
	}

	// parameter {{ .image }} and {{ .kind }}
	for _, node := range nm.Nodes {
		if !node.IsMaterialised() {
			continue
		}
		// A switch node is not a container, so it has no image. Which kinds go
		// without one is containerlab's business, not ours: the check keys on
		// dot2net's own class so that a new imageless kind needs no change here.
		if !cfg.IsSwitchNode(node) {
			_, err := node.GetParamValue(ClabImageParamName)
			if err != nil {
				return fmt.Errorf("every (non-virtual) node must have {{ .image }} parameter (none for %s)", node.Name)
			}
		}
		if _, err := node.GetParamValue(ClabKindParamName); err != nil {
			return fmt.Errorf("every (non-virtual) node must have {{ .kind }} parameter (none for %s)", node.Name)
		}
	}
	return nil
}

// addEntryScript registers the script that stands in front of this platform's
// own command. It sits where the lab is operated from - the output root, or a
// machine's directory - even when the topology file itself has moved into a
// directory of its own, because the point of it is to be reachable without
// knowing that layout.
func entryScriptTemplate(cfg *types.Config, scope, subdir string) (*types.ConfigTemplate, error) {
	cfg.AddFileDefinition(&types.FileDefinition{
		Name:       ScriptFile,
		Path:       "",
		Scope:      scope,
		Executable: true,
	})
	bytes, err := templates.ReadFile("templates/containerlab.sh.entry")
	if err != nil {
		return nil, err
	}
	// %%TOPO%% is filled in here rather than by the template engine: the path is
	// known at registration and the script is otherwise the same for every lab,
	// so there is nothing for the engine to do.
	path := ClabOutputFile
	if subdir != "" {
		path = subdir + "/" + ClabOutputFile
	}
	script := strings.ReplaceAll(string(bytes), "%%TOPO%%", path)
	script = strings.ReplaceAll(script, "%%COLLECT%%", types.CollectDirName)
	return &types.ConfigTemplate{File: ScriptFile, Template: []string{script}}, nil
}

// copyFileParams turns the node's staged files into the source and target a
// copy command names. The command itself lives in the module's template, since
// only the platform knows where its commands are written.
func copyFileParams(target types.ValueOwner, cfg *types.Config) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("copyfiles generator requires Node target, got %T", target)
	}
	if !node.IsMaterialised() {
		return nil, nil
	}
	copies, err := types.StagedCopies(cfg, node)
	if err != nil {
		return nil, err
	}
	var results []map[string]string
	for _, c := range copies {
		results = append(results, map[string]string{
			"source": c.Source,
			"target": c.Target,
			"dir":    c.Dir,
		})
	}
	return results, nil
}

// usesBridgeSetup reports whether any node class in the topology pulls in one of
// the bridge setup classes. It reads the topology's own configuration, which is
// loaded before the modules are, so the answer is available while the module is
// still deciding what to register.
func usesBridgeSetup(cfg *types.Config) bool {
	for _, nc := range cfg.NodeClasses {
		for _, used := range nc.Use {
			if used == OvsBridgeSetupClassName || used == LinuxBridgeSetupClassName {
				return true
			}
		}
	}
	return false
}

// readTemplate fills a config template in from the module's own files.
func readTemplate(path string, ct *types.ConfigTemplate) (*types.ConfigTemplate, error) {
	bytes, err := templates.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ct.Template = []string{string(bytes)}
	return ct, nil
}

// generateCollectParams names the files to copy out of a node.
func generateCollectParams(target types.ValueOwner, cfg *types.Config, nm *types.NetworkModel) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("collectfiles generator requires Node target, got %T", target)
	}
	targets, err := types.CollectTargets(cfg, nm)
	if err != nil {
		return nil, err
	}
	var results []map[string]string
	for _, t := range targets {
		if t.Node != node.Name {
			continue
		}
		results = append(results, map[string]string{"device": t.Node, "path": t.ContainerPath})
	}
	return results, nil
}
