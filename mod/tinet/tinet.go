package tinet

import (
	"embed"
	"fmt"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

const ModuleName = "tinet"

const TinetOutputFile = "spec.yaml"
const ScriptFile = "tinet.sh"
const ScriptClassName = "_tinetScript"

const TinetNetworkNameParamName = "_tn_networkName"
const TinetImageParamName = "image"
const TinetBindMountsParamName = "_tn_bindMounts"

const TinetYamlFormatName = "_tinetYaml"

// TinetSwitchFormatName joins the switches: entries one per line. The general
// _tinetYaml style joins with ", " because it also renders the inline
// interfaces list, which would run the switch entries together.
const TinetSwitchFormatName = "_tinetSwitchList"
const SpecCmdFormatName = "tinetSpecCmd"

// const TinetVtyshCLIFormatName = "tinetVtyshCLI"

const NetworkClassName = "_tinetNetwork"

// WorkerGroupClassName carries the spec file when the scenario declares
// placement units, the way NetworkClassName carries it when it does not.
const WorkerGroupClassName = "_tinetWorkerGroup"

const NodeClassName = "_tinetNode"
const InterfaceClassName = "_tinetInterface"
const SwitchNodeClassName = "_tinetSwitch"
const SwitchInterfaceClassName = "_tinetSwitchInterface"

//go:embed templates/*
var templates embed.FS

type TinetModule struct {
	*types.StandardModule
}

// Capabilities provided by this module.
var (
	_ types.Module             = (*TinetModule)(nil)
	_ types.ObjectClassifier   = (*TinetModule)(nil)
	_ types.ParameterProvider  = (*TinetModule)(nil)
	_ types.RequirementChecker = (*TinetModule)(nil)
	_ types.ParameterGenerator = (*TinetModule)(nil)
)

func NewModule() types.Module {
	return &TinetModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *TinetModule) UpdateConfig(cfg *types.Config) error {
	// add file format
	formatStyle := &types.FormatStyle{
		Name:                TinetYamlFormatName,
		MergeBlockSeparator: ", ",
	}
	cfg.AddFormatStyle(formatStyle)
	formatStyle = &types.FormatStyle{
		Name:                TinetSwitchFormatName,
		MergeBlockSeparator: "\n",
	}
	cfg.AddFormatStyle(formatStyle)
	formatStyle = &types.FormatStyle{
		Name:                SpecCmdFormatName,
		FormatLinePrefix:    "      - cmd: ",
		MergeBlockSeparator: "\n",
	}
	cfg.AddFormatStyle(formatStyle)

	// One spec file, or one per machine. A lab is deployed to a single machine,
	// so a topology spread over several of them needs a file each; with no
	// placement units declared there is one machine and one file.
	//
	// The choice is made here because a FileDefinition carries a fixed scope
	// and the model does not exist yet. What can be read at this point is the
	// scenario's own configuration, which is loaded before the modules are.
	_, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName)

	// add file definition
	scope := types.ClassTypeNetwork
	if perWorker {
		scope = types.ClassTypeGroup
	}
	subdir := ""
	if cfg.GlobalSettings.SplitModuleOutput {
		subdir = ModuleName
	}
	fileDef := &types.FileDefinition{
		Name:   TinetOutputFile,
		Path:   "",
		Scope:  scope,
		Subdir: subdir,
	}
	cfg.AddFileDefinition(fileDef)

	// add the class that owns the spec file
	// The switches: section is a separate block so that it disappears entirely
	// when the topology has no switch node: RequiredParams is satisfied only by
	// a parameter that is present and non-empty.
	ctSwitches := &types.ConfigTemplate{
		Name:           "tn_switches",
		RequiredParams: []string{"nodes_tn_switch"},
	}
	bytes, err := templates.ReadFile("templates/spec.yaml.network_tn_switches")
	if err != nil {
		return err
	}
	ctSwitches.Template = []string{string(bytes)}

	ct1 := &types.ConfigTemplate{File: TinetOutputFile, Depends: []string{"tn_switches"}}
	bytes, err = templates.ReadFile("templates/spec.yaml.network")
	if err != nil {
		return err
	}
	ct1.Template = []string{string(bytes)}

	// The spec file is owned by whichever object it is scoped to: the network as
	// a whole, or one worker group. Only the group knows which nodes and which
	// links belong to it, and it already answers that - a connection with one
	// end outside the group is not one of its children, which is exactly the
	// set a machine can wire itself. The template text is the same either way:
	// it aggregates over "the nodes of this object", and the object differs.
	if perWorker {
		// Not AddModuleGroupClassLabel: that would give the spec file to every
		// group, including the ones that only share parameters. ClassifyObjects
		// picks the worker groups out.
		cfg.AddGroupClass(&types.GroupClass{
			Name:            WorkerGroupClassName,
			ConfigTemplates: []*types.ConfigTemplate{ctSwitches, ct1},
		})
	} else {
		cfg.AddNetworkClass(&types.NetworkClass{
			Name:            NetworkClassName,
			ConfigTemplates: []*types.ConfigTemplate{ctSwitches, ct1},
		})
	}

	var opts Options
	if _, err := cfg.DecodeModuleConfig(ModuleName, &opts); err != nil {
		return err
	}
	if opts.GenerateScripts {
		if err := addEntryScript(cfg, scope, subdir); err != nil {
			return err
		}
	}

	// add node class
	ct1 = &types.ConfigTemplate{
		Name:           "tn_cmds",
		Format:         SpecCmdFormatName,
		Depends:        []string{"startup"},
		RequiredParams: []string{"self_startup"},
	}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_cmd")
	if err != nil {
		return err
	}
	ct1.Template = []string{string(bytes)}

	ct2 := &types.ConfigTemplate{Name: "tn_spec"}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_spec")
	if err != nil {
		return err
	}
	ct2.Template = []string{string(bytes)}

	// The copies run before the scenario's own commands: a command the author
	// wrote may use a file that is only there once it has been copied.
	ctCopies := &types.ConfigTemplate{
		Name:           "tn_copies",
		Format:         SpecCmdFormatName,
		RequiredParams: []string{"values_tinet_copy_entry"},
	}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_copies")
	if err != nil {
		return err
	}
	ctCopies.Template = []string{string(bytes)}

	ct3 := &types.ConfigTemplate{
		Name:    "tn_config",
		Depends: []string{"tn_copies", "tn_cmds"},
		Blocks: types.BlocksConfig{
			After: []string{"self_tn_copies", "self_tn_cmds"},
		},
	}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_config")
	if err != nil {
		return err
	}
	ct3.Template = []string{string(bytes)}

	nodeClass := &types.NodeClass{
		Name:            NodeClassName,
		Parameters:      []string{"tinet_binds", "tinet_copies"},
		ConfigTemplates: []*types.ConfigTemplate{ct1, ct2, ct3, ctCopies},
	}
	cfg.AddNodeClass(nodeClass)
	// Not AddModuleNodeClassLabel: ClassifyObjects picks between this class and
	// the switch one per node.

	// A switch is a shared L2 domain TiNET realizes as an OVS bridge of its own,
	// so it belongs in the switches: section rather than in nodes:.
	ctSwitch := &types.ConfigTemplate{Name: "tn_switch", Format: TinetSwitchFormatName}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_switch")
	if err != nil {
		return err
	}
	ctSwitch.Template = []string{string(bytes)}
	cfg.AddNodeClass(&types.NodeClass{
		Name:            SwitchNodeClassName,
		ConfigTemplates: []*types.ConfigTemplate{ctSwitch},
	})

	// add interface class
	// RequiredLink: this template tells TiNET to create a veth pair. It must not be
	// emitted for a connection that models a shared segment without an actual wire
	// (e.g. bridges joined by a VXLAN overlay), or TiNET would create an interface
	// whose name collides with the device the node config creates itself.
	ct1 = &types.ConfigTemplate{Name: "tn_spec", Format: TinetYamlFormatName, RequiredLink: true}
	bytes, err = templates.ReadFile("templates/spec.yaml.interface_spec")
	if err != nil {
		return err
	}
	ct1.Template = []string{string(bytes)}
	interfaceClass := &types.InterfaceClass{
		Name:            InterfaceClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct1},
	}
	cfg.AddInterfaceClass(interfaceClass)
	// Not AddModuleInterfaceClassLabel: ClassifyObjects decides which of the two
	// interface classes applies, and gives the switch's own interfaces neither.

	// An interface facing a switch attaches to it by name instead of naming a
	// peer interface. It shares the tn_spec config name so that both kinds
	// aggregate into the same interfaces: list.
	ctSwitchIface := &types.ConfigTemplate{Name: "tn_spec", Format: TinetYamlFormatName, RequiredLink: true}
	bytes, err = templates.ReadFile("templates/spec.yaml.interface_switch_spec")
	if err != nil {
		return err
	}
	ctSwitchIface.Template = []string{string(bytes)}
	cfg.AddInterfaceClass(&types.InterfaceClass{
		Name:            SwitchInterfaceClassName,
		ConfigTemplates: []*types.ConfigTemplate{ctSwitchIface},
	})

	// add param_rule for bind mounts using Value class
	bindsParamRule := &types.ParameterRule{
		Name:      "tinet_binds",
		Mode:      types.ParameterRuleModeAttach,
		Generator: "tinet.filemounts",
		ConfigTemplates: []*types.ConfigTemplate{
			{
				Name:     "tinet_bind_entry",
				Template: []string{"{{ .source }}:{{ .target }}{{ .mode }}"},
				Format:   TinetYamlFormatName,
			},
		},
	}
	cfg.AddParameterRule(bindsParamRule)

	copiesEntry, err := templates.ReadFile("templates/spec.yaml.value_tn_copy_entry")
	if err != nil {
		return err
	}
	cfg.AddParameterRule(&types.ParameterRule{
		Name:      "tinet_copies",
		Mode:      types.ParameterRuleModeAttach,
		Generator: "tinet.copyfiles",
		ConfigTemplates: []*types.ConfigTemplate{
			{Name: "tinet_copy_entry", Template: []string{string(copiesEntry)}},
		},
	})

	return nil
}

