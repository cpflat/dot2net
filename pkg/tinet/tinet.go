package tinet

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/cpflat/dot2net/pkg/model"
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
	Interfaces []Interface `yaml:"interfaces,flow,omitempty" mapstructure:"interfaces,flow,omitempty"`
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
		// skip virtual nodes
		if n.Virtual {
			continue
		}

		node, err := getTinetNode(n)
		if err != nil {
			return nil, err
		}
		ifaces := []Interface{}
		for _, i := range n.Interfaces {

			// skip virtual interface
			if i.Virtual {
				continue
			}

			iface, err := getTinetInterface(i)
			if err != nil {
				return nil, err
			}
			if iface.Type == "" {
				iface.Type = "direct"
			}
			ifaces = append(ifaces, iface)
		}

		// add mount points for file outputs
		for _, filename := range n.Files.FileNames() {
			file := n.Files.GetFile(filename)
			if file.FileDefinition.Path == "" {
				continue
			}
			//dirpath, err := cfg.MountSourcePath(node.Name)
			dirpath, err := filepath.Abs(n.Name)
			if err != nil {
				return nil, fmt.Errorf("path handling panic for %s", node.Name)
			}
			// dirpath, err := filepath.Abs(n.Name) // requires absolute path
			// if err != nil {
			// 	return nil, fmt.Errorf("directory path panic")
			// }
			//dirpath = strings.TrimRight(dirpath, "/")
			cfgpath := filepath.Join(dirpath, file.FileDefinition.Name)
			targetpath := file.FileDefinition.Path
			bindstr := cfgpath + ":" + targetpath
			node.Mounts = append(node.Mounts, bindstr)
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

		embed := n.Files.GetEmbeddedConfig()

		if embed != nil {
			// check switch node configuration is empty
			if node.Type == "switch" {
				return nil, fmt.Errorf("commands are specified for switch-type node %s", node.Name)
			}

			// add configuration commands to Tn.NodeConfigs
			ncfg := NodeConfig{Name: node.Name, Cmds: []Cmd{}}
			for _, line := range embed.Content {
				ncfg.Cmds = append(ncfg.Cmds, Cmd{Cmd: line})
			}
			tn.NodeConfigs = append(tn.NodeConfigs, ncfg)
		}
	}

	bytes, err := yaml.Marshal(tn)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func getTinetNode(n *model.Node) (Node, error) {
	node := Node{}

	mapper := n.TinetAttr
	if n.TinetAttr == nil {
		return Node{}, nil
	}
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

func getTinetInterface(i *model.Interface) (Interface, error) {
	iface := Interface{
		Name: i.Name,
	}
	if i.Opposite != nil {
		iface.Args = i.Opposite.Node.Name + "#" + i.Opposite.Name
	}

	mapper := i.TinetAttr
	if i.TinetAttr == nil {
		return iface, nil
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
