package tinet

import (
	"embed"
	"fmt"

	"github.com/cpflat/dot2net/pkg/types"
)

const TinetOutputFile = "spec.yaml"

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

	// add file definition
	fileDef := &types.FileDefinition{
		Name:  TinetOutputFile,
		Path:  "",
		Scope: types.ClassTypeNetwork,
	}
	cfg.AddFileDefinition(fileDef)

	// add network class
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

	networkClass := &types.NetworkClass{
		Name:            NetworkClassName,
		ConfigTemplates: []*types.ConfigTemplate{ctSwitches, ct1},
	}
	cfg.AddNetworkClass(networkClass)

	// add node class
	ct1 = &types.ConfigTemplate{Name: "tn_cmds", Format: SpecCmdFormatName, Depends: []string{"startup"}}
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

	ct3 := &types.ConfigTemplate{
		Name:    "tn_config",
		Depends: []string{"tn_cmds"},
		Blocks: types.BlocksConfig{
			After: []string{"self_tn_cmds"},
		},
	}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_config")
	if err != nil {
		return err
	}
	ct3.Template = []string{string(bytes)}

	nodeClass := &types.NodeClass{
		Name:            NodeClassName,
		Parameters:      []string{"tinet_binds"},
		ConfigTemplates: []*types.ConfigTemplate{ct1, ct2, ct3},
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
				Template: []string{"{{ .source }}:{{ .target }}"},
				Format:   TinetYamlFormatName,
			},
		},
	}
	cfg.AddParameterRule(bindsParamRule)

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
	return nil
}

func (m TinetModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
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