func (m TinetModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {

	// set network name
	nm.AddParam(TinetNetworkNameParamName, cfg.Name)

	// Note: bind mounts are now generated through Value class mechanism
	// (param_rule "tinet_binds" with generator "tinet.filemounts")

	return nil
}

// GenerateValueParameters implements ParameterGenerator interface
// Generates parameter sets for Value objects based on generator name
func (m *TinetModule) GenerateValueParameters(
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
	default:
		return nil, fmt.Errorf("unknown generator: %s", generatorName)
	}
}

// generateFilemountParams generates source/target pairs for container bind mounts
func (m *TinetModule) generateFilemountParams(
	target types.ValueOwner,
	cfg *types.Config,
	nm *types.NetworkModel,
) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("filemounts generator requires Node target, got %T", target)
	}

	// Skip virtual nodes
	if node.IsVirtual() {
		return nil, nil
	}

	// Get list of files this node will generate
	nodeFiles := node.FilesToGenerate(cfg)
	fileSet := make(map[string]bool)
	for _, file := range nodeFiles {
		fileSet[file] = true
	}

	// Every mount path is stated the same way, so the adjustments live in one
	// place rather than beside each source.
	adjust := func(srcPath string) (string, error) {
		// With one spec file per machine, that file sits in the machine's
		// directory and the lab is brought up from there, so the mount path has
		// to start at that directory rather than at the output root - otherwise
		// the machine's own name appears twice.
		if _, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName); perWorker {
			dir, err := node.OutputDir(cfg)
			if err != nil {
				return "", err
			}
			if dir != "" {
				srcPath = strings.TrimPrefix(srcPath, dir+"/")
			}
		}
		// Docker refuses a relative bind source - it reads one as a volume name
		// - and TiNET passes the string into `docker run -v` untouched. The
		// output of `tinet up` is meant to be piped to a shell, so $PWD is
		// expanded there, against the directory the lab is brought up from.
		// That is where the spec file sits, which is what the paths above are
		// relative to.
		// $PWD is where the lab is brought up from, which is where the spec
		// file sits - one level down when the output is split.
		if cfg.GlobalSettings.SplitModuleOutput {
			srcPath = "../" + srcPath
		}
		return "$PWD/" + srcPath, nil
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

		// A file provided by copy is not mounted at its own path: it waits in
		// the staging directory, mounted below, and is copied once the
		// container is up.
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

		results = append(results, map[string]string{
			"source": srcPath,
			"target": fileDef.Path,
			"mode":   "",
		})

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

