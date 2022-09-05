package model

import (
	"fmt"
	"math"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/goccy/go-yaml"
	// "gopkg.in/yaml.v2"
	// "github.com/spf13/viper"
)

const PathSpecificationDefault string = "default" // search files from working directory
const PathSpecificationLocal string = "local"     // search files from the directory with config file

const DefaultInterfacePrefix string = "net"

const ClassAll string = "all"         // all nodes/interfaces/connections
const ClassDefault string = "default" // all empty nodes/interfaces/connections

const NumberPrefixNode string = "node_"
const NumberPrefixOppositeInterface string = "opp_"
const NumberPrefixOppositeNode string = "oppnode_"

const NumberReplacerName string = "name"
const NumberReplacerIPAddress string = "ipaddr"
const NumberReplacerIPNetwork string = "ipnet"
const NumberReplacerIPLoopback string = "loopback"

const NumberIP string = "ip"
const NumberAS string = "as"

const TargetLocal string = "local"
const TargetFRR string = "frr"

type Config struct {
	GlobalSettings    GlobalSettings    `yaml:"global" mapstructure:"global"`
	NodeClasses       []NodeClass       `yaml:"nodeclass,flow" mapstructure:"nodes,flow"`
	InterfaceClasses  []InterfaceClass  `yaml:"interfaceclass,flow" mapstructure:"interfaces,flow"`
	ConnectionClasses []ConnectionClass `yaml:"connectionclass,flow" mapstructure:"connections,flow"`

	nodeClassMap       map[string]*NodeClass
	interfaceClassMap  map[string]*InterfaceClass
	connectionClassMap map[string]*ConnectionClass
	localDir           string
}

func (cfg *Config) NodeClassByName(name string) (*NodeClass, bool) {
	nc, ok := cfg.nodeClassMap[name]
	return nc, ok
}

func (cfg *Config) InterfaceClassByName(name string) (*InterfaceClass, bool) {
	nc, ok := cfg.interfaceClassMap[name]
	return nc, ok
}

func (cfg *Config) ConnectionClassByName(name string) (*ConnectionClass, bool) {
	cc, ok := cfg.connectionClassMap[name]
	return cc, ok
}

func (cfg *Config) getValidClasses(given []string, hasAll bool, hasDefault bool) []string {
	cnt := len(given)
	if hasAll {
		cnt = cnt + 1
	}
	if len(given) == 0 && hasDefault {
		cnt = cnt + 1
	}
	classes := make([]string, 0, cnt)

	if hasAll {
		classes = append(classes, ClassAll)
	}
	if len(given) == 0 {
		if hasDefault {
			classes = append(classes, ClassDefault)
		}
	} else {
		classes = append(classes, given...)
	}
	return classes
}

func (cfg *Config) getValidNodeClasses(given []string) []string {
	_, hasAllNodeClass := cfg.nodeClassMap[ClassAll]
	_, hasDefaultNodeClass := cfg.nodeClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllNodeClass, hasDefaultNodeClass)
}

func (cfg *Config) getValidInterfaceClasses(given []string) []string {
	_, hasAllInterfaceClass := cfg.interfaceClassMap[ClassAll]
	_, hasDefaultInterfaceClass := cfg.interfaceClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllInterfaceClass, hasDefaultInterfaceClass)
}

func (cfg *Config) getValidConnectionClasses(given []string) []string {
	_, hasAllConnectionClass := cfg.connectionClassMap[ClassAll]
	_, hasDefaultConnectionClass := cfg.connectionClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllConnectionClass, hasDefaultConnectionClass)
}

type GlobalSettings struct {
	IPAddrPool        string `yaml:"ippool" mapstructure:"ippool"`
	IPNetPrefixLength int    `yaml:"ipprefix" mapstructure:"ipprefix"`
	IPLoopbackRange   string `yaml:"iploopback" mapstructure:"iploopback"`
	PathSpecification string `yaml:"path" mapstructure:"path"`
	NodeAutoName      bool   `yaml:"nodeautoname" mapstructure:"nodeautoname"`
}

type NodeClass struct {
	Name            string               `yaml:"name" mapstructure:"name"`
	Prefix          string               `yaml:"prefix" mapstructure:"prefix"` // prefix of auto-naming
	Numbered        []string             `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	ConfigTemplates []NodeConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	// tinet attributes
	Attributes map[string]interface{} `yaml:"attr" mapstructure:"attr"`
}

type NodeConfigTemplate struct {
	Target         string   `yaml:"target" mapstructure:"target"` // config type, such as "shell", "frr", etc.
	Priority       int      `yaml:"priority" mapstructure:"priority"`
	Template       []string `yaml:"template" mapstructure:"template"`
	Filepath       string   `yaml:"filepath" mapstructure:"filepath"`
	parsedTemplate *template.Template
}

type InterfaceClass struct {
	Name            string                    `yaml:"name" mapstructure:"name"`
	Prefix          string                    `yaml:"prefix" mapstructure:"prefix"` // prefix of auto-naming
	Type            string                    `yaml:"type" mapstructure:"type"`     // Tn Interface.Type, prior to ConnectionClass
	Numbered        []string                  `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	ConfigTemplates []InterfaceConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
}

