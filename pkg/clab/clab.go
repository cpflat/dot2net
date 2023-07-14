package clab

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/cpflat/dot2net/pkg/model"
)

const (
	DEFAULT_NAME = "clab"
)

func GetClabTopology(cfg *model.Config, nm *model.NetworkModel) ([]byte, error) {

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
	mlayer := cfg.ManagementLayer
	if cfg.HasManagementLayer() {
		addrrange, err := netip.ParsePrefix(mlayer.AddrRange)
		if err != nil {
			return nil, err
		}
		if addrrange.Addr().Is4() {
			config.Mgmt.IPv4Subnet = mlayer.AddrRange
			config.Mgmt.IPv4Gw = mlayer.ExternalGateway
		} else if addrrange.Addr().Is6() {
			config.Mgmt.IPv6Subnet = mlayer.AddrRange
			config.Mgmt.IPv6Gw = mlayer.ExternalGateway
		}
	}

	for _, node := range nm.Nodes {
		// skip virtual nodes
		if node.Virtual {
			continue
		}

		// node settings
		name := node.Name
		ndef, err := getClabNode(cfg, node)
		if err != nil {
			return nil, err
		}
		config.Topology.Nodes[name] = ndef

		// mgmt interface settings
		mgmtif := node.GetManagementInterface()
		if cfg.HasManagementLayer() && mgmtif != nil {
			val, err := mgmtif.GetValue(mlayer.IPAddressReplacer())
			if err != nil {
				return nil, err
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

		// add mount points
		for _, filename := range node.Files.FileNames() {
			file := node.Files.GetFile(filename)
			if file.FileDefinition.Path == "" {
				continue
			}
			dirpath, err := filepath.Abs(node.Name)
			if err != nil {
				return nil, fmt.Errorf("directory path panic")
			}
			//dirpath = strings.TrimRight(dirpath, "/")
			cfgpath := filepath.Join(dirpath, file.FileDefinition.Name)
			targetpath := file.FileDefinition.Path
			bindstr := cfgpath + ":" + targetpath
			ndef.Binds = append(ndef.Binds, bindstr)
		}

		embed := node.Files.GetEmbeddedConfig()
		if embed != nil {
			// add inline configuration commands
			config.Topology.Nodes[name].Exec = append(config.Topology.Nodes[name].Exec, node.Files.GetEmbeddedConfig().Content...)
		}
	}

	for _, conn := range nm.Connections {
		// skip virtual links
		if conn.Src.Virtual || conn.Dst.Virtual {
			continue
		}

		// link settings
		link := getClabLink(cfg, conn)
		config.Topology.Links = append(config.Topology.Links, link)
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
