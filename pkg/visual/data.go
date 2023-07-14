package visual

import (
	"bytes"
	"encoding/json"

	"github.com/cpflat/dot2net/pkg/model"
)

type NetworkModelData struct {
	Name        string            `json:"name" mapstructure:"name"`
	Nodes       []*NodeData       `json:"nodes" mapstructure:"nodes"`
	Connections []*ConnectionData `json:"connections" mapstructure:"connections"`
}

type NodeData struct {
	Name       string            `json:"name" mapstructure:"name"`
	Params     map[string]string `json:"params" mapstructure:"params"`
	Interfaces []*InterfaceData  `json:"interfaces" mapstructure:"interfaces"`
}

type InterfaceData struct {
	Name   string            `json:"name" mapstructure:"name"`
	Params map[string]string `json:"params" mapstructure:"params"`
}

type ConnectionData struct {
	SrcNode      string `json:"src_node" mapstructure:"src_node"`
	SrcInterface string `json:"src_interface" mapstructure:"src_interface"`
	DstNode      string `json:"dst_node" mapstructure:"dst_node"`
	DstInterface string `json:"dst_interface" mapstructure:"dst_interface"`
}

func GetDataJSON(cfg *model.Config, nm *model.NetworkModel) ([]byte, error) {
	nmd := &NetworkModelData{Name: cfg.Name}
	for _, node := range nm.Nodes {
		nd := &NodeData{
			Name:   node.Name,
			Params: node.GetNumbers(),
		}
		for _, iface := range node.Interfaces {
			id := &InterfaceData{
				Name:   iface.Name,
				Params: iface.GetNumbers(),
			}
			nd.Interfaces = append(nd.Interfaces, id)
		}
		nmd.Nodes = append(nmd.Nodes, nd)
	}
	for _, conn := range nm.Connections {
		cd := &ConnectionData{
			SrcNode:      conn.Src.Node.Name,
			SrcInterface: conn.Src.Name,
			DstNode:      conn.Dst.Node.Name,
			DstInterface: conn.Dst.Name,
		}
		nmd.Connections = append(nmd.Connections, cd)
	}
	js, err := json.Marshal(nmd)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = json.Indent(&buf, js, "", "  ")
	if err != nil {
		return nil, err
	}
	js = buf.Bytes()
	return js, err
}