type InterfaceConfigTemplate struct {
	NodeClass      string   `yaml:"node" mapstructure:"node"`     // NodeClass.Name
	Target         string   `yaml:"target" mapstructure:"target"` // config target, such as "shell", "frr", etc.
	Priority       int      `yaml:"priority" mapstructure:"priority"`
	Template       []string `yaml:"template" mapstructure:"template"`
	Filepath       string   `yaml:"filepath" mapstructure:"filepath"`
	parsedTemplate *template.Template
}

type ConnectionClass struct {
	Name            string                     `yaml:"name" mapstructure:"name"`
	Prefix          string                     `yaml:"prefix" mapstructure:"prefix"`               // prefix of interface auto-naming
	Type            string                     `yaml:"type" mapstructure:"type"`                   // Tn Interface.Type
	Numbered        []string                   `yaml:"numbered,flow" mapstructure:"numbered,flow"` // Numbers to be assigned automatically
	ConfigTemplates []ConnectionConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
}

type ConnectionConfigTemplate struct {
	NodeClass      string   `yaml:"node" mapstructure:"node"`     // NodeClass.Name
	Target         string   `yaml:"target" mapstructure:"target"` // config target, such as "shell", "frr", etc.
	Priority       int      `yaml:"priority" mapstructure:"priority"`
	Template       []string `yaml:"template" mapstructure:"template"`
	Filepath       string   `yaml:"filepath" mapstructure:"filepath"`
	parsedTemplate *template.Template
}

type NetworkModel struct {
	Nodes       []Node
	Connections []Connection

	nodeMap map[string]*Node
}

func (nm *NetworkModel) NodeByName(name string) (*Node, bool) {
	node, ok := nm.nodeMap[name]
	return node, ok
}

type Node struct {
	Name            string
	Interfaces      []Interface
	Classes         []string
	Numbered        []string
	Numbers         map[string]string
	RelativeNumbers map[string]string
	Commands        []string

	interfaceMap map[string]*Interface
}

func (n *Node) InterfaceByName(name string) (*Interface, bool) {
	iface, ok := n.interfaceMap[name]
	return iface, ok
}

func (n *Node) HasClass(name string) bool {
	for _, cls := range n.Classes {
		if cls == name {
			return true
		}
	}
	return false
}

//func (n *Node) NextInterfaceName() string {
//	return InterfaceNamePrefix + strconv.Itoa(len(n.Interfaces))
//}

func (n *Node) addNumber(key, val string) {
	n.Numbers[key] = val
	// fmt.Printf("NUMBER %+v %+v=%+v\n", n.Name, key, val)
}

func (n *Node) HasIPLoopback() bool {
	for _, num := range n.Numbered {
		if num == NumberIP {
			return true
		}
	}
	return false
}

type Interface struct {
	Name            string
	Node            *Node
	Classes         []string
	Numbered        []string
	Numbers         map[string]string
	RelativeNumbers map[string]string
	Connection      *Connection
	Opposite        *Interface
}

func (iface *Interface) IsIPAware() bool {
	for _, num := range iface.Numbered {
		if num == NumberIP {
			return true
		}
	}
	return false
}

func (iface *Interface) addNumber(key, val string) {
	iface.Numbers[key] = val
	// fmt.Printf("NUMBER %+v.%+v %+v=%+v\n", iface.Node.Name, iface.Name, key, val)
}

type Connection struct {
	Src     *Interface
	Dst     *Interface
	Classes []string
}

func convertLineFeed(str, lcode string) string {
	return strings.NewReplacer(
		"\r\n", lcode,
		"\r", lcode,
		"\n", lcode,
	).Replace(str)
}

func getPath(path string, cfg *Config) string {
	pathspec := cfg.GlobalSettings.PathSpecification
	if pathspec == "local" {
		return cfg.localDir + "/" + path
	} else {
		return path
	}
}

