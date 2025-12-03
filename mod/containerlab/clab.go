package containerlab

import (
	"embed"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

// special parameter ".virtual" for nodes
// -> config files for the virtual nodes are not generated and just ignored

const VirtualNodeClassName = "virtual"

const ClabOutputFile = "topo.yaml"

const ClabNetworkNameParamName = "_clab_networkName"
const ClabImageParamName = "image"
const ClabKindParamName = "kind"
const ClabBindMountsParamName = "_clab_bindMounts"
const ClabEndpointsParamName = "_clab_link_endpoints"

const ClabYamlFormatName = "_clabYaml"
const ClabCmdFormatName = "clabCmd"

const NetworkClassName = "_clabNetwork"
const NodeClassName = "_clabNode"
const InterfaceClassName = "_clabInterface"

//go:embed templates/*
var templates embed.FS

type ClabModule struct {
	*types.StandardModule
}

func NewModule() types.Module {
	return &ClabModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *ClabModule) UpdateConfig(cfg *types.Config) error {
	// add file format
	formatStyle := &types.FormatStyle{
		Name: ClabYamlFormatName,

		// New fields (Format Phase)
		// (BlockSeparator only, no Line processing needed)

		// Legacy (v0.6.x compatibility)
		BlockSeparator: ", ",
	}
	cfg.AddFormatStyle(formatStyle)
	formatStyle = &types.FormatStyle{
		Name: ClabCmdFormatName,

		// New fields (Format Phase)
		FormatLinePrefix: "      - ",

		// Legacy (v0.6.x compatibility)
		LinePrefix:     "      - ",
		BlockSeparator: "\n",
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

	ct2 := &types.ConfigTemplate{Name: "clab_topo", Depends: []string{"clab_cmds"}}
	bytes, err = templates.ReadFile("templates/topo.yaml.node_clab_topo")
	if err != nil {
		return err
	}
	ct2.Template = []string{string(bytes)}

	nodeClass := &types.NodeClass{
		Name:            NodeClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct1, ct2},
	}
	cfg.AddNodeClass(nodeClass)
	m.AddModuleNodeClassLabel(NodeClassName)

	return nil
}

func (m *ClabModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {

	// set network name
	nm.AddParam(ClabNetworkNameParamName, cfg.Name)

	// generate connection endpoint descriptions
	endpoints := []string{}
	for _, conn := range nm.Connections {
		// skip connections involving virtual nodes
		if conn.Src.Node.IsVirtual() || conn.Dst.Node.IsVirtual() {
			continue
		}
		endpoint := fmt.Sprintf("[%s:%s, %s:%s]", conn.Src.Node.Name, conn.Src.Name, conn.Dst.Node.Name, conn.Dst.Name)
		endpoints = append(endpoints, endpoint)
	}
	nm.AddParam(
		ClabEndpointsParamName,
		"  - endpoints: "+strings.Join(endpoints, "\n  - endpoints: ")+"\n",
	)

	for _, node := range nm.Nodes {
		// skip virtual nodes
		if node.IsVirtual() {
			continue
		}
		// generate file mount point descriptions
		bindItems := []string{}
		for _, fileDef := range cfg.FileDefinitions {
			if fileDef.Path == "" {
				continue
			}

			srcPath := filepath.Join(node.Name, fileDef.Name)
			dstPath := fileDef.Path
			bindItems = append(bindItems, srcPath+":"+dstPath)
		}
		node.AddParam(
			ClabBindMountsParamName,
			"      - "+strings.Join(bindItems, "\n      - ")+"\n",
		)
	}

	return nil
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
		_, err := node.GetParamValue(ClabImageParamName)
		if err != nil {
			return fmt.Errorf("every (non-virtual) node must have {{ .image }} parameter (none for %s)", node.Name)
		}
		_, err = node.GetParamValue(ClabKindParamName)
		if err != nil {
			return fmt.Errorf("every (non-virtual) node must have {{ .kind }} parameter (none for %s)", node.Name)
		}
	}
	return nil
}
