package tinet

import (
	"encoding/json"
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/cpflat/dot2tinet/pkg/model"
)

// Tn specification definition based on github.com/tinynetwork/tinet v0.0.2

// Tn tinet config
type Tn struct {
	PreCmd      []PreCmd     `yaml:"precmd,omitempty"`
	PreInit     []PreInit    `yaml:"preinit,omitempty"`
	PostInit    []PostInit   `yaml:"postinit,omitempty"`
	PostFini    []PostFini   `yaml:"postfini,omitempty"`
	Nodes       []Node       `yaml:"nodes,omitempty" mapstructure:"nodes,omitempty"`
	Switches    []Switch     `yaml:"switches,omitempty" mapstructure:"switches,omitempty"`
	NodeConfigs []NodeConfig `yaml:"node_configs,omitempty" mapstructure:"node_configs,omitempty"`
	Test        []Test       `yaml:"test,omitempty"`
}

// PreCmd
type PreCmd struct {
	// Cmds []Cmd `yaml:"cmds"`
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// PreInit
type PreInit struct {
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// PostInit
type PostInit struct {
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// PostFini
type PostFini struct {
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

// Node
type Node struct {
	Name           string                 `yaml:"name" mapstructure:"name"`
	Type           string                 `yaml:"type,omitempty" mapstructure:"type,omitempty"`
	NetBase        string                 `yaml:"net_base,omitempty" mapstructure:"net_base,omitempty"`
	VolumeBase     string                 `yaml:"volume,omitempty" mapstructure:"volume,omitempty"`
	Image          string                 `yaml:"image" mapstructure:"image"`
	BuildFile      string                 `yaml:"buildfile,omitempty" mapstructure:"buildfile,omitempty"`
	BuildContext   string                 `yaml:"buildcontext,omitempty" mapstructure:"buildcontext,omitempty"`
	Interfaces     []Interface            `yaml:"interfaces,flow" mapstructure:"interfaces,flow"`
	Sysctls        []Sysctl               `yaml:"sysctls,omitempty" mapstructure:"sysctls,omitempty"`
	Mounts         []string               `yaml:"mounts,flow,omitempty" mapstructure:"mounts,flow,omitempty"`
	DNS            []string               `yaml:"dns,flow,omitempty" mapstructure:"dns,flow,omitempty"`
	DNSSearches    []string               `yaml:"dns_search,flow,omitempty" mapstructure:"dns_search,flow,omitempty"`
	HostNameIgnore bool                   `yaml:"hostname_ignore,omitempty" mapstructure:"hostname_ignore,omitempty"`
	EntryPoint     string                 `yaml:"entrypoint,omitempty" mapstructure:"entrypoint,omitempty"`
	ExtraArgs      string                 `yaml:"docker_run_extra_args,omitempty" mapstructure:"docker_run_extra_args,omitempty"`
	Vars           map[string]interface{} `yaml:"vars,omitempty" mapstructure:"vars,omitempty"`
	Templates      []Template             `yaml:"templates,omitempty" mapstructure:"templates,omitempty"`
}

// Interface
type Interface struct {
	Name string `yaml:"name"`
	Type string `yaml:"type,omitempty"`
	Args string `yaml:"args,omitempty"`
	Addr string `yaml:"addr,omitempty"`
}

// Sysctl
type Sysctl struct {
	Sysctl string `yaml:"string"`
}

type Template struct {
	Src     string `yaml:"src"`
	Dst     string `yaml:"dst"`
	Content string `yaml:"content"`
}

// Switch
type Switch struct {
	Name       string      `yaml:"name"`
	Interfaces []Interface `yaml:"interfaces,omitempty" mapstructure:"interfaces,omitempty"`
}

// NodeConfig
type NodeConfig struct {
	Name string `yaml:"name"`
	Cmds []Cmd  `yaml:"cmds" mapstructure:"cmds"`
}

// Cmd
type Cmd struct {
	Cmd string `yaml:"cmd"`
}

// Test
type Test struct {
	Name string
	Cmds []Cmd `yaml:"cmds" mapstructure:"cmds"`
}

func GetTinetSpecification(cfg *model.Config, nm *model.NetworkModel) ([]byte, error) {
	tn := Tn{}

	for _, n := range nm.Nodes {
		node, err := getTinetNode(cfg, n)
		if err != nil {
			return nil, err
		}
		ifaces := []Interface{}
		for _, i := range n.Interfaces {
			iface, err := getTinetInterface(cfg, i)
			if err != nil {
				return nil, err
			}
			ifaces = append(ifaces, iface)
		}

		if node.Type == "switch" {
			sw := Switch{Name: n.Name}
			sw.Interfaces = ifaces
			tn.Switches = append(tn.Switches, sw)
		} else {
			node.Name = n.Name
			node.Interfaces = ifaces
			tn.Nodes = append(tn.Nodes, node)
		}

		ncfg := NodeConfig{Name: n.Name, Cmds: []Cmd{}}
		for _, line := range n.Commands {
			ncfg.Cmds = append(ncfg.Cmds, Cmd{Cmd: line})
		}
		tn.NodeConfigs = append(tn.NodeConfigs, ncfg)
	}

	bytes, err := yaml.Marshal(tn)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func getTinetNode(cfg *model.Config, n model.Node) (Node, error) {
	mapper := map[string]interface{}{}
	for _, cls := range n.Labels.ClassLabels {
		nc, ok := cfg.NodeClassByName(cls)
		if !ok {
			return Node{}, fmt.Errorf("invalid NodeClass name %v", cls)
		}
		for key, val := range nc.TinetAttr {
			if _, ok := mapper[key]; ok {
				// key already exists -> duplicated
				return Node{}, fmt.Errorf("duplicated Attribute %v in classes %v", key, n.Labels.ClassLabels)
			} else {
				mapper[key] = val
			}
		}
	}

	node := Node{}
	bytes, err := json.Marshal(mapper)
	if err != nil {
		return Node{}, err
	}
	err = json.Unmarshal(bytes, &node)
	if err != nil {
		return Node{}, err
	}
	return node, nil
}

func getTinetInterface(cfg *model.Config, i model.Interface) (Interface, error) {
	iface := Interface{Name: i.Name, Args: i.Node.Name + "#" + i.Name}

	connectionType := ""
	for _, cls := range i.Connection.Labels.ClassLabels {
		cc, ok := cfg.ConnectionClassByName(cls)
		if !ok {
			return Interface{}, fmt.Errorf("invalid ConnectionClass name %v", cls)
		}
		if cc.Type != "" {
			if connectionType == "" {
				connectionType = cc.Type
			} else {
				return Interface{}, fmt.Errorf("duplicated type in ConnectionClasses %v", i.Connection.Labels.ClassLabels)
			}
		}
	}
	interfaceType := ""
	for _, cls := range i.Labels.ClassLabels {
		ic, ok := cfg.InterfaceClassByName(cls)
		if !ok {
			return Interface{}, fmt.Errorf("invalid InterfaceClass name %v", cls)
		}
		if ic.Type != "" {
			if interfaceType == "" {
				interfaceType = ic.Type
			} else {
				return Interface{}, fmt.Errorf("duplicated type in InterfaceClasses %v", i.Labels.ClassLabels)
			}
		}
	}
	iface.Type = connectionType
	if interfaceType != "" {
		iface.Type = interfaceType
	}
	if iface.Type == "" {
		return Interface{}, fmt.Errorf("no given interface type in node %v", i.Node.Name)
	}
	iface.Args = i.Opposite.Node.Name + "#" + i.Opposite.Name

	return iface, nil
}
