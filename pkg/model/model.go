package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/goccy/go-yaml"
	// "gopkg.in/yaml.v2"
	// "github.com/spf13/viper"
)

const PathSpecificationDefault string = "default" // search files from working directory
const PathSpecificationLocal string = "local"     // search files from the directory with config file

const DefaultInterfacePrefix string = "net"

const ClassAll string = "all"         // all nodes/interfaces/connections
const ClassDefault string = "default" // all empty nodes/interfaces/connections
const PlaceLabelPrefix string = "@"
const ValueLabelSeparator string = "="

const NumberSeparator string = "_"
const NumberPrefixNode string = "node_"
const NumberPrefixOppositeInterface string = "opp_"
const NumberPrefixOppositeNode string = "oppnode_"

const NumberReplacerName string = "name"

// IP number replacer: [IPSpace]_[IPReplacerXX]
const IPLoopbackReplacerFooter string = "loopback"
const IPAddressReplacerFooter string = "addr"
const IPNetworkReplacerFooter string = "net"
const IPPrefixLengthReplacerFooter string = "plen"

const NumberIP string = "ip" // available for both v4 and v6
const NumberIPv4 string = "ipv4"
const NumberIPv6 string = "ipv6"
const NumberAS string = "as"

const DummyIPSpace string = "dummy"

const TargetLocal string = "local"
const TargetFRR string = "frr"

type Config struct {
	GlobalSettings     GlobalSettings      `yaml:"global" mapstructure:"global"`
	IPSpaceDefinitions []IPSpaceDefinition `yaml:"ipspace" mapstructure:"ipspace"`
	NodeClasses        []NodeClass         `yaml:"nodeclass,flow" mapstructure:"nodes,flow"`
	InterfaceClasses   []InterfaceClass    `yaml:"interfaceclass,flow" mapstructure:"interfaces,flow"`
	ConnectionClasses  []ConnectionClass   `yaml:"connectionclass,flow" mapstructure:"connections,flow"`

	ipSpaceDefinitionMap map[string]*IPSpaceDefinition
	nodeClassMap         map[string]*NodeClass
	interfaceClassMap    map[string]*InterfaceClass
	connectionClassMap   map[string]*ConnectionClass
	localDir             string
}

func (cfg *Config) IPSpaceDefinitionByName(name string) (*IPSpaceDefinition, bool) {
	ipspace, ok := cfg.ipSpaceDefinitionMap[name]
	return ipspace, ok
}

func (cfg *Config) NodeClassByName(name string) (*NodeClass, bool) {
	nc, ok := cfg.nodeClassMap[name]
	return nc, ok
}

func (cfg *Config) InterfaceClassByName(name string) (*InterfaceClass, bool) {
	ic, ok := cfg.interfaceClassMap[name]
	return ic, ok
}

func (cfg *Config) ConnectionClassByName(name string) (*ConnectionClass, bool) {
	cc, ok := cfg.connectionClassMap[name]
	return cc, ok
}

func (cfg *Config) IPSpaceNames() []string {
	names := make([]string, len(cfg.IPSpaceDefinitions), 0)
	for _, ipspace := range cfg.IPSpaceDefinitions {
		names = append(names, ipspace.Name)
	}
	return names
}

func (cfg *Config) DefaultIPAware() []string {
	spaces := make([]string, 0, len(cfg.IPSpaceDefinitions))
	for _, ipspace := range cfg.IPSpaceDefinitions {
		if ipspace.DefaultAware {
			spaces = append(spaces, ipspace.Name)
		}
	}
	return spaces
}

func (cfg *Config) DefaultIPConnect() []string {
	spaces := make([]string, 0, len(cfg.IPSpaceDefinitions))
	for _, ipspace := range cfg.IPSpaceDefinitions {
		if ipspace.DefaultConnect {
			spaces = append(spaces, ipspace.Name)
		}
	}
	return spaces
}

