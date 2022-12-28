package clab

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"

	//"github.com/srl-labs/containerlab/clab"
	//"github.com/srl-labs/containerlab/types"

	"github.com/cpflat/dot2tinet/pkg/model"
)

const DEFAULT_NAME = "clab"

func GetClabTopologyConfig(cfg *model.Config, nm *model.NetworkModel) ([]byte, error) {
	// config := &clab.Config{
	// 	Name: DEFAULT_NAME,
	// 	Mgmt: new(types.MgmtNet),
	// 	Topology: &types.Topology{
	// 		Kinds: make(map[string]*types.NodeDefinition),
	// 		Nodes: make(map[string]*types.NodeDefinition),
	// 	},
	// }
	config := &Config{
		Name: DEFAULT_NAME,
		Mgmt: new(MgmtNet),
		Topology: &Topology{
			Kinds: make(map[string]*NodeDefinition),
			Nodes: make(map[string]*NodeDefinition),
		},
	}

	gattr := cfg.GlobalSettings.ClabAttr
	for k, v := range gattr {
		switch k {
		case "name":
			name, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("global.clab.name must be string")
			}
			config.Name = name
		case "mgmt":
			mapper, ok := v.(map[string]string)
			if !ok {
				return nil, fmt.Errorf("global.clab.mgmt invalid format")
			}
			bytes, err := json.Marshal(mapper)
			if err != nil {
				return nil, err
			}
			// mgmt := types.MgmtNet{}
			mgmt := MgmtNet{}
			err = yaml.Unmarshal(bytes, &mgmt)
			if err != nil {
				return nil, err
			}
			config.Mgmt = &mgmt
		case "kinds":
			mapper, ok := v.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("global.clab.kinds invalid format")
			}
			bytes, err := json.Marshal(mapper)
			if err != nil {
				return nil, err
			}
			// kinds := make(map[string]*types.NodeDefinition)
			kinds := make(map[string]*NodeDefinition)
			err = yaml.Unmarshal(bytes, &kinds)
			if err != nil {
				return nil, err
			}
			config.Topology.Kinds = kinds
		default:
			return nil, fmt.Errorf("invalid field in global.clab")
		}
	}

	for _, node := range nm.Nodes {
		name := node.Name
		ndef, err := getClabNode(cfg, node)
		if err != nil {
			return nil, err
		}
		config.Topology.Nodes[name] = ndef
	}

	for _, conn := range nm.Connections {
		link := getClabLink(cfg, conn)
		config.Topology.Links = append(config.Topology.Links, link)
	}

	bytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

// func getClabNode(cfg *model.Config, n model.Node) (*types.NodeDefinition, error) {
func getClabNode(cfg *model.Config, n model.Node) (*NodeDefinition, error) {
	mapper := map[string]interface{}{}
	for _, cls := range n.Labels.ClassLabels {
		nc, ok := cfg.NodeClassByName(cls)
		if !ok {
			return nil, fmt.Errorf("invalid NodeClass name %v", cls)
		}
		for key, val := range nc.ClabAttr {
			if _, ok := mapper[key]; ok {
				// key already exists -> duplicated
				return nil, fmt.Errorf("duplicated Attribute %v in classes %v", key, n.Labels.ClassLabels)
			} else {
				mapper[key] = val
			}
		}
	}

	// ndef := types.NodeDefinition{}
	ndef := NodeDefinition{}
	bytes, err := json.Marshal(mapper)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &ndef)
	if err != nil {
		return nil, err
	}
	return &ndef, nil
}

// func getClabLink(cfg *model.Config, conn model.Connection) *types.LinkConfig {
func getClabLink(cfg *model.Config, conn model.Connection) *LinkConfig {
	src := conn.Src.Node.Name + ":" + conn.Src.Name
	dst := conn.Dst.Node.Name + ":" + conn.Dst.Name
	// link := types.LinkConfig{
	link := LinkConfig{
		Endpoints: []string{src, dst},
	}
	return &link
}

func GetScripts(cfg *model.Config, nm *model.NetworkModel) map[string][]string {

	buffers := map[string][]string{}

	for _, n := range nm.Nodes {
		buffers[n.Name] = n.Commands
	}
	return buffers
}
