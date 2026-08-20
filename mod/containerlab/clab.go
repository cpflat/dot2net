package containerlab

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
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

// ClabBridgeParamName carries what a switch node is called on the machine, as
// opposed to in the model. Everything that names the bridge reads it, the
// topology included: a command putting the machine's own NIC into the bridge
// cannot be written without it. No leading underscore for that reason - the
// module's own names carry one, and this is not one of them.
// HookPrefix names this module's gathering points and ties a block to its
// script. One constant, so that a block's scope and the slots that read it
// cannot be spelled differently - they would not meet, and the block would be
// dropped with only the "nothing would run it" error to say so.
const HookPrefix = "clab"

const ClabBridgeParamName = "clab_bridge"

// ClabHostPortParamName carries what the veth reaching a bridge is called on
// the machine. A topology writing host-side commands - a capture on one link, a
// machine's own NIC put into the bridge - needs to name these, and cannot know
// them otherwise. Set on a switch node's interfaces, so the node facing one
// reads it as {{ .opp_clab_host_port }}.
const ClabHostPortParamName = "clab_host_port"

// The anchors of the module's own blocks in the machine-side columns. A
// topology places its own blocks against these by name, and the check that a
// block does not read something that is not there yet works out where they sit.
const (
	BridgeSetupAnchor   = "clab_bridge_setup"
	BridgeCleanupAnchor = "clab_bridge_cleanup"
)

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
// class and writes its own worker_deploy template, or none at all.
//
// The names carry no underscore because they are meant to be written by users.
const OvsBridgeSetupClassName = "clabOvsBridgeSetup"
const LinuxBridgeSetupClassName = "clabLinuxBridgeSetup"

