package tinet

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/cpflat/dot2tinet/pkg/model"
)

const (
	SCRIPT_PATH      = "/tinet"
	SCRIPT_EXTENSION = ".sh"
	SCRIPT_SHELL     = "sh"
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

func GetScriptPaths(cfg *model.Config, nm *model.NetworkModel) map[string]string {
	cfgmap := map[string]string{}
	for _, n := range nm.Nodes {
		filename := n.Name + SCRIPT_EXTENSION
		cfgmap[n.Name] = filename
	}
	return cfgmap
}

func getTinetSpecificationBase(cfg *model.Config, nm *model.NetworkModel) (*Tn, error) {
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
			if iface.Type == "" {
				iface.Type = "direct"
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
	}

	return &tn, nil
}

func GetTinetSpecification(cfg *model.Config, nm *model.NetworkModel,
	cfgmap map[string]string, dirname string) ([]byte, error) {

	tn, err := getTinetSpecificationBase(cfg, nm)
	if err != nil {
		return nil, err
	}

	// add configuration commands to Tn.NodeConfigs
	cfgdir := strings.TrimRight(dirname, "/")
	for i, node := range tn.Nodes {
		n, exists := nm.NodeByName(node.Name)
		if !exists {
			return nil, fmt.Errorf("node %s not found", node.Name)
		}

		cfgname, ok := cfgmap[n.Name]
		if !ok {
			return nil, fmt.Errorf("configuration file name not found for node %s", node.Name)
		}
		cfgpath := filepath.Join(cfgdir, cfgname)
		targetpath := filepath.Join(SCRIPT_PATH, cfgname)
		bindstr := cfgpath + ":" + targetpath
		execstr := SCRIPT_SHELL + " " + targetpath

		// mount script
		tn.Nodes[i].Mounts = append(tn.Nodes[i].Mounts, bindstr)
		// add script execution command
		ncfg := NodeConfig{Name: node.Name, Cmds: []Cmd{}}
		ncfg.Cmds = append(ncfg.Cmds, Cmd{Cmd: execstr})
		tn.NodeConfigs = append(tn.NodeConfigs, ncfg)
	}

	bytes, err := yaml.Marshal(tn)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func GetTinetSpecificationConfig(cfg *model.Config, nm *model.NetworkModel) ([]byte, error) {
	tn, err := getTinetSpecificationBase(cfg, nm)
	if err != nil {
		return nil, err
	}

	for _, node := range tn.Nodes {
		n, exists := nm.NodeByName(node.Name)
		if !exists {
			return nil, fmt.Errorf("node %s not found", node.Name)
		}

		// check switch node configuration is empty
		if node.Type == "switch" && len(n.Commands) > 0 {
			return nil, fmt.Errorf("commands specified for switch node %s", node.Name)
		}

		// add configuration commands to Tn.NodeConfigs
		ncfg := NodeConfig{Name: node.Name, Cmds: []Cmd{}}
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

func getTinetNode(cfg *model.Config, n *model.Node) (Node, error) {
	if n.TinetAttr == nil {
		return Node{}, nil
	}
	mapper := n.TinetAttr

	node := Node{}
	// Node name is empty here, added after checking node type
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

func getTinetInterface(cfg *model.Config, i *model.Interface) (Interface, error) {
	if i.TinetAttr == nil {
		return Interface{}, nil
	}
	mapper := i.TinetAttr

	iface := Interface{
		Name: i.Name,
		Args: i.Opposite.Node.Name + "#" + i.Opposite.Name,
	}
	bytes, err := json.Marshal(mapper)
	if err != nil {
		return Interface{}, err
	}
	err = json.Unmarshal(bytes, &iface)
	if err != nil {
		return Interface{}, err
	}
	return iface, nil
}
