package clab

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/cpflat/dot2tinet/pkg/model"
)

const (
	DEFAULT_NAME     = "clab"
	SCRIPT_PATH      = "/"
	SCRIPT_EXTENSION = ".sh"
	SCRIPT_SHELL     = "sh"
)

func GetScriptPaths(cfg *model.Config, nm *model.NetworkModel) map[string]string {
	cfgmap := map[string]string{}
	for _, n := range nm.Nodes {
		filename := n.Name + SCRIPT_EXTENSION
		cfgmap[n.Name] = filename
	}
	return cfgmap
}

func getClabConfigBase(cfg *model.Config, nm *model.NetworkModel) (*Config, error) {

	config := &Config{
		Name: "",
		Mgmt: new(MgmtNet),
		Topology: &Topology{
			Kinds: make(map[string]*NodeDefinition),
			Nodes: make(map[string]*NodeDefinition),
		},
	}

	// clab global attributes
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

	// global settings
	if config.Name == "" {
		if cfg.Name != "" {
			config.Name = cfg.Name
		} else {
			config.Name = DEFAULT_NAME
		}
	}

	// mgmt network settings
	mgmtspace := cfg.GetManagementIPSpace()
	if mgmtspace != nil {
		addrrange, err := netip.ParsePrefix(mgmtspace.AddrRange)
		if err != nil {
			return nil, err
		}
		if addrrange.Addr().Is4() {
			config.Mgmt.IPv4Subnet = mgmtspace.AddrRange
			config.Mgmt.IPv4Gw = mgmtspace.ExternalGateway
		} else if addrrange.Addr().Is6() {
			config.Mgmt.IPv6Subnet = mgmtspace.AddrRange
			config.Mgmt.IPv6Gw = mgmtspace.ExternalGateway
		}
	}

	for _, node := range nm.Nodes {
		// node settings
		name := node.Name
		ndef, err := getClabNode(cfg, node)
		if err != nil {
			return nil, err
		}
		config.Topology.Nodes[name] = ndef

		// mgmt interface settings
		mgmtif := node.GetManagementInterface()
		if mgmtspace != nil && mgmtif != nil {
			val, ok := mgmtif.RelativeNumbers[mgmtspace.IPAddressReplacer()]
			if !ok {
				return nil, fmt.Errorf("clab mgmt address store panic")
			}
			addr, err := netip.ParseAddr(val)
			if err != nil {
				return nil, err
			}
			if addr.Is4() {
				config.Topology.Nodes[name].MgmtIPv4 = val
			} else if addr.Is6() {
				config.Topology.Nodes[name].MgmtIPv6 = val
			} else {
				return nil, fmt.Errorf("clab mgmt address format panic %s", val)
			}
		}
	}

	for _, conn := range nm.Connections {
		// link settings
		link := getClabLink(cfg, conn)
		config.Topology.Links = append(config.Topology.Links, link)
	}

	return config, nil
}

func GetClabTopology(cfg *model.Config, nm *model.NetworkModel,
	cfgmap map[string]string, dirname string) ([]byte, error) {

	config, err := getClabConfigBase(cfg, nm)
	if err != nil {
		return nil, err
	}

	for _, node := range nm.Nodes {
		name := node.Name

		// configuration script settings
		cfgname, ok := cfgmap[node.Name]
		if !ok {
			return nil, fmt.Errorf("configuration file name not found for node %s", node.Name)
		}
		cfgpath := filepath.Join(dirname, cfgname)
		targetpath := filepath.Join(SCRIPT_PATH, cfgname)
		bindstr := cfgpath + ":" + targetpath
		execstr := SCRIPT_SHELL + " " + targetpath

		// mount script
		config.Topology.Nodes[name].Binds = append(config.Topology.Nodes[name].Binds, bindstr)
		// add script execution command
		config.Topology.Nodes[name].Exec = append(config.Topology.Nodes[name].Exec, execstr)
	}

	bytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return bytes, nil

}

func GetClabTopologyConfig(cfg *model.Config, nm *model.NetworkModel) ([]byte, error) {

	config, err := getClabConfigBase(cfg, nm)
	if err != nil {
		return nil, err
	}

	for _, node := range nm.Nodes {
		// add inline configuration commands
		name := node.Name
		config.Topology.Nodes[name].Exec = append(config.Topology.Nodes[name].Exec, node.Commands...)
	}

	bytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, err
	}
	return bytes, nil

}

func getClabNode(cfg *model.Config, n *model.Node) (*NodeDefinition, error) {
	// clab node attributes
	ndef := &NodeDefinition{}
	if n.ClabAttr == nil {
		return ndef, nil
	}
	mapper := n.ClabAttr

	bytes, err := json.Marshal(mapper)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, ndef)
	if err != nil {
		return nil, err
	}
	return ndef, nil
}

func getClabLink(cfg *model.Config, conn *model.Connection) *LinkConfig {
	src := conn.Src.Node.Name + ":" + conn.Src.Name
	dst := conn.Dst.Node.Name + ":" + conn.Dst.Name
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