func LoadConfig(path string) (*Config, error) {

	cfg := Config{}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(bytes, &cfg)
	if err != nil {
		return nil, err
	}

	//viper.SetConfigFile(path)
	//if err := viper.ReadInConfig(); err != nil {
	//	return nil, err
	//}
	//err = viper.Unmarshal(cfg)

	cfg.localDir = filepath.Dir(path)
	cfg.nodeClassMap = map[string]*NodeClass{}
	for i, node := range cfg.NodeClasses {
		cfg.nodeClassMap[node.Name] = &cfg.NodeClasses[i]
	}
	cfg.interfaceClassMap = map[string]*InterfaceClass{}
	for i, iface := range cfg.InterfaceClasses {
		cfg.interfaceClassMap[iface.Name] = &cfg.InterfaceClasses[i]
	}
	cfg.connectionClassMap = map[string]*ConnectionClass{}
	for i, conn := range cfg.ConnectionClasses {
		cfg.connectionClassMap[conn.Name] = &cfg.ConnectionClasses[i]
	}
	return &cfg, err
}

func loadTemplates(cfg *Config) (*Config, error) {
	for i, nc := range cfg.NodeClasses {
		for j, nct := range nc.ConfigTemplates {
			path := ""
			if nct.Filepath != "" {
				path = getPath(nct.Filepath, cfg)
			}
			tpl, err := loadTemplate(nct.Template, path)
			if err != nil {
				return nil, err
			}
			cfg.NodeClasses[i].ConfigTemplates[j].parsedTemplate = tpl
		}
	}
	for i, ic := range cfg.InterfaceClasses {
		for j, ict := range ic.ConfigTemplates {
			path := ""
			if ict.Filepath != "" {
				path = getPath(ict.Filepath, cfg)
			}
			tpl, err := loadTemplate(ict.Template, path)
			if err != nil {
				return nil, err
			}
			cfg.InterfaceClasses[i].ConfigTemplates[j].parsedTemplate = tpl
		}
	}
	for i, cc := range cfg.ConnectionClasses {
		for j, cct := range cc.ConfigTemplates {
			path := ""
			if cct.Filepath != "" {
				path = getPath(cct.Filepath, cfg)
			}
			tpl, err := loadTemplate(cct.Template, path)
			if err != nil {
				return nil, err
			}
			cfg.ConnectionClasses[i].ConfigTemplates[j].parsedTemplate = tpl
		}
	}
	return cfg, nil
}

func loadTemplate(tpl []string, path string) (*template.Template, error) {
	if len(tpl) == 0 && path == "" {
		return nil, fmt.Errorf("empty config template")
	} else if len(tpl) == 0 {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		buf := convertLineFeed(string(bytes), "\n")
		return template.New("").Parse(buf)
	} else if path == "" {
		buf := strings.Join(tpl, "\n")
		return template.New("").Parse(buf)
	} else {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		buf := strings.Join(tpl, "\n") + "\n" + convertLineFeed(string(bytes), "\n")
		return template.New("").Parse(buf)
	}
}

func BuildNetworkModel(cfg *Config, nd *NetworkDiagram) (nm *NetworkModel, err error) {

	// build topology
	nm, err = buildSkeleton(cfg, nd)
	if err != nil {
		return nil, err
	}

	// assign names for unnamed objects in topology
	if cfg.GlobalSettings.NodeAutoName {
		nm, err = assignNodeNames(cfg, nm)
		if err != nil {
			return nil, err
		}
	}
	nm, err = assignInterfaceNames(cfg, nm)
	if err != nil {
		return nil, err
	}

	// assign numbers, interface names and addresses
	nm, err = checkNumbered(cfg, nm)
	if err != nil {
		return nil, err
	}

	nm, err = assignIPAddresses(cfg, nm)
	if err != nil {
		return nil, err
	}

	nm, err = assignNumbers(cfg, nm)
	if err != nil {
		return nil, err
	}

	nm, err = formatNumbers(nm)
	if err != nil {
		return nil, err
	}

	// build config commands from config templates
	cfg, err = loadTemplates(cfg)
	if err != nil {
		return nil, err
	}

	nm, err = generateConfig(cfg, nm)
	if err != nil {
		return nil, err
	}

	return nm, err
}