// ClassifyObjects assigns the node and interface classes that depend on which
// nodes are switches, which AddModuleNodeClassLabel cannot express: it applies
// one class to every object before this hook runs.
func (m TinetModule) ClassifyObjects(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			node.AddModuleClassLabels(SwitchNodeClassName)
			// A switch's own interfaces produce nothing: the attachment is
			// declared from the other end, by name.
			continue
		}
		node.AddModuleClassLabels(NodeClassName)
		for _, iface := range node.Interfaces {
			if iface.Opposite != nil && cfg.IsSwitchNode(iface.Opposite.Node) {
				iface.AddModuleClassLabels(SwitchInterfaceClassName)
			} else {
				iface.AddModuleClassLabels(InterfaceClassName)
			}
		}
	}
	// Only the groups standing for a machine get a spec file. The others group
	// nodes for some purpose of the scenario's own - an AS, an area - and
	// nothing is deployed to them.
	if _, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName); perWorker {
		for _, group := range nm.Groups {
			if cfg.IsWorkerGroup(group) {
				group.AddModuleClassLabels(WorkerGroupClassName)
			}
		}
	}
	return nil
}

func (m TinetModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	// A machine is brought up from its own directory, so everything its spec
	// file mounts has to live under that directory. Splitting the output by
	// some other grouping would scatter the node files elsewhere and leave no
	// relative path from the spec to them.
	if _, perWorker := cfg.GroupClassByName(types.WorkerGroupClassName); perWorker {
		if got := cfg.GlobalSettings.OutputGroupClass; got != types.WorkerGroupClassName {
			return fmt.Errorf(
				"a topology split across %s groups needs global.output_group_class: %s so that "+
					"each machine's files sit beside its spec.yaml (it is %q)",
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

	// parameter {{ .image }}
	for _, node := range nm.Nodes {
		// A switch is realized by TiNET itself as an OVS bridge, so it has no
		// image to run.
		if node.IsVirtual() || cfg.IsSwitchNode(node) {
			continue
		}
		if _, err := node.GetParamValue(TinetImageParamName); err != nil {
			return fmt.Errorf("every (non-virtual) node must have {{ .image }} parameter (none for %s)", node.Name)
		}
	}
	return nil
}

// Options are the TiNET module's own settings, written under
// module_config.tinet.
type Options struct {
	// GenerateScripts writes an entry point script beside the lab. It carries
	// what a person otherwise has to remember: TiNET brings a lab up in two
	// steps, its output is a shell script to be piped, and one of its lines is
	// not a command.
	GenerateScripts bool `yaml:"generate_scripts"`
}

// addEntryScript registers the script that stands in front of TiNET's own
// commands. It sits where the lab is operated from, even when the spec file has
// moved into a directory of its own.
func addEntryScript(cfg *types.Config, scope, subdir string) error {
	cfg.AddFileDefinition(&types.FileDefinition{
		Name:  ScriptFile,
		Path:  "",
		Scope: scope,
	})
	bytes, err := templates.ReadFile("templates/tinet.sh.entry")
	if err != nil {
		return err
	}
	path := TinetOutputFile
	if subdir != "" {
		path = subdir + "/" + TinetOutputFile
	}
	script := strings.ReplaceAll(string(bytes), "%%SPEC%%", path)
	ct := &types.ConfigTemplate{File: ScriptFile, Template: []string{script}}
	if scope == types.ClassTypeGroup {
		cfg.AddGroupClass(&types.GroupClass{
			Name:            ScriptClassName,
			ConfigTemplates: []*types.ConfigTemplate{ct},
		})
	} else {
		cfg.AddNetworkClass(&types.NetworkClass{
			Name:            ScriptClassName,
			ConfigTemplates: []*types.ConfigTemplate{ct},
		})
	}
	return nil
}

// copyFileParams turns the node's staged files into the source and target a copy
// command names. The command itself lives in the module's template, since only
// the platform knows where its commands are written.
func copyFileParams(target types.ValueOwner, cfg *types.Config) ([]map[string]string, error) {
	node, ok := target.(*types.Node)
	if !ok {
		return nil, fmt.Errorf("copyfiles generator requires Node target, got %T", target)
	}
	if node.IsVirtual() {
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
