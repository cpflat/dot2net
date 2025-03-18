package tinet

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	fileFormat := &types.FileFormat{
		Name:           TinetYamlFormatName,
		BlockSeparator: ", ",
	}
	cfg.AddFileFormat(fileFormat)
	fileFormat = &types.FileFormat{
		Name:           SpecCmdFormatName,
		LinePrefix:     "      - cmd: ",
		BlockSeparator: "\n",
	}
	cfg.AddFileFormat(fileFormat)
	// 	fileFormat = &types.FileFormat{
	// 		Name:          TinetVtyshCLIFormatName,
	// 		LineSeparator: "\" -c \"",
	// 		BlockPrefix:   "vtysh -c \"conf t\" -c \"",
	// 		BlockSuffix:   "\"",
	// 	}
	// 	cfg.AddFileFormat(fileFormat)

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

	ct3 := &types.ConfigTemplate{Name: "tn_config", Depends: []string{"tn_cmds"}}
	bytes, err = templates.ReadFile("templates/spec.yaml.node_tn_config")
	if err != nil {
		return err
	}
	ct3.Template = []string{string(bytes)}

	nodeClass := &types.NodeClass{
		Name:            NodeClassName,
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

	return nil
}

func (m TinetModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {

	// set network name
	nm.AddParam(TinetNetworkNameParamName, cfg.Name)

	for _, node := range nm.Nodes {
		// generate file mount point descriptions
		bindItems := []string{}
		for _, fileDef := range cfg.FileDefinitions {
			if fileDef.Path == "" {
				continue
			}

			dirpath, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to obtain currrent directory")
			}
			srcPath := filepath.Join(dirpath, node.Name, fileDef.Name)
			dstPath := fileDef.Path
			bindItems = append(bindItems, srcPath+":"+dstPath)
		}
		node.AddParam(TinetBindMountsParamName, strings.Join(bindItems, ", "))
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
		_, err := node.GetParamValue(TinetImageParamName)
		if err != nil {
			return fmt.Errorf("every node must have {{ .image }} parameter (none for %s)", node.Name)
		}
	}
	return nil
}