func buildSkeleton(cfg *Config, nd *NetworkDiagram) (*NetworkModel, error) {
	nm := NetworkModel{}
	allNodes := nd.AllNodes()
	allEdges := nd.AllEdges()

	ifaceCounter := map[string]int{}
	for _, e := range allEdges {
		srcDOTName := e.From().(*DiagramNode).Name
		ifaceCounter[srcDOTName]++
		dstDOTName := e.To().(*DiagramNode).Name
		ifaceCounter[dstDOTName]++
	}

	nm.Nodes = make([]Node, 0, len(allNodes))
	nm.nodeMap = map[string]*Node{}
	for i, n := range allNodes {
		nm.Nodes = append(nm.Nodes, Node{})
		node := &nm.Nodes[len(nm.Nodes)-1]
		node.Name = n.Name // Set DOTID, overwritten later if nodeautoname = true
		nm.nodeMap[node.Name] = &nm.Nodes[i]
		node.Interfaces = make([]Interface, 0, ifaceCounter[n.Name])
		node.Classes = cfg.getValidNodeClasses(n.Classes)
		node.Numbers = map[string]string{}
		node.RelativeNumbers = map[string]string{}
		node.interfaceMap = map[string]*Interface{}

		if len(node.Classes) == 0 {
			return nil, fmt.Errorf("set default nodeclass to leave nodes unlabeled")
		}
	}

	nm.Connections = make([]Connection, 0, len(allEdges))
	for _, e := range allEdges {
		srcNode, ok := nm.NodeByName(e.From().(*DiagramNode).Name)
		if !ok {
			return nil, fmt.Errorf("inconsistent DiagramEdge")
		}
		srcNode.Interfaces = append(srcNode.Interfaces, Interface{})
		srcIf := &srcNode.Interfaces[len(srcNode.Interfaces)-1]
		if e.SrcName != "" {
			srcIf.Name = e.SrcName
			srcNode.interfaceMap[srcIf.Name] = srcIf
		}
		srcIf.Classes = cfg.getValidInterfaceClasses(e.SrcClasses)
		srcIf.Node = srcNode
		srcIf.Numbers = map[string]string{}
		srcIf.RelativeNumbers = map[string]string{}

		dstNode, ok := nm.NodeByName(e.To().(*DiagramNode).Name)
		if !ok {
			return nil, fmt.Errorf("inconsistent DiagramEdge")
		}
		dstNode.Interfaces = append(dstNode.Interfaces, Interface{})
		dstIf := &dstNode.Interfaces[len(dstNode.Interfaces)-1]
		if e.DstName != "" {
			dstIf.Name = e.DstName
			dstNode.interfaceMap[dstIf.Name] = dstIf
		}
		dstIf.Classes = cfg.getValidInterfaceClasses(e.DstClasses)
		dstIf.Node = dstNode
		dstIf.Numbers = map[string]string{}
		dstIf.RelativeNumbers = map[string]string{}

		srcIf.Opposite = dstIf
		dstIf.Opposite = srcIf

		conn := Connection{Src: srcIf, Dst: dstIf}
		conn.Classes = cfg.getValidConnectionClasses(e.Classes)
		nm.Connections = append(nm.Connections, conn)
		srcIf.Connection = &nm.Connections[len(nm.Connections)-1]
		dstIf.Connection = &nm.Connections[len(nm.Connections)-1]

		if (len(srcIf.Classes) == 0 || len(dstIf.Classes) == 0) && len(conn.Classes) == 0 {
			return nil, fmt.Errorf("set default interfaceclass or connectionclass to leave edges unlabeled")
		}
	}

	return &nm, nil
}

func assignNodeNames(cfg *Config, nm *NetworkModel) (*NetworkModel, error) {
	nodePrefixes := map[string][]*Node{}
	for i, node := range nm.Nodes {
		checked := false
		for _, cls := range node.Classes {
			nc, ok := cfg.NodeClassByName(cls)
			if !ok {
				return nil, fmt.Errorf("invalid NodeClass name")
			}
			if nc.Prefix != "" {
				if checked {
					return nil, fmt.Errorf("duplicated node name prefix (classes %+v)", node.Classes)
				} else {
					checked = true
					nodePrefixes[nc.Prefix] = append(nodePrefixes[nc.Prefix], &nm.Nodes[i])
				}
			}
			if !checked {
				return nil, fmt.Errorf("unnamed node without node name prefix (classes %+v)", node.Classes)
			}
		}
	}

	for prefix, nodes := range nodePrefixes {
		for i, node := range nodes {
			node.Name = prefix + strconv.Itoa(i+1) // starts with 1
			nm.nodeMap[node.Name] = node
		}
	}

	return nm, nil
}