// Both classes write into worker_deploy and worker_destroy - the hooks dot2net
// owns for what runs on the machine rather than in a node. A topology adds its
// own commands under the same names, so making a bridge and attaching a host
// interface to it are written the same way, and neither has to know about the
// other.
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
	// The lab's name carries this run's own, so that one machine can hold two
	// labs made from one topology: containerlab names every container after the
	// lab, and without it both would be clab-host1-r1.
	ct1.Template = []string{strings.ReplaceAll(string(bytes), "%%NODEPREFIX%%", cfg.LabNamePrefix(perWorker))}

	owns := []*types.ConfigTemplate{ct1}

	// The entry script joins the same class, rather than getting one of its
	// own: a class of its own is a class no group carries a label for, and the
	// script would silently not be written for a multi-machine lab.
	// What these two name comes into being at different moments, and a block
	// reading one too early is caught rather than left to fail on the machine.
	// The bridge is made by the module's own worker_deploy; the veth reaching it
	// is made by containerlab as it brings the lab up.
	cfg.DeclareParamMadeBy(ClabBridgeParamName, BridgeSetupAnchor)
	cfg.DeclareParamGoneBy(ClabBridgeParamName, BridgeCleanupAnchor)
	cfg.DeclareParamMadeBy(ClabHostPortParamName, "worker_deploy")
	cfg.DeclareParamGoneBy(ClabHostPortParamName, "worker_destroy")

	if opts.GenerateScripts {
		entry, err := entryScriptTemplate(cfg, scope, subdir, perWorker)
		if err != nil {
			return err
		}
		slots, names := cfg.MachineHookSlots(HookPrefix)
		owns = append(owns, slots...)
		commands, err := machineCommands()
		if err != nil {
			return err
		}
		owns = append(owns, commands...)
		entry.Depends = names
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
		Depends:        []string{"clab_startup"},
		RequiredParams: []string{"self_clab_startup"},
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

	// exec section - only output if there are commands to run
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
		Name:          "clab_topo",
		Depends:       []string{"clab_topo_binds", "clab_topo_exec"},
		PlatformEntry: true,
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
		Name:           "clab_teardown_body",
		Depends:        []string{"clab_teardown"},
		RequiredParams: []string{"self_clab_teardown"},
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
		Name:       NodeClassName,
		Parameters: []string{"clab_binds", "clab_copies", "clab_collects"},
		ConfigTemplates: append([]*types.ConfigTemplate{
			// Where the lab's own commands are gathered: the topology writes
			// into startup and teardown, and this module's own files read the
			// result. Each platform has a pair of these, and a block written
			// once reaches all of them.
			types.HookSorter(HookPrefix, "startup"),
			types.HookSorter(HookPrefix, "teardown"),
		}, ct1, ct2, ct3, ct4, ctCopies, ctExecBody, ctTeardown, ctCollect),
	}
	cfg.AddNodeClass(nodeClass)
	// Not AddModuleNodeClassLabel: which of the two node classes a node gets is
	// decided per node in ClassifyObjects.

	// A switch node is a shared L2 domain the platform realizes itself, so it
	// carries no image, no bind mounts and no commands - only the kind, whose
	// value comes from the user untouched.
	ct6 := &types.ConfigTemplate{Name: "clab_topo", PlatformEntry: true}
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
	//
	// Both are blocks of the entry script's columns, so they are written when
	// there is a script to run them and not otherwise. Without one, the bridges
	// are the machine's own business and dot2net has nowhere to say so.
	for _, setup := range []struct{ className, file, cleanupFile string }{
		{OvsBridgeSetupClassName, "templates/setup.node_clab_ovs_bridge", "templates/setup.node_clab_ovs_bridge_cleanup"},
		{LinuxBridgeSetupClassName, "templates/setup.node_clab_linux_bridge", "templates/setup.node_clab_linux_bridge_cleanup"},
	} {
		if !opts.GenerateScripts {
			break
		}
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
				// The module's own groups, which only this module's script
				// gathers: the commands name a bridge under a name only
				// containerlab's files use.
				{
					Group: HookPrefix + "/worker_deploy", Priority: types.ModuleHookPriority,
					Anchor: BridgeSetupAnchor, Template: []string{string(bytes)},
				},
				{
					Group: HookPrefix + "/worker_destroy", Priority: types.ModuleUndoPriority,
					Anchor: BridgeCleanupAnchor, Template: []string{string(cleanup)},
				},
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
// maxHostNetdevName is what Linux allows an interface to be called: IFNAMSIZ is
// 16 and includes the terminator. An OVS bridge is subject to it too, because
// it comes with an internal device of the same name - and an over-long one is
// worse than refused, since ovs-vsctl reports the failure but still records the
// bridge, leaving one that no device answers for.
const maxHostNetdevName = 15

// hostTokenLength is how much of the hash is kept. Six hex digits is 16 million
// values, against the few dozen names one machine holds at a time; four would
// be 65536, where eighty names already collide about one time in twenty. What
// is left is eight characters for the interface's own name.
const hostTokenLength = 6

// maxHostPortOwnName is what an interface on a switch node may be called, once
// the token and its separator are taken out of the fifteen. Stated as a rule
// the topology can follow: an automatic name is a prefix and a number, so a
// prefix of five leaves room for a thousand ports.
const maxHostPortOwnName = maxHostNetdevName - hostTokenLength - 1

// checkHostNamesFit reports a name that will not fit on the machine, and a
// machine that would be given one name twice.
//
// Both are settled while generating rather than at deploy time, where an
// over-long name appears as an ip or ovs-vsctl error naming nothing the
// topology wrote - and where OVS makes it worse, reporting the failure but
// recording the bridge anyway, leaving one that no device answers for.
func checkHostNamesFit(cfg *types.Config, nm *types.NetworkModel) error {
	// Per machine: two labs cannot be checked against each other from here, but
	// one machine's own names are all made in this pass.
	seen := map[string]map[string]string{} // machine -> host name -> what claimed it

	for _, node := range nm.Nodes {
		if !cfg.IsSwitchNode(node) || !node.IsMaterialised() {
			continue
		}
		machine := ""
		for _, group := range node.Groups {
			if cfg.IsWorkerGroup(group) {
				machine = group.Name
			}
		}
		if seen[machine] == nil {
			seen[machine] = map[string]string{}
		}
		claim := func(name, by string) error {
			if other, taken := seen[machine][name]; taken {
				return fmt.Errorf(
					"%s and %s would both be called %s on the machine: "+
						"give one of them a name of its own",
					other, by, name)
			}
			seen[machine][name] = by
			return nil
		}
		if err := claim(HostBridgeName(cfg, node), "node "+node.Name); err != nil {
			return err
		}

		for _, iface := range node.Interfaces {
			if len(iface.Name) > maxHostPortOwnName {
				return fmt.Errorf(
					"interface %s of node %s is %d characters where a switch node's "+
						"interfaces may have %d: on the machine it becomes %s, and Linux "+
						"allows an interface name %d characters (IFNAMSIZ). An automatic "+
						"name is a prefix and a number, so a prefix of %d leaves room for "+
						"a thousand ports",
					iface.Name, node.Name, len(iface.Name), maxHostPortOwnName,
					HostPortName(cfg, iface), maxHostNetdevName, maxHostPortOwnName-3)
			}
			if err := claim(HostPortName(cfg, iface), "interface "+iface.Name+" of node "+node.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

// clabEndpoint is how a link names one of its ends in topo.yaml.
//
// For a container both halves are the node's own business: the interface is
// made inside its network namespace, where nothing else can be reached, and
// eth0 there is eth0 no matter how many labs the machine is holding.
//
// A bridge has no namespace to be inside - it is a device, and the veth that
// reaches it has to sit in the same namespace it does, which is the machine's
// own. So both the bridge and every veth reaching it are named next to the
// machine's own equipment, where eth0 is its first NIC. containerlab hands
// that naming to whoever writes the topology and says so:
//
//	When choosing names of the interfaces that need to be connected to the
//	bridge make sure that these names are not clashing with existing interfaces.
//
// Here that is dot2net. The names it makes therefore carry a token standing for
// the lab and the bridge, the way docker names a container's host-side veth
// vethXXXXXXX rather than eth0 - for the same reason, and to no worse effect,
// since nobody types these: a topology writing host-side commands reads them
// from the parameters below.
func clabEndpoint(cfg *types.Config, iface *types.Interface) string {
	if !cfg.IsSwitchNode(iface.Node) {
		return fmt.Sprintf("%s:%s", iface.Node.Name, iface.Name)
	}
	return fmt.Sprintf("%s:%s", HostBridgeName(cfg, iface.Node), HostPortName(cfg, iface))
}

// hostNameToken stands for the lab and the bridge in a name the machine sees.
// Deterministic, so that destroy names what deploy made; short, because it is
// spent out of fifteen characters.
//
// Being able to work the name out again is what it is for, and not only within
// one run: a tool that lost its generated files can rebuild the lab name and ask
// dot2net for the bridge, which is how leftovers from an interrupted run are
// found. So the input, the length and the shape are a promise. Changing any of
// them renames every bridge and orphans whatever an older version left behind -
// a breaking change, to be released as one.
func hostNameToken(cfg *types.Config, node *types.Node) string {
	sum := sha256.Sum256([]byte(cfg.Name + "\x00" + node.LocalNameOr()))
	return hex.EncodeToString(sum[:])[:hostTokenLength]
}

// HostPortName is the veth reaching a bridge, as the machine sees it. The
// interface keeps its own name in front so that the port is recognisable, and
// the token behind it makes the whole unique on a machine holding more than one
// lab.
func HostPortName(cfg *types.Config, iface *types.Interface) string {
	return iface.Name + "-" + hostNameToken(cfg, iface.Node)
}

// HostBridgeName is what a switch node is called on the machine, as opposed to
// in the model. Exported because the bridge setup classes name the same thing.
func HostBridgeName(cfg *types.Config, node *types.Node) string {
	return "br-" + hostNameToken(cfg, node)
}

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
	// What a switch node is called on the machine. One parameter, so that the
	// entry in topo.yaml and the commands that make and remove the bridge
	// cannot disagree about it.
	for _, node := range nm.Nodes {
		if !cfg.IsSwitchNode(node) {
			continue
		}
		node.AddParam(ClabBridgeParamName, HostBridgeName(cfg, node))
		for _, iface := range node.Interfaces {
			iface.AddParam(ClabHostPortParamName, HostPortName(cfg, iface))
		}
	}

	for _, conn := range nm.Connections {
		conn.AddParam(ClabSrcEndpointParamName, clabEndpoint(cfg, conn.Src))
		conn.AddParam(ClabDstEndpointParamName, clabEndpoint(cfg, conn.Dst))
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

	// Nothing to deliver to a node that is not there, and nothing to deliver to
	// one whose configuration is withheld: no file was written for it, so a mount
	// naming one would point at a path that does not exist.
	if !node.IsMaterialised() || node.IsVirtual() {
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
	var collectOpts Options
	if _, err := cfg.DecodeModuleConfig("containerlab", &collectOpts); err != nil {
		return err
	}
	if err := types.CheckCollectNeedsScript(cfg, nm, "containerlab", collectOpts.GenerateScripts); err != nil {
		return err
	}

	if err := checkHostNamesFit(cfg, nm); err != nil {
		return err
	}

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
				return fmt.Errorf("every deployed node must have {{ .image }} parameter (none for %s)", node.Name)
			}
		}
		if _, err := node.GetParamValue(ClabKindParamName); err != nil {
			return fmt.Errorf("every deployed node must have {{ .kind }} parameter (none for %s)", node.Name)
		}
	}
	return nil
}

// addEntryScript registers the script that stands in front of this platform's
// own command. It sits where the lab is operated from - the output root, or a
// machine's directory - even when the topology file itself has moved into a
// directory of its own, because the point of it is to be reachable without
// knowing that layout.
func entryScriptTemplate(cfg *types.Config, scope, subdir string, perWorker bool) (*types.ConfigTemplate, error) {
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
	script = strings.ReplaceAll(script, "%%NODEPREFIX%%", cfg.LabNamePrefix(perWorker))
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

// readTemplate fills a config template in from the module's own files.
// machineCommands are containerlab's own commands, one per machine-side hook,
// as blocks of the columns the entry script reads.
//
// They are blocks like any other, which is what lets a topology say `after:
// worker_deploy` and have its commands run once the lab is up. Each carries the
// hook's name as its anchor: that name is dot2net's, so the same line works
// whichever platform is writing the script.
func machineCommands() ([]*types.ConfigTemplate, error) {
	files := map[string]string{
		"worker_deploy":  "templates/worker.deploy",
		"worker_exec":    "templates/worker.exec",
		"worker_collect": "templates/worker.collect",
		"worker_destroy": "templates/worker.destroy",
	}
	cts := make([]*types.ConfigTemplate, 0, len(types.MachineHookOrder))
	for _, hook := range types.MachineHookOrder {
		ct, err := readTemplate(files[hook], &types.ConfigTemplate{
			Group:    HookPrefix + "/" + hook,
			Priority: types.PlatformCommandPriority,
			Anchor:   hook,
		})
		if err != nil {
			return nil, err
		}
		cts = append(cts, ct)
	}
	return cts, nil
}

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
