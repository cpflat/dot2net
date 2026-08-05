package containerlab

import (
	"embed"
	"fmt"

	"github.com/cpflat/dot2net/pkg/types"
)

const ClabOutputFile = "topo.yaml"

const ClabNetworkNameParamName = "_clab_networkName"
const ClabImageParamName = "image"
const ClabKindParamName = "kind"
const ClabBindMountsParamName = "_clab_bindMounts"
const ClabSrcEndpointParamName = "_clab_src_endpoint"
const ClabDstEndpointParamName = "_clab_dst_endpoint"

const ClabYamlFormatName = "_clabYaml"
const ClabCmdFormatName = "clabCmd"
const ClabLinkFormatName = "_clabLink"

const NetworkClassName = "_clabNetwork"
const NodeClassName = "_clabNode"
const SwitchNodeClassName = "_clabSwitchNode"
const InterfaceClassName = "_clabInterface"
const ConnectionClassName = "_clabConnection"

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
	// add file format
	formatStyle := &types.FormatStyle{
		Name:                ClabYamlFormatName,
		MergeBlockSeparator: ", ",
	}
	cfg.AddFormatStyle(formatStyle)
	formatStyle = &types.FormatStyle{
		Name:                ClabCmdFormatName,
		FormatLinePrefix:    "      - ",
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

	// add file definition
	fileDef := &types.FileDefinition{
		Name:  ClabOutputFile,
		Path:  "",
		Scope: types.ClassTypeNetwork,
	}
	cfg.AddFileDefinition(fileDef)

	// add network class
	ct1 := &types.ConfigTemplate{File: ClabOutputFile}
	bytes, err := templates.ReadFile("templates/topo.yaml.network_clab_topo")
	if err != nil {
		return err
	}
	ct1.Template = []string{string(bytes)}

	networkClass := &types.NetworkClass{
		Name:            NetworkClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct1},
	}
	cfg.AddNetworkClass(networkClass)

	// add node class
	ct1 = &types.ConfigTemplate{Name: "clab_cmds", Format: ClabCmdFormatName, Depends: []string{"startup"}}
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
	ct3 := &types.ConfigTemplate{
		Name:           "clab_topo_exec",
		RequiredParams: []string{"self_startup"},
		Depends:        []string{"clab_cmds"},
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
	ct4.Template = []string{string(bytes)}

	nodeClass := &types.NodeClass{
		Name:            NodeClassName,
		Parameters:      []string{"clab_binds"},
		ConfigTemplates: []*types.ConfigTemplate{ct1, ct2, ct3, ct4},
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

	// add connection class emitting one "links:" entry per connection.
	// The endpoint names come from GenerateParameters; rendering lives in the
	// template so that the list can be aggregated by whichever object owns the
	// output file (see the network class below).
	ct5 := &types.ConfigTemplate{Name: "clab_link", NamespaceFormat: ClabLinkFormatName}
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
				Template: []string{"      - {{ .source }}:{{ .target }}"},
			},
		},
	}
	cfg.AddParameterRule(bindsParamRule)

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
	if node.IsVirtual() {
		return nil, nil
	}

	// Get list of files this node will generate
	nodeFiles := node.FilesToGenerate(cfg)
	fileSet := make(map[string]bool)
	for _, file := range nodeFiles {
		fileSet[file] = true
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

		srcPath, err := node.OutputPath(cfg, fileDef)
		if err != nil {
			return nil, err
		}
		dstPath := fileDef.Path

		params := map[string]string{
			"source": srcPath,
			"target": dstPath,
		}
		results = append(results, params)
	}

	return results, nil
}

func (m *ClabModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
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
		if node.IsVirtual() {
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