func assignInterfaceNames(cfg *Config, nm *NetworkModel) (*NetworkModel, error) {
	ifacePrefixes := map[string]map[*Interface]string{}

	for _, conn := range nm.Connections {
		checked := false
		for _, cls := range conn.Classes {
			cc, ok := cfg.ConnectionClassByName(cls)
			if !ok {
				return nil, fmt.Errorf("invalid InterfaceClass name")
			}
			if cc.Prefix != "" {
				if checked {
					return nil, fmt.Errorf("duplicated interface name prefix (connection classes %+v)", conn.Classes)
				}
				checked = true
				if conn.Src.Name == "" {
					if _, ok := ifacePrefixes[conn.Src.Node.Name]; !ok {
						ifacePrefixes[conn.Src.Node.Name] = map[*Interface]string{}
					}
					ifacePrefixes[conn.Src.Node.Name][conn.Src] = cc.Prefix
				}
				if conn.Dst.Name == "" {
					if _, ok := ifacePrefixes[conn.Dst.Node.Name]; !ok {
						ifacePrefixes[conn.Dst.Node.Name] = map[*Interface]string{}
					}
					ifacePrefixes[conn.Dst.Node.Name][conn.Dst] = cc.Prefix
				}
			}
		}
	}

	for i, node := range nm.Nodes {
		existingNames := map[string]struct{}{}
		for j, iface := range node.Interfaces {
			if iface.Name != "" {
				existingNames[iface.Name] = struct{}{}
				continue
			}
			checked := false
			for _, cls := range iface.Classes {
				ic, ok := cfg.InterfaceClassByName(cls)
				if !ok {
					return nil, fmt.Errorf("invalid InterfaceClass name")
				}
				if ic.Prefix != "" {
					if checked {
						return nil, fmt.Errorf("duplicated interface name prefix (classes %+v)", node.Classes)
						// } else if ifacePrefixes[node.Name][iface.Name] != "" {
						// TODO show warnings
						// InterfaceClass prefix is prior to ConnectionClass prefix
					}
					checked = true
					if _, ok := ifacePrefixes[node.Name]; !ok {
						ifacePrefixes[node.Name] = map[*Interface]string{}
					}
					ifacePrefixes[node.Name][&nm.Nodes[i].Interfaces[j]] = ic.Prefix
				}
			}
		}

		prefixMap := map[string][]*Interface{}
		for j := range node.Interfaces {
			iface := &nm.Nodes[i].Interfaces[j]
			prefix, exists := ifacePrefixes[node.Name][iface]
			if iface.Name == "" {
				if prefix == "" || !exists {
					prefix = DefaultInterfacePrefix
				}
				prefixMap[prefix] = append(prefixMap[prefix], iface)
			}
		}
		for prefix, interfaces := range prefixMap {
			i := 0
			for _, iface := range interfaces {
				var name string
				for { // avoid existing names
					name = prefix + strconv.Itoa(i)
					_, exists := existingNames[name]
					if !exists {
						break
					}
					i++ // starts with 0
				}
				iface.Name = name
				iface.Node.interfaceMap[iface.Name] = iface
				i++
			}
		}
	}

	// confirm all interfaces are named
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			if iface.Name == "" {
				return nil, fmt.Errorf("still exists unnamed interfaces")
			}
		}
	}

	return nm, nil
}

/*
checkNumbered put numbered-flags to Nodes, Interfaces, and Connections in NetworkModel.
It searches all classes in Config and remove duplication.
Numbered of Interfaces will be affected from both InterfaceClasses and ConnectionClasses.
*/
func checkNumbered(cfg *Config, nm *NetworkModel) (*NetworkModel, error) {
	for i, node := range nm.Nodes {
		nodeNumberedSet := map[string]struct{}{}
		for _, cls := range node.Classes {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return nil, fmt.Errorf("invalid NodeClass name %v", cls)
			}
			for _, num := range nc.Numbered {
				nodeNumberedSet[num] = struct{}{}
			}
		}
		nodeNumbered := make([]string, 0, len(nodeNumberedSet))
		for num := range nodeNumberedSet {
			nodeNumbered = append(nodeNumbered, num)
		}
		nm.Nodes[i].Numbered = nodeNumbered

		for j, iface := range node.Interfaces {
			ifaceNumberedSet := map[string]struct{}{}
			for _, cls := range iface.Classes {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return nil, fmt.Errorf("invalid InterfaceClass name %v", cls)
				}
				for _, num := range ic.Numbered {
					ifaceNumberedSet[num] = struct{}{}
				}
			}
			ifaceNumbered := make([]string, 0, len(ifaceNumberedSet))
			for num := range ifaceNumberedSet {
				ifaceNumbered = append(ifaceNumbered, num)
			}
			nm.Nodes[i].Interfaces[j].Numbered = ifaceNumbered
		}
	}

	for _, conn := range nm.Connections {
		srcNumberedSet := map[string]struct{}{}
		dstNumberedSet := map[string]struct{}{}
		for _, num := range conn.Src.Numbered {
			srcNumberedSet[num] = struct{}{}
		}
		for _, num := range conn.Dst.Numbered {
			dstNumberedSet[num] = struct{}{}
		}

		for _, cls := range conn.Classes {
			cc, ok := cfg.connectionClassMap[cls]
			if !ok {
				return nil, fmt.Errorf("invalid ConnectionClass name %v", cls)
			}
			for _, num := range cc.Numbered {
				srcNumberedSet[num] = struct{}{}
				dstNumberedSet[num] = struct{}{}
			}
		}

		srcNumbered := make([]string, 0, len(srcNumberedSet))
		for num := range srcNumberedSet {
			srcNumbered = append(srcNumbered, num)
		}
		conn.Src.Numbered = srcNumbered

		dstNumbered := make([]string, 0, len(dstNumberedSet))
		for num := range dstNumberedSet {
			dstNumbered = append(dstNumbered, num)
		}
		conn.Dst.Numbered = dstNumbered

	}

	return nm, nil
}