func (cfg *Config) classifyLabels(given []string) parsedLabels {
	pl := parsedLabels{}
	pl.ValueLabels = map[string]string{}
	pl.MetaValueLabels = map[string]string{}
	for _, label := range given {
		if label == "" {
		} else if strings.HasPrefix(label, PlaceLabelPrefix) {
			if strings.Contains(label, ValueLabelSeparator) {
				// with "@" and include "=" -> MetaValueLabel
				sep := strings.SplitN(strings.TrimPrefix(label, PlaceLabelPrefix), ValueLabelSeparator, 2)
				mvlabel := sep[0]
				value := sep[1]
				pl.MetaValueLabels[mvlabel] = value
			} else {
				// with "@" -> PlaceLabel
				plabel := strings.TrimPrefix(label, PlaceLabelPrefix)
				pl.PlaceLabels = append(pl.PlaceLabels, plabel)
			}
		} else {
			if strings.Contains(label, ValueLabelSeparator) {
				// include "=" -> ValueLabel
				sep := strings.SplitN(label, ValueLabelSeparator, 2)
				vlabel := sep[0]
				value := sep[1]
				pl.ValueLabels[vlabel] = value
			} else {
				// ClassLabel
				pl.ClassLabels = append(pl.ClassLabels, label)
			}
		}
	}
	return pl
}

func (cfg *Config) getValidClasses(given []string, hasAll bool, hasDefault bool) parsedLabels {
	pl := cfg.classifyLabels(given)
	classLabels := pl.ClassLabels

	cnt := len(classLabels)
	if hasAll {
		cnt = cnt + 1
	}
	if len(classLabels) == 0 && hasDefault {
		cnt = cnt + 1
	}
	classes := make([]string, 0, cnt)

	if hasAll {
		classes = append(classes, ClassAll)
	}
	if len(classLabels) == 0 {
		if hasDefault {
			classes = append(classes, ClassDefault)
		}
	} else {
		classes = append(classes, classLabels...)
	}

	pl.ClassLabels = classes
	return pl
}

func (cfg *Config) getValidNodeClasses(given []string) parsedLabels {
	_, hasAllNodeClass := cfg.nodeClassMap[ClassAll]
	_, hasDefaultNodeClass := cfg.nodeClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllNodeClass, hasDefaultNodeClass)
}

func (cfg *Config) getValidInterfaceClasses(given []string) parsedLabels {
	_, hasAllInterfaceClass := cfg.interfaceClassMap[ClassAll]
	_, hasDefaultInterfaceClass := cfg.interfaceClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllInterfaceClass, hasDefaultInterfaceClass)
}

func (cfg *Config) getValidConnectionClasses(given []string) parsedLabels {
	_, hasAllConnectionClass := cfg.connectionClassMap[ClassAll]
	_, hasDefaultConnectionClass := cfg.connectionClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllConnectionClass, hasDefaultConnectionClass)
}

type GlobalSettings struct {
	PathSpecification string `yaml:"path" mapstructure:"path"`
	NodeAutoName      bool   `yaml:"nodeautoname" mapstructure:"nodeautoname"`

	ClabAttr map[string]interface{} `yaml:"clab" mapstructure:"clab"` // containerlab attributes
}

type IPSpaceDefinition struct {
	Name                string `yaml:"name" mapstructure:"name"`
	AddrRange           string `yaml:"range" mapstructure:"range"`
	LoopbackRange       string `yaml:"loopback_range" mapstructure:"loopback_range"`
	DefaultPrefixLength int    `yaml:"prefix" mapstructure:"prefix"`
	// If default_aware is true, classes without ipaware field are considered as aware of this IPSpace
	DefaultAware bool `yaml:"default_aware" mapstructure:"default_aware"`
	// If default_connect is true, ConnectionClasses without ipspaces field are considered as connected on this IPSpace
	DefaultConnect bool `yaml:"default_connect" mapstructure:"default_connect"`
}

func (ipspace *IPSpaceDefinition) IPAddressReplacer() string {
	return ipspace.Name + "_" + IPAddressReplacerFooter
}

func (ipspace *IPSpaceDefinition) IPNetworkReplacer() string {
	return ipspace.Name + "_" + IPNetworkReplacerFooter
}

func (ipspace *IPSpaceDefinition) IPPrefixLengthReplacer() string {
	return ipspace.Name + "_" + IPPrefixLengthReplacerFooter
}

func (ipspace *IPSpaceDefinition) IPLoopbackReplacer() string {
	return ipspace.Name + "_" + IPLoopbackReplacerFooter
}

