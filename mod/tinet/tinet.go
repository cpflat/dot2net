package tinet

import (
	"embed"
	"fmt"
	"path"

	"github.com/cpflat/dot2net/pkg/types"
)

const TinetOutputFile = "spec.yaml"

const TinetNetworkNameParamName = "_tn_networkName"
const TinetImageParamName = "image"
const TinetBindMountsParamName = "_tn_bindMounts"

const TinetYamlFormatName = "_tinetYaml"
const SpecCmdFormatName = "tinetSpecCmd"

// const TinetVtyshCLIFormatName = "tinetVtyshCLI"

const NetworkClassName = "_tinetNetwork"
const NodeClassName = "_tinetNode"
const InterfaceClassName = "_tinetInterface"

//go:embed templates/*
var templates embed.FS

type TinetModule struct {
	*types.StandardModule
}

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
	ct1 := &types.ConfigTemplate{File: TinetOutputFile}
	bytes, err := templates.ReadFile("templates/spec.yaml.network")
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
	m.AddModuleNodeClassLabel(NodeClassName)

	// add interface class
	ct1 = &types.ConfigTemplate{Name: "tn_spec", Format: TinetYamlFormatName}
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
	m.AddModuleInterfaceClassLabel(InterfaceClassName)

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

		srcPath := path.Join(node.Name, fileDef.Name)
		dstPath := fileDef.Path

		params := map[string]string{
			"source": srcPath,
			"target": dstPath,
		}
		results = append(results, params)
	}

	return results, nil
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
		if node.IsVirtual() {
			continue
		} else {
			_, err := node.GetParamValue(TinetImageParamName)
			if err != nil {
				return fmt.Errorf("every (non-virtual) node must have {{ .image }} parameter (none for %s)", node.Name)
			}
		}
	}
	return nil
}