func assignNumbers(cfg *Config, nm *NetworkModel) (*NetworkModel, error) {
	nodesForNumbers := map[string][]*Node{}
	interfacesForNumbers := map[string][]*Interface{}
	for i, node := range nm.Nodes {
		nm.Nodes[i].addNumber(NumberReplacerName, node.Name)
		for _, num := range node.Numbered {
			nodesForNumbers[num] = append(nodesForNumbers[num], &nm.Nodes[i])
		}
		for j, iface := range node.Interfaces {
			nm.Nodes[i].Interfaces[j].addNumber(NumberReplacerName, iface.Name)
			for _, num := range iface.Numbered {
				interfacesForNumbers[num] = append(interfacesForNumbers[num], &nm.Nodes[i].Interfaces[j])
			}
		}
	}

	for num, nodes := range nodesForNumbers {
		cnt := len(nodes)
		switch num {
		case NumberIP:
			// Assigned by AssignIPAddresses, ignore
		case NumberAS:
			asnumbers, err := getASNumber(cnt)
			if err != nil {
				return nil, err
			}
			for nid, node := range nodes {
				val := strconv.Itoa(asnumbers[nid])
				node.addNumber(num, val)
			}
		default:
			return nil, fmt.Errorf("not implemented")
		}
	}

	for num, ifaces := range interfacesForNumbers {
		cnt := len(ifaces)
		switch num {
		case NumberIP:
			// Assigned by AssignIPAddresses, ignore
		default:
			// TODO aasign customized numbers
			fmt.Printf("cnt %v", cnt)
			return nil, fmt.Errorf("not implemented")
		}
	}

	return nm, nil
}

func formatNumbers(nm *NetworkModel) (*NetworkModel, error) {
	for i, node := range nm.Nodes {
		// node self
		for num, val := range node.Numbers {
			nm.Nodes[i].RelativeNumbers[num] = val
		}

		for j, iface := range node.Interfaces {
			target := nm.Nodes[i].Interfaces[j]

			// interface self
			for num, val := range iface.Numbers {
				target.RelativeNumbers[num] = val
			}

			// node of interface
			for nodenum, val := range node.Numbers {
				num := NumberPrefixNode + nodenum
				target.RelativeNumbers[num] = val
			}

			// opposite interface
			oppIf := iface.Opposite
			for oppnum, val := range oppIf.Numbers {
				num := NumberPrefixOppositeInterface + oppnum
				target.RelativeNumbers[num] = val
			}

			// node of opposite interface
			oppNode := oppIf.Node
			for oppnnum, val := range oppNode.Numbers {
				num := NumberPrefixOppositeNode + oppnnum
				target.RelativeNumbers[num] = val
			}
		}
	}
	return nm, nil
}

func assignIPAddresses(cfg *Config, nm *NetworkModel) (*NetworkModel, error) {
	// search ip loopbacks
	allLoopbacks := []*Node{}
	for i, node := range nm.Nodes {
		if node.HasIPLoopback() {
			allLoopbacks = append(allLoopbacks, &nm.Nodes[i])
		}
	}

	// search networks
	allNetworkInterfaces := [][]*Interface{}
	checked := map[*Connection]struct{}{} // set alternative
	for i, conn := range nm.Connections {
		if _, ok := checked[&nm.Connections[i]]; ok {
			break
		}
		checked[&nm.Connections[i]] = struct{}{}
		networkInterfaces := []*Interface{}
		todo := []*Interface{conn.Dst, conn.Src} // stack (Last In First Out)
		for len(todo) > 0 {
			iface := todo[len(todo)-1]
			todo = todo[:len(todo)-1] // pop iface from todo
			if iface.IsIPAware() {
				// ip aware -> network ends
				networkInterfaces = append(networkInterfaces, iface)
			} else {
				// ip unaware -> search adjacent interfaces
				for _, nextIf := range iface.Node.Interfaces {
					if _, ok := checked[nextIf.Connection]; ok {
						// already checked network, something wrong
						return nil, fmt.Errorf("IPaddress assignment algorithm panic")
					}
					checked[nextIf.Connection] = struct{}{}
					if nextIf.Name == iface.Name {
						// ignore iface itself
					} else if nextIf.IsIPAware() {
						// ip aware -> network ends
						networkInterfaces = append(networkInterfaces, iface)
					} else {
						// ip unaware -> search adjacent connection
						oppIf := nextIf.Opposite
						todo = append(todo, oppIf)
					}
				}
			}
		}
		allNetworkInterfaces = append(allNetworkInterfaces, networkInterfaces)
	}

	// assign loopback addresses to nodes
	loopbackrange, err := netip.ParsePrefix(cfg.GlobalSettings.IPLoopbackRange)
	if err != nil {
		return nil, err
	}
	addrs, err := getIPAddr(loopbackrange, len(allLoopbacks))
	if err != nil {
		return nil, err
	}
	for i, node := range allLoopbacks {
		node.addNumber(NumberReplacerIPLoopback, addrs[i].String())
	}

	// assign network addresses to networks
	poolrange, err := netip.ParsePrefix(cfg.GlobalSettings.IPAddrPool)
	if err != nil {
		return nil, err
	}
	bits := cfg.GlobalSettings.IPNetPrefixLength
	cnt := len(allNetworkInterfaces)
	pool, err := getIPAddrPool(poolrange, bits, cnt)
	if err != nil {
		return nil, err
	}

	// assign ip addressees to interfaces
	for netid, networkInterfaces := range allNetworkInterfaces {
		net := pool[netid]
		addrs, err := getIPAddr(net, len(networkInterfaces))
		if err != nil {
			return nil, err
		}
		for ifid, iface := range networkInterfaces {
			iface.addNumber(NumberReplacerIPAddress, addrs[ifid].String())
			iface.addNumber(NumberReplacerIPNetwork, net.String())
		}
	}

	return nm, nil
}