type NodeClass struct {
	Name            string               `yaml:"name" mapstructure:"name"`
	Prefix          string               `yaml:"prefix" mapstructure:"prefix"`   // prefix of auto-naming
	IPAware         []string             `yaml:"ipaware" mapstructure:"ipaware"` // aware ip spaces for loopback
	Numbered        []string             `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	ConfigTemplates []NodeConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"` // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`   // containerlab attributes
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
	Prefix          string                    `yaml:"prefix" mapstructure:"prefix"`   // prefix of auto-naming
	Type            string                    `yaml:"type" mapstructure:"type"`       // Tn Interface.Type, prior to ConnectionClass
	IPAware         []string                  `yaml:"ipaware" mapstructure:"ipaware"` // aware ip spaces
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
	IPAware         []string                   `yaml:"ipaware,flow" mapstructure:"ipaware,flow"`   // aware ip spaces for end interfaces
	IPSpaces        []string                   `yaml:"ipspaces,flow" mapstructure:"ipspaces,flow"` // Connection is limited to specified spaces
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

type parsedLabels struct {
	ClassLabels     []string
	PlaceLabels     []string
	ValueLabels     map[string]string
	MetaValueLabels map[string]string
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

/*
checkFlags put flags to Nodes, Interfaces, and Connections in NetworkModel.
It searches all classes in Config and remove duplication.
Node: IPAware, Numbered
Interface: IPAware, Numbered
Connection: IPSpaces
Flags of Interfaces will be affected from both InterfaceClasses and ConnectionClasses.
*/
func (nm *NetworkModel) checkFlags(cfg *Config) error {
	defaultIPAware := cfg.DefaultIPAware()
	defaultIPConnect := cfg.DefaultIPConnect()

	for i, node := range nm.Nodes {
		nm.Nodes[i].IPAware = mapset.NewSet[string]()
		nm.Nodes[i].Numbered = mapset.NewSet[string]()
		// add defaults of node loopback ipaware
		for _, space := range defaultIPAware {
			nm.Nodes[i].IPAware.Add(space)
		}
		// check nodeclass flags
		err := nm.Nodes[i].checkFlags(cfg)
		if err != nil {
			return err
		}

		for j := range node.Interfaces {
			nm.Nodes[i].Interfaces[j].IPAware = mapset.NewSet[string]()
			nm.Nodes[i].Interfaces[j].Numbered = mapset.NewSet[string]()
			// add defaults of interface ipaware (added by Interface.checkFlags and Connection.checkFlags)
			for _, space := range defaultIPAware {
				nm.Nodes[i].Interfaces[j].IPAware.Add(space)
			}
			// check interfaceclass flags
			err = nm.Nodes[i].Interfaces[j].checkFlags(cfg)
			if err != nil {
				return err
			}
		}
	}

	for i := range nm.Connections {
		nm.Connections[i].IPSpaces = mapset.NewSet[string]()
		for _, space := range defaultIPConnect {
			nm.Connections[i].IPSpaces.Add(space)
		}
		// check connectionclass flags to connections and their interfaces
		err := nm.Connections[i].checkFlags(cfg)
		if err != nil {
			return err
		}
	}

	return nil
}

type Node struct {
	Name            string
	Interfaces      []Interface
	Labels          parsedLabels
	IPAware         mapset.Set[string] // Aware IPspaces for loopbacks
	Numbered        mapset.Set[string]
	Numbers         map[string]string
	RelativeNumbers map[string]string
	Commands        []string

	interfaceMap map[string]*Interface
}

func (n *Node) HasClass(name string) bool {
	for _, cls := range n.Labels.ClassLabels {
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

func (n *Node) GivenIPLoopback(ipspace IPSpaceDefinition) (string, bool) {
	for k, v := range n.Labels.ValueLabels {
		if k == ipspace.IPLoopbackReplacer() {
			return v, true
		}
	}
	return "", false
}

func (n *Node) checkFlags(cfg *Config) error {
	for _, cls := range n.Labels.ClassLabels {
		nc, ok := cfg.nodeClassMap[cls]
		if !ok {
			return fmt.Errorf("invalid NodeClass name %+v", cls)
		}

		// check IP aware
		for _, space := range nc.IPAware {
			n.IPAware.Add(space)
		}

		// check numbered
		for _, num := range nc.Numbered {
			n.Numbered.Add(num)
		}
	}
	return nil
}

type Interface struct {
	Name            string
	Node            *Node
	Labels          parsedLabels
	IPAware         mapset.Set[string] // Aware IPSpaces for IP address assignment
	Numbered        mapset.Set[string]
	Numbers         map[string]string
	RelativeNumbers map[string]string
	Connection      *Connection
	Opposite        *Interface
}

func (iface *Interface) GivenIPAddress(ipspace IPSpaceDefinition) (string, bool) {
	for k, v := range iface.Labels.ValueLabels {
		if k == ipspace.IPAddressReplacer() {
			return v, true
		}
	}
	return "", false
}

func (iface *Interface) addNumber(key, val string) {
	iface.Numbers[key] = val
	// fmt.Printf("NUMBER %+v.%+v %+v=%+v\n", iface.Node.Name, iface.Name, key, val)
}

func (iface *Interface) checkFlags(cfg *Config) error {
	for _, cls := range iface.Labels.ClassLabels {
		ic, ok := cfg.interfaceClassMap[cls]
		if !ok {
			return fmt.Errorf("invalid InterfaceClass name %+v", cls)
		}

		// ip spaces
		for _, space := range ic.IPAware {
			iface.IPAware.Add(space)
		}

		// ip numbered
		for _, num := range ic.Numbered {
			iface.Numbered.Add(num)
		}
	}

	return nil
}

type Connection struct {
	Src      *Interface
	Dst      *Interface
	Labels   parsedLabels
	IPSpaces mapset.Set[string]
}

func (conn *Connection) GivenIPNetwork(ipspace IPSpaceDefinition) (string, bool) {
	for k, v := range conn.Labels.ValueLabels {
		if k == ipspace.IPNetworkReplacer() {
			return v, true
		}
	}
	return "", false
}

func (conn *Connection) checkFlags(cfg *Config) error {

	for _, cls := range conn.Labels.ClassLabels {
		cc, ok := cfg.connectionClassMap[cls]
		if !ok {
			return fmt.Errorf("invalid ConnectionClass name %+v", cls)
		}

		// ip spaces
		for _, space := range cc.IPAware {
			conn.Src.IPAware.Add(space)
			conn.Dst.IPAware.Add(space)
		}

		// ip connection
		for _, space := range cc.IPSpaces {
			conn.IPSpaces.Add(space)
		}

		// ip numbered
		for _, num := range cc.Numbered {
			conn.Src.Numbered.Add(num)
			conn.Dst.Numbered.Add(num)
		}
	}
	return nil
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
	cfg.ipSpaceDefinitionMap = map[string]*IPSpaceDefinition{}
	for i, ipspace := range cfg.IPSpaceDefinitions {
		cfg.ipSpaceDefinitionMap[ipspace.Name] = &cfg.IPSpaceDefinitions[i]
	}
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
		err = assignNodeNames(cfg, nm)
		if err != nil {
			return nil, err
		}
	}
	err = assignInterfaceNames(cfg, nm)
	if err != nil {
		return nil, err
	}

	// assign numbers, interface names and addresses
	err = nm.checkFlags(cfg)
	if err != nil {
		return nil, err
	}

	err = assignIPParameters(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = assignNumbers(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = formatNumbers(nm)
	if err != nil {
		return nil, err
	}

	// build config commands from config templates
	cfg, err = loadTemplates(cfg)
	if err != nil {
		return nil, err
	}

	err = generateConfig(cfg, nm)
	if err != nil {
		return nil, err
	}

	return nm, err
}

func buildSkeleton(cfg *Config, nd *NetworkDiagram) (*NetworkModel, error) {
	nm := NetworkModel{}
	allNodes := nd.AllNodes()
	allEdges := nd.AllLines()

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
		node.Labels = cfg.getValidNodeClasses(n.Classes)
		node.Numbers = map[string]string{}
		node.RelativeNumbers = map[string]string{}
		node.interfaceMap = map[string]*Interface{}

		if len(node.Labels.ClassLabels) == 0 {
			return nil, fmt.Errorf("set default nodeclass to leave nodes unlabeled")
		}
	}

	nm.Connections = make([]Connection, 0, len(allEdges))
	for _, e := range allEdges {
		srcNode, ok := nm.NodeByName(e.From().(*DiagramNode).Name)
		if !ok {
			return nil, fmt.Errorf("inconsistent DiagramEdge")
		}
		if _, ok := srcNode.interfaceMap[e.SrcName]; ok {
			// Existing named interface
			return nil, fmt.Errorf("duplicated interface names")
		}
		// New interface
		srcNode.Interfaces = append(srcNode.Interfaces, Interface{})
		srcIf := &srcNode.Interfaces[len(srcNode.Interfaces)-1]
		if e.SrcName != "" {
			// New named interface
			srcIf.Name = e.SrcName
			srcNode.interfaceMap[srcIf.Name] = srcIf
		}
		srcIf.Node = srcNode
		srcIf.Numbers = map[string]string{}
		srcIf.RelativeNumbers = map[string]string{}
		srcIf.Labels = cfg.getValidInterfaceClasses(e.SrcClasses)

		dstNode, ok := nm.NodeByName(e.To().(*DiagramNode).Name)
		if !ok {
			return nil, fmt.Errorf("inconsistent DiagramEdge")
		}
		if _, ok := dstNode.interfaceMap[e.DstName]; ok {
			// Existing named interface
			return nil, fmt.Errorf("duplicated interface names")
		}
		// New interface
		dstNode.Interfaces = append(dstNode.Interfaces, Interface{})
		dstIf := &dstNode.Interfaces[len(dstNode.Interfaces)-1]
		if e.DstName != "" {
			// New named interface
			dstIf.Name = e.DstName
			dstNode.interfaceMap[dstIf.Name] = dstIf
		}
		dstIf.Node = dstNode
		dstIf.Numbers = map[string]string{}
		dstIf.RelativeNumbers = map[string]string{}
		dstIf.Labels = cfg.getValidInterfaceClasses(e.DstClasses)

		srcIf.Opposite = dstIf // TODO opposite -> method function?
		dstIf.Opposite = srcIf

		conn := Connection{Src: srcIf, Dst: dstIf}
		conn.Labels = cfg.getValidConnectionClasses(e.Classes)
		if len(conn.Labels.PlaceLabels) > 0 {
			return nil, fmt.Errorf("connection cannot have placeLabels")
		}
		nm.Connections = append(nm.Connections, conn)
		srcIf.Connection = &nm.Connections[len(nm.Connections)-1]
		dstIf.Connection = &nm.Connections[len(nm.Connections)-1]

		if (len(srcIf.Labels.ClassLabels) == 0 || len(dstIf.Labels.ClassLabels) == 0) && len(conn.Labels.ClassLabels) == 0 {
			return nil, fmt.Errorf("set default interfaceclass or connectionclass to leave edges unlabeled")
		}
	}

	return &nm, nil
}

// assignNodeNames assign names for unnamed nodes with given name prefix automatically
func assignNodeNames(cfg *Config, nm *NetworkModel) error {
	nodePrefixes := map[string][]*Node{}
	for i, node := range nm.Nodes {
		checked := false
		for _, cls := range node.Labels.ClassLabels {
			nc, ok := cfg.NodeClassByName(cls)
			if !ok {
				return fmt.Errorf("invalid NodeClass name %+v", cls)
			}
			if nc.Prefix != "" {
				if checked {
					return fmt.Errorf("duplicated node name prefix (classes %+v)", node.Labels.ClassLabels)
				} else {
					checked = true
					nodePrefixes[nc.Prefix] = append(nodePrefixes[nc.Prefix], &nm.Nodes[i])
				}
			}
			if !checked {
				return fmt.Errorf("unnamed node without node name prefix (classes %+v)", node.Labels.ClassLabels)
			}
		}
	}

	for prefix, nodes := range nodePrefixes {
		for i, node := range nodes {
			node.Name = prefix + strconv.Itoa(i+1) // starts with 1
			nm.nodeMap[node.Name] = node
		}
	}

	return nil
}

// assignNodeNames assign names for unnamed interfaces with given name prefix automatically
func assignInterfaceNames(cfg *Config, nm *NetworkModel) error {
	ifacePrefixes := map[string]map[*Interface]string{}

	for _, conn := range nm.Connections {
		checked := false
		for _, cls := range conn.Labels.ClassLabels {
			cc, ok := cfg.ConnectionClassByName(cls)
			if !ok {
				return fmt.Errorf("invalid InterfaceClass name %+v", cls)
			}
			if cc.Prefix != "" {
				if checked {
					return fmt.Errorf("duplicated interface name prefix (connection classes %+v)", conn.Labels.ClassLabels)
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
			for _, cls := range iface.Labels.ClassLabels {
				ic, ok := cfg.InterfaceClassByName(cls)
				if !ok {
					return fmt.Errorf("invalid InterfaceClass name %+v", cls)
				}
				if ic.Prefix != "" {
					if checked {
						return fmt.Errorf("duplicated interface name prefix (classes %+v)", node.Labels.ClassLabels)
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
				return fmt.Errorf("still exists unnamed interfaces")
			}
		}
	}

	return nil
}

func assignIPParameters(cfg *Config, nm *NetworkModel) error {
	for _, ipspace := range cfg.IPSpaceDefinitions {
		err := assignIPLoopbacks(cfg, nm, ipspace)
		if err != nil {
			return err
		}
		err = assignIPAddresses(cfg, nm, ipspace)
		if err != nil {
			return err
		}
	}
	return nil
}

func assignNumbers(cfg *Config, nm *NetworkModel) error {
	nodesForNumbers := map[string][]*Node{}
	interfacesForNumbers := map[string][]*Interface{}
	for i, node := range nm.Nodes {
		nm.Nodes[i].addNumber(NumberReplacerName, node.Name)
		for num := range node.Numbered.Iter() {
			nodesForNumbers[num] = append(nodesForNumbers[num], &nm.Nodes[i])
		}
		for j, iface := range node.Interfaces {
			nm.Nodes[i].Interfaces[j].addNumber(NumberReplacerName, iface.Name)
			for num := range iface.Numbered.Iter() {
				interfacesForNumbers[num] = append(interfacesForNumbers[num], &nm.Nodes[i].Interfaces[j])
			}
		}
	}

	for num, nodes := range nodesForNumbers {
		cnt := len(nodes)
		switch num {
		case NumberAS:
			asnumbers, err := getASNumber(cnt)
			if err != nil {
				return err
			}
			for nid, node := range nodes {
				val := strconv.Itoa(asnumbers[nid])
				node.addNumber(num, val)
			}
		default:
			return fmt.Errorf("not implemented number (%v)", num)
		}
	}

	for num, ifaces := range interfacesForNumbers {
		cnt := len(ifaces)
		switch num {
		default:
			// TODO assign customized numbers
			fmt.Printf("cnt %v", cnt)
			return fmt.Errorf("not implemented number (%v)", num)
		}
	}

	// set values in ValueLabels
	for i, node := range nm.Nodes {
		for k, v := range node.Labels.ValueLabels {
			nm.Nodes[i].addNumber(k, v) // overwrite
		}
		for j, iface := range node.Interfaces {
			for k, v := range iface.Labels.ValueLabels {
				nm.Nodes[i].Interfaces[j].addNumber(k, v)
			}
		}
	}
	for _, conn := range nm.Connections {
		for k, v := range conn.Labels.ValueLabels {
			conn.Src.addNumber(k, v)
			conn.Dst.addNumber(k, v)
		}
	}

	return nil
}

func formatNumbers(nm *NetworkModel) error {
	// Search global namespace with placelabels
	globalNumbers := map[string]string{}
	numbersForAlias := map[string]map[string]string{}
	for _, node := range nm.Nodes {
		if len(node.Labels.PlaceLabels) > 0 {
			for _, plabel := range node.Labels.PlaceLabels {
				if _, exists := numbersForAlias[plabel]; exists {
					return fmt.Errorf("duplicated PlaceLabels %+v", plabel)
				}
				numbersForAlias[plabel] = map[string]string{}

				for k, v := range node.Numbers {
					num := plabel + NumberSeparator + k
					globalNumbers[num] = v
					numbersForAlias[plabel][k] = v
				}
			}
		}

		for _, iface := range node.Interfaces {
			if len(iface.Labels.PlaceLabels) > 0 {
				for _, plabel := range iface.Labels.PlaceLabels {
					if _, exists := numbersForAlias[plabel]; exists {
						return fmt.Errorf("duplicated PlaceLabels %+v", plabel)
					}
					numbersForAlias[plabel] = map[string]string{}

					for k, v := range iface.Numbers {
						num := plabel + NumberSeparator + k
						globalNumbers[num] = v
						numbersForAlias[plabel][k] = v
					}
				}
			}
		}
	}

	for i, node := range nm.Nodes {

		// node self
		for num, val := range node.Numbers {
			nm.Nodes[i].RelativeNumbers[num] = val
		}

		// global namespace of PlaceLabels
		for k, v := range globalNumbers {
			nm.Nodes[i].RelativeNumbers[k] = v
		}

		// alias of MetaValueLabels
		if len(node.Labels.MetaValueLabels) > 0 {
			for mvlabel, target := range node.Labels.MetaValueLabels {
				if _, ok := numbersForAlias[target]; !ok {
					return fmt.Errorf("invalid MetaValueLabel for PlaceLabel %+v", target)
				}
				for k, v := range numbersForAlias[target] {
					num := mvlabel + NumberSeparator + k
					nm.Nodes[i].RelativeNumbers[num] = v
				}
			}
		}

		for j, iface := range node.Interfaces {

			// interface self
			for num, val := range iface.Numbers {
				nm.Nodes[i].Interfaces[j].RelativeNumbers[num] = val
			}

			// node of interface
			for nodenum, val := range node.Numbers {
				num := NumberPrefixNode + nodenum
				nm.Nodes[i].Interfaces[j].RelativeNumbers[num] = val
			}

			// opposite interface
			oppIf := iface.Opposite
			for oppnum, val := range oppIf.Numbers {
				num := NumberPrefixOppositeInterface + oppnum
				nm.Nodes[i].Interfaces[j].RelativeNumbers[num] = val
			}

			// node of opposite interface
			oppNode := oppIf.Node
			for oppnnum, val := range oppNode.Numbers {
				num := NumberPrefixOppositeNode + oppnnum
				nm.Nodes[i].Interfaces[j].RelativeNumbers[num] = val
			}

			// global namespace of PlaceLabels
			for k, v := range globalNumbers {
				nm.Nodes[i].Interfaces[j].RelativeNumbers[k] = v
			}

			// alias of MetaValueLabels
			if len(iface.Labels.MetaValueLabels) > 0 {
				for mvlabel, target := range iface.Labels.MetaValueLabels {
					if _, ok := numbersForAlias[target]; !ok {
						return fmt.Errorf("invalid MetaValueLabel for PlaceLabel %+v", target)
					}
					for k, v := range numbersForAlias[target] {
						num := mvlabel + NumberSeparator + k
						nm.Nodes[i].Interfaces[j].RelativeNumbers[num] = v
					}
				}
			}
		}
	}

	return nil
}

func generateConfig(cfg *Config, nm *NetworkModel) error {
	for i, node := range nm.Nodes {
		configBlocks := []string{}
		configTarget := []string{}
		configPriority := []int{}
		for _, cls := range node.Labels.ClassLabels {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return fmt.Errorf("undefined NodeClass name %v", cls)
			}
			for _, nct := range nc.ConfigTemplates {
				block, err := getConfig(nct.parsedTemplate, node.RelativeNumbers)
				if err != nil {
					return err
				}
				configBlocks = append(configBlocks, block)
				configTarget = append(configTarget, nct.Target)
				configPriority = append(configPriority, nct.Priority)
			}
		}

		for _, iface := range node.Interfaces {
			for _, cls := range iface.Labels.ClassLabels {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined InterfaceClass name %v", cls)
				}
				for _, ict := range ic.ConfigTemplates {
					if !(ict.NodeClass == "" || node.HasClass(ict.NodeClass)) {
						continue
					}
					block, err := getConfig(ict.parsedTemplate, iface.RelativeNumbers)
					if err != nil {
						return err
					}
					configBlocks = append(configBlocks, block)
					configTarget = append(configTarget, ict.Target)
					configPriority = append(configPriority, ict.Priority)
				}
			}

			for _, cls := range iface.Connection.Labels.ClassLabels {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined ConnectionClass name %v", cls)
				}
				for _, cct := range cc.ConfigTemplates {
					if !(cct.NodeClass == "" || node.HasClass(cct.NodeClass)) {
						continue
					}
					block, err := getConfig(cct.parsedTemplate, iface.RelativeNumbers)
					if err != nil {
						return err
					}
					configBlocks = append(configBlocks, block)
					configTarget = append(configTarget, cct.Target)
					configPriority = append(configPriority, cct.Priority)
				}
			}
		}
		commands, err := sortConfig(configBlocks, configTarget, configPriority)
		if err != nil {
			return err
		}
		nm.Nodes[i].Commands = commands
	}
	return nil
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