/*
getIPAddrPool generates set of prefixes

Arguments:

	poolrange: Source IP address space range
	bits: Prefix length of prefixes to generate
	cnt: Number of prefixes to generate. If 0 or smaller, all possible prefixes are returned.
*/
func getIPAddrPool(poolrange netip.Prefix, bits int, cnt int) ([]netip.Prefix, error) {
	pbits := poolrange.Bits()
	err_too_small := fmt.Errorf("IPAddrPoolRange is too small")

	if pbits > bits { // pool range is smaller
		return nil, err_too_small
	} else if pbits == bits {
		if cnt > 1 {
			return nil, err_too_small
		} else {
			return []netip.Prefix{poolrange}, nil
		}
	} else { // pbits < bits
		// calculate number of prefixes to generate
		potential := int(math.Pow(2, float64(bits-pbits)))
		if cnt <= 0 {
			cnt = potential
		} else if cnt > potential {
			return nil, err_too_small
		}
		var pool = make([]netip.Prefix, 0, cnt)

		// add first prefix
		new_prefix := netip.PrefixFrom(poolrange.Addr(), bits)
		pool = append(pool, new_prefix)

		// calculate following prefixes
		current_slice := poolrange.Addr().AsSlice()
		for i := 0; i < cnt-1; i++ { // pool addr index
			byte_idx := bits / 8
			byte_increase := int(math.Pow(2, float64(8-bits%8)))
			for byte_idx > 0 { // byte index to modify
				tmp_sum := int(current_slice[byte_idx]) + byte_increase
				if tmp_sum >= 256 {
					current_slice[byte_idx] = byte(tmp_sum - 256)
					byte_idx = byte_idx - 1
					byte_increase = 1
				} else {
					current_slice[byte_idx] = byte(tmp_sum)
					break
				}
			}
			new_addr, ok := netip.AddrFromSlice(current_slice)
			if ok {
				new_prefix = netip.PrefixFrom(new_addr, bits)
				pool = append(pool, new_prefix)
			} else {
				return pool, fmt.Errorf("format error in address pool calculation")
			}
		}
		return pool, nil
	}

}

func getIPAddr(pool netip.Prefix, cnt int) ([]netip.Addr, error) {
	var potential int
	err_too_small := fmt.Errorf("addr pool is too small")

	// calculate number of addresses to generate
	if pool.Addr().Is4() {
		// IPv4: skip network address and broadcast address
		potential = int(math.Pow(2, float64(32-pool.Bits()))) - 2
	} else {
		// IPv6: skip network address
		potential = int(math.Pow(2, float64(128-pool.Bits()))) - 1
	}
	if cnt <= 0 {
		cnt = potential
	} else if cnt > potential {
		return nil, err_too_small
	}

	// generate addresses
	var addrs = make([]netip.Addr, 0, cnt)
	current_addr := pool.Addr()
	for i := 0; i < cnt; i++ { // pool addr index
		current_addr = current_addr.Next()
		addrs = append(addrs, current_addr)
	}
	return addrs, nil
}

func getASNumber(cnt int) ([]int, error) {
	var asnumbers = make([]int, 0, cnt)
	if cnt <= 535 {
		for i := 0; i < cnt; i++ {
			asnumbers = append(asnumbers, 65001+i)
		}
	} else if cnt <= 1024 {
		for i := 0; i < cnt; i++ {
			asnumbers = append(asnumbers, 64512+i)
		}
	} else { // cnt > 1024
		// currently returns error
		return nil, fmt.Errorf("requested more than 1024 private AS numbers")
	}
	return asnumbers, nil
}

func generateConfig(cfg *Config, nm *NetworkModel) (*NetworkModel, error) {
	for i, node := range nm.Nodes {
		configBlocks := []string{}
		configTarget := []string{}
		configPriority := []int{}
		for _, cls := range node.Classes {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return nil, fmt.Errorf("undefined NodeClass name %v", cls)
			}
			for _, nct := range nc.ConfigTemplates {
				block, err := getConfig(nct.parsedTemplate, node.RelativeNumbers)
				if err != nil {
					return nil, err
				}
				configBlocks = append(configBlocks, block)
				configTarget = append(configTarget, nct.Target)
				configPriority = append(configPriority, nct.Priority)
			}
		}

		for _, iface := range node.Interfaces {
			for _, cls := range iface.Classes {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return nil, fmt.Errorf("undefined InterfaceClass name %v", cls)
				}
				for _, ict := range ic.ConfigTemplates {
					if !(ict.NodeClass == "" || node.HasClass(ict.NodeClass)) {
						continue
					}
					block, err := getConfig(ict.parsedTemplate, iface.RelativeNumbers)
					if err != nil {
						return nil, err
					}
					configBlocks = append(configBlocks, block)
					configTarget = append(configTarget, ict.Target)
					configPriority = append(configPriority, ict.Priority)
				}
			}

			for _, cls := range iface.Connection.Classes {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return nil, fmt.Errorf("undefined ConnectionClass name %v", cls)
				}
				for _, cct := range cc.ConfigTemplates {
					if !(cct.NodeClass == "" || node.HasClass(cct.NodeClass)) {
						continue
					}
					block, err := getConfig(cct.parsedTemplate, iface.RelativeNumbers)
					if err != nil {
						return nil, err
					}
					configBlocks = append(configBlocks, block)
					configTarget = append(configTarget, cct.Target)
					configPriority = append(configPriority, cct.Priority)
				}
			}
		}
		commands, err := sortConfig(configBlocks, configTarget, configPriority)
		if err != nil {
			return nil, err
		}
		nm.Nodes[i].Commands = commands
	}
	return nm, nil
}

func getConfig(tpl *template.Template, numbers map[string]string) (string, error) {
	writer := new(strings.Builder)
	err := tpl.Execute(writer, numbers)
	if err != nil {
		return "", err
	}
	return writer.String(), nil
}

func sortConfig(configBlocks []string, configTarget []string, configPriority []int) ([]string, error) {
	if !(len(configBlocks) == len(configTarget) && len(configBlocks) == len(configPriority)) {
		return nil, fmt.Errorf("different length of config components, unable to sort")
	}

	configIndex := make([]int, 0, len(configBlocks))
	for i := 0; i < len(configBlocks); i++ {
		configIndex = append(configIndex, i)
	}

	// large priority to head
	sort.SliceStable(configIndex, func(i, j int) bool {
		return configPriority[configIndex[i]] > configPriority[configIndex[j]]
	})

	allCommands := []string{}
	for _, index := range configIndex {
		target := configTarget[index]
		commands, err := configByTarget(configBlocks[index], target)
		if err != nil {
			return nil, err
		}
		allCommands = append(allCommands, commands...)
	}

	// prev := ""
	// carry := []string{}
	// allCommands := []string{}
	// for i, index := range configIndex {
	// 	// combine config blocks of same target values
	// 	target := configTarget[index]
	// 	if i > 0 && prev != target {
	// 		commands, err := configByTarget(strings.Join(carry, "\n"), prev)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		allCommands = append(allCommands, commands...)
	// 		carry = []string{}
	// 	}
	// 	carry = append(carry, configBlocks[index])
	// 	if i == len(configBlocks)-1 {
	// 		commands, err := configByTarget(strings.Join(carry, "\n"), target)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		allCommands = append(allCommands, commands...)
	// 		carry = []string{}
	// 	}
	// 	prev = target
	// }
	// if len(carry) > 0 {
	// 	return nil, fmt.Errorf("loop error, sanity check failure")
	// }

	return allCommands, nil

}

func configByTarget(config string, target string) ([]string, error) {
	if target == "" {
		target = TargetLocal
	}

	var commands []string
	switch target {
	case TargetLocal:
		commands = strings.Split(config, "\n")
	case TargetFRR:
		lines := []string{"conf t"}
		lines = append(lines, strings.Split(config, "\n")...)
		cmd := "vtysh -c \"" + strings.Join(lines, "\" -c \"") + "\""
		commands = []string{cmd}
	}
	return commands, nil
}
