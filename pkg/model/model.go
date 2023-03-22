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

const DefaultNodePrefix string = "node"
const DefaultInterfacePrefix string = "net"
const ManagementInterfaceName string = "mgmt"

const ClassAll string = "all"         // all nodes/interfaces/connections
const ClassDefault string = "default" // all empty nodes/interfaces/connections
const PlaceLabelPrefix string = "@"
const ValueLabelSeparator string = "="

const NumberSeparator string = "_"
const NumberPrefixNode string = "node_"
const NumberPrefixGroup string = "group_"
const NumberPrefixOppositeHeader string = "opp_"
const NumberPrefixOppositeInterface string = "opp_"

//const NumberPrefixOppositeNode string = "oppnode_"
//const NumberPrefixOppositeGroup string = "oppgroup_"

const NumberReplacerName string = "name"

// IP number replacer: [IPSpace]_[IPReplacerXX]
const IPLoopbackReplacerFooter string = "loopback"
const IPAddressReplacerFooter string = "addr"
const IPNetworkReplacerFooter string = "net"
const IPPrefixLengthReplacerFooter string = "plen"

const NumberAS string = "as"

const DummyIPSpace string = "dummy"

const TargetLocal string = "local"
const TargetFRR string = "frr"

const OutputTinet string = "tinet"
const OutputClab string = "clab"
const OutputAsis string = "command"

type Config struct {
	Name               string              `yaml:"name" mapstructure:"name"`
	GlobalSettings     GlobalSettings      `yaml:"global" mapstructure:"global"`
	IPSpaceDefinitions []IPSpaceDefinition `yaml:"ipspace" mapstructure:"ipspace"`
	NodeClasses        []NodeClass         `yaml:"nodeclass,flow" mapstructure:"nodes,flow"`
	InterfaceClasses   []InterfaceClass    `yaml:"interfaceclass,flow" mapstructure:"interfaces,flow"`
	ConnectionClasses  []ConnectionClass   `yaml:"connectionclass,flow" mapstructure:"connections,flow"`
	GroupClasses       []GroupClass        `yaml:"groupclass,flow" mapstructure:"group,flow"`

	ipSpaceDefinitionMap map[string]*IPSpaceDefinition
	nodeClassMap         map[string]*NodeClass
	interfaceClassMap    map[string]*InterfaceClass
	connectionClassMap   map[string]*ConnectionClass
	groupClassMap        map[string]*GroupClass
	mgmtIPSpace          *IPSpaceDefinition
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

func (cfg *Config) GroupClassByName(name string) (*GroupClass, bool) {
	gc, ok := cfg.groupClassMap[name]
	return gc, ok
}

func (cfg *Config) GetManagementIPSpace() *IPSpaceDefinition {
	return cfg.mgmtIPSpace
}

func (cfg *Config) IPSpaceNames() []string {
	names := make([]string, 0, len(cfg.IPSpaceDefinitions))
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

func (cfg *Config) getValidGroupClasses(given []string) parsedLabels {
	_, hasAllGroupClass := cfg.groupClassMap[ClassAll]
	_, hasDefaultGroupClass := cfg.groupClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllGroupClass, hasDefaultGroupClass)
}

type GlobalSettings struct {
	PathSpecification string `yaml:"path" mapstructure:"path"`
	NodeAutoRename    bool   `yaml:"nodeautoname" mapstructure:"nodeautoname"`
	// If mgmt_ipspace is given, specified ipspace is used only for management network (connection with host machine)
	ManagementIPSpace string `yaml:"mgmt_ipspace" mapstructure:"mgmt_ipspace"`
	// If mgmt_name is given, used for management interface name as is
	ManagementInterfaceName string `yaml:"mgmt_name" mapstructure:"mgmt_name"`
	// ASNumberMin and ASNumberMAX are optional, considered in AssignASNumbers if specified
	ASNumberMin int `yaml:"asnumber_min" mapstructure:"asnumber_min"`
	ASNumberMax int `yaml:"asnumber_max" mapstructure:"asnumber_max"`

	ClabAttr map[string]interface{} `yaml:"clab" mapstructure:"clab"` // containerlab attributes
}

type IPSpaceDefinition struct {
	Name                string `yaml:"name" mapstructure:"name"`
	AddrRange           string `yaml:"range" mapstructure:"range"`
	LoopbackRange       string `yaml:"loopback_range" mapstructure:"loopback_range"`
	DefaultPrefixLength int    `yaml:"prefix" mapstructure:"prefix"`
	// gateway is used only for management network or external network
	// the address is avoided in automated IPaddress assignment
	ExternalGateway string `yaml:"gateway" mapstructure:"gateway"`
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
	// A node can have only one "primary" node class.
	// Unprimary node classes only have "name", "numbered" and "config". Other attributes are ignored.
	Name            string               `yaml:"name" mapstructure:"name"`
	Primary         bool                 `yaml:"primary" mapstructure:"primary"`
	IPAware         []string             `yaml:"ipaware" mapstructure:"ipaware"` // aware ip spaces for loopback
	Numbered        []string             `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	ConfigTemplates []NodeConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix        string                 `yaml:"prefix" mapstructure:"prefix"`                 // prefix of auto-naming
	MgmtInterface string                 `yaml:"mgmt_interface" mapstructure:"mgmt_interface"` // InterfaceClass name for mgmt
	TinetAttr     map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`                   // tinet attributes
	ClabAttr      map[string]interface{} `yaml:"clab" mapstructure:"clab"`                     // containerlab attributes
}

type NodeConfigTemplate struct {
	Output   []string `yaml:"output,flow" mapstructure:"output,flow"` // add config only for included output
	Target   string   `yaml:"target" mapstructure:"target"`           // config type, such as "shell", "frr", etc.
	Priority int      `yaml:"priority" mapstructure:"priority"`
	Template []string `yaml:"template" mapstructure:"template"`
	Filepath string   `yaml:"filepath" mapstructure:"filepath"`

	parsedTemplate *template.Template
	outputSet      mapset.Set[string]
}

type InterfaceClass struct {
	// An interface can have only one of "primary" interface class or "primary" connection class.
	Name            string                    `yaml:"name" mapstructure:"name"`
	Primary         bool                      `yaml:"primary" mapstructure:"primary"`
	Numbered        []string                  `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	IPAware         []string                  `yaml:"ipaware" mapstructure:"ipaware"` // aware ip spaces
	ConfigTemplates []InterfaceConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix    string                 `yaml:"prefix" mapstructure:"prefix"` // prefix of auto-naming
	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`   // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`     // containerlab attributes
}

type InterfaceConfigTemplate struct {
	NodeClass string   `yaml:"node" mapstructure:"node"`               // add config only for interfaces of nodes belongs to this nodeclass
	Output    []string `yaml:"output,flow" mapstructure:"output,flow"` // add config only for included output
	Target    string   `yaml:"target" mapstructure:"target"`           // config target, such as "shell", "frr", etc.
	Priority  int      `yaml:"priority" mapstructure:"priority"`
	Template  []string `yaml:"template" mapstructure:"template"`
	Filepath  string   `yaml:"filepath" mapstructure:"filepath"`

	parsedTemplate *template.Template
	outputSet      mapset.Set[string]
}

type ConnectionClass struct {
	Name            string                     `yaml:"name" mapstructure:"name"`
	Primary         bool                       `yaml:"primary" mapstructure:"primary"`
	Numbered        []string                   `yaml:"numbered,flow" mapstructure:"numbered,flow"` // Numbers to be assigned automatically
	IPAware         []string                   `yaml:"ipaware,flow" mapstructure:"ipaware,flow"`   // aware ip spaces for end interfaces
	IPSpaces        []string                   `yaml:"ipspaces,flow" mapstructure:"ipspaces,flow"` // Connection is limited to specified spaces
	ConfigTemplates []ConnectionConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix    string                 `yaml:"prefix" mapstructure:"prefix"` // prefix of interface auto-naming
	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`   // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`     // containerlab attributes
}

type ConnectionConfigTemplate struct {
	NodeClass string   `yaml:"node" mapstructure:"node"`               // add config only for interfaces of nodes belongs to this nodeclass
	Output    []string `yaml:"output,flow" mapstructure:"output,flow"` // add config only for included output
	Target    string   `yaml:"target" mapstructure:"target"`           // config target, such as "shell", "frr", etc.
	Priority  int      `yaml:"priority" mapstructure:"priority"`
	Template  []string `yaml:"template" mapstructure:"template"`
	Filepath  string   `yaml:"filepath" mapstructure:"filepath"`

	parsedTemplate *template.Template
	outputSet      mapset.Set[string]
}

type GroupClass struct {
	Name     string   `yaml:"name" mapstructure:"name"`
	Numbered []string `yaml:"numbered,flow" mapstructure:"numbered,flow"`
}

type parsedLabels struct {
	ClassLabels     []string
	PlaceLabels     []string
	ValueLabels     map[string]string
	MetaValueLabels map[string]string
}

type NetworkModel struct {
	Nodes       []*Node
	Connections []*Connection
	Groups      []*Group

	nodeMap  map[string]*Node
	groupMap map[string]*Group
}

func (nm *NetworkModel) newNode(name string) *Node {
	node := &Node{
		Name:            name,
		Numbers:         map[string]string{},
		RelativeNumbers: map[string]string{},
		ipAware:         mapset.NewSet[string](),
		numbered:        mapset.NewSet[string](),
		interfaceMap:    map[string]*Interface{},
	}
	nm.Nodes = append(nm.Nodes, node)
	nm.nodeMap[name] = node

	return node
}

func (nm *NetworkModel) newConnection(src *Interface, dst *Interface) *Connection {
	conn := &Connection{
		Src:      src,
		Dst:      dst,
		IPSpaces: mapset.NewSet[string](),
	}
	nm.Connections = append(nm.Connections, conn)
	src.Connection = conn
	dst.Connection = conn
	return conn
}

func (nm *NetworkModel) newGroup(name string) *Group {
	group := &Group{
		Name:     name,
		Nodes:    []*Node{},
		Numbers:  map[string]string{},
		numbered: mapset.NewSet[string](),
	}
	nm.Groups = append(nm.Groups, group)
	nm.groupMap[name] = group

	return group
}

func (nm *NetworkModel) NodeByName(name string) (*Node, bool) {
	node, ok := nm.nodeMap[name]
	return node, ok
}

func (nm *NetworkModel) GroupByName(name string) (*Group, bool) {
	group, ok := nm.groupMap[name]
	return group, ok
}

type Node struct {
	Name            string
	Interfaces      []*Interface
	Groups          []*Group
	Labels          parsedLabels
	Numbers         map[string]string
	RelativeNumbers map[string]string
	Commands        []string
	TinetAttr       *map[string]interface{}
	ClabAttr        *map[string]interface{}

	ipAware            mapset.Set[string] // Aware IPspaces for loopbacks
	numbered           mapset.Set[string]
	namePrefix         string
	mgmtInterface      *Interface
	mgmtInterfaceClass *InterfaceClass
	interfaceMap       map[string]*Interface
}

func (n *Node) newInterface(name string) *Interface {
	iface := &Interface{
		Name:            name,
		Node:            n,
		Numbers:         map[string]string{},
		RelativeNumbers: map[string]string{},
		ipAware:         mapset.NewSet[string](),
		numbered:        mapset.NewSet[string](),
	}
	n.Interfaces = append(n.Interfaces, iface)
	if name != "" {
		n.interfaceMap[iface.Name] = iface
	}
	return iface
}

func (n *Node) HasClass(name string) bool {
	for _, cls := range n.Labels.ClassLabels {
		if cls == name {
			return true
		}
	}
	return false
}

func (n *Node) GetManagementInterface() *Interface {
	return n.mgmtInterface
}

func (n *Node) addNumber(key, val string) {
	n.Numbers[key] = val
}

func (n *Node) GivenIPLoopback(ipspace *IPSpaceDefinition) (string, bool) {
	for k, v := range n.Labels.ValueLabels {
		if k == ipspace.IPLoopbackReplacer() {
			return v, true
		}
	}
	return "", false
}

type Interface struct {
	Name            string
	Node            *Node
	Labels          parsedLabels
	Numbers         map[string]string
	RelativeNumbers map[string]string
	Connection      *Connection
	Opposite        *Interface
	TinetAttr       *map[string]interface{}
	// ClabAttr        *map[string]interface{}

	ipAware    mapset.Set[string] // Aware IPSpaces for IP address assignment
	numbered   mapset.Set[string]
	namePrefix string
}

func (iface *Interface) GivenIPAddress(ipspace *IPSpaceDefinition) (string, bool) {
	for k, v := range iface.Labels.ValueLabels {
		if k == ipspace.IPAddressReplacer() {
			return v, true
		}
	}
	return "", false
}

func (iface *Interface) addNumber(key, val string) {
	iface.Numbers[key] = val
}

type Connection struct {
	Src      *Interface
	Dst      *Interface
	Labels   parsedLabels
	IPSpaces mapset.Set[string]
}

func (conn *Connection) GivenIPNetwork(ipspace *IPSpaceDefinition) (string, bool) {
	for k, v := range conn.Labels.ValueLabels {
		if k == ipspace.IPNetworkReplacer() {
			return v, true
		}
	}
	return "", false
}

type Group struct {
	Name    string
	Nodes   []*Node
	Labels  parsedLabels
	Numbers map[string]string

	numbered mapset.Set[string]
}

func (g *Group) addNumber(key, val string) {
	g.Numbers[key] = val
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
	cfg.groupClassMap = map[string]*GroupClass{}
	for i, group := range cfg.GroupClasses {
		cfg.groupClassMap[group.Name] = &cfg.GroupClasses[i]
	}
	return &cfg, err
}

func loadTemplates(cfg *Config) (*Config, error) {
	var outputs []string
	allOutput := []string{OutputTinet, OutputClab, OutputAsis}

	for i, nc := range cfg.NodeClasses {
		for j, nct := range nc.ConfigTemplates {
			// init output list
			cfg.NodeClasses[i].ConfigTemplates[j].outputSet = mapset.NewSet[string]()
			if outputs = nct.Output; len(outputs) == 0 {
				outputs = allOutput
			}
			for _, output := range outputs {
				cfg.NodeClasses[i].ConfigTemplates[j].outputSet.Add(output)
			}
			// init parsed template object
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
			// init output list
			cfg.InterfaceClasses[i].ConfigTemplates[j].outputSet = mapset.NewSet[string]()
			if outputs = ict.Output; len(outputs) == 0 {
				outputs = allOutput
			}
			for _, output := range outputs {
				cfg.InterfaceClasses[i].ConfigTemplates[j].outputSet.Add(output)
			}
			// init parsed template object
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
			// init output list
			cfg.ConnectionClasses[i].ConfigTemplates[j].outputSet = mapset.NewSet[string]()
			if outputs = cct.Output; len(outputs) == 0 {
				outputs = allOutput
			}
			for _, output := range outputs {
				cfg.ConnectionClasses[i].ConfigTemplates[j].outputSet.Add(output)
			}
			// init parsed template object
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

func BuildNetworkModel(cfg *Config, d *Diagram, output string) (nm *NetworkModel, err error) {

	// build topology
	nm, err = buildSkeleton(cfg, d)
	if err != nil {
		return nil, err
	}

	err = checkClasses(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = addSpecialInterfaces(cfg, nm)
	if err != nil {
		return nil, err
	}

	// assign names for unnamed objects in topology
	if cfg.GlobalSettings.NodeAutoRename {
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

	err = generateConfig(cfg, nm, output)
	if err != nil {
		return nil, err
	}

	return nm, err
}

func buildSkeleton(cfg *Config, d *Diagram) (*NetworkModel, error) {
	nm := &NetworkModel{}

	ifaceCounter := map[string]int{}
	for _, e := range d.graph.Edges.Edges {
		ifaceCounter[e.Src]++
		ifaceCounter[e.Dst]++
	}

	nm.Groups = make([]*Group, 0, len(d.graph.SubGraphs.SubGraphs))
	nm.groupMap = map[string]*Group{}
	for _, s := range d.graph.SubGraphs.SubGraphs {
		group := nm.newGroup(s.Name)
		group.Labels = cfg.getValidGroupClasses(getSubGraphLabels(s))
	}

	nm.Nodes = make([]*Node, 0, len(d.graph.Nodes.Nodes))
	nm.nodeMap = map[string]*Node{}
	for _, n := range d.SortedNodes() {
		node := nm.newNode(n.Name)
		// Note: node.Name can be overwritten later if nodeautoname = true
		// but the name must be DOTID in this function to keep consistency with other graph objects
		node.Labels = cfg.getValidNodeClasses(getNodeLabels(n))
		if groups, ok := d.nodeGroups[n.Name]; ok {
			for _, name := range groups {
				group, ok := nm.GroupByName(name)
				if !ok {
					return nil, fmt.Errorf("invalid group name %s", name)
				}
				node.Groups = append(node.Groups, group)
			}
		}
	}

	nm.Connections = make([]*Connection, 0, len(d.graph.Edges.Edges))
	for _, e := range d.SortedLinks() {
		labels, srcLabels, dstLabels := getEdgeLabels(e)

		srcNode, ok := nm.NodeByName(e.Src)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := srcNode.interfaceMap[e.SrcPort]; ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.SrcPort)
		}
		// new interface
		// interface name can be blank (automatically named later)
		srcIf := srcNode.newInterface(strings.TrimLeft(e.SrcPort, ":"))
		srcIf.Labels = cfg.getValidInterfaceClasses(srcLabels)

		dstNode, ok := nm.NodeByName(e.Dst)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := dstNode.interfaceMap[e.DstPort]; ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.DstPort)
		}
		dstIf := dstNode.newInterface(strings.TrimLeft(e.DstPort, ":"))
		dstIf.Labels = cfg.getValidInterfaceClasses(dstLabels)

		srcIf.Opposite = dstIf
		dstIf.Opposite = srcIf

		conn := nm.newConnection(srcIf, dstIf)
		conn.Labels = cfg.getValidConnectionClasses(labels)
		if len(conn.Labels.PlaceLabels) > 0 {
			return nil, fmt.Errorf("connection cannot have placeLabels")
		}
		if (len(srcIf.Labels.ClassLabels) == 0 || len(dstIf.Labels.ClassLabels) == 0) && len(conn.Labels.ClassLabels) == 0 {
			return nil, fmt.Errorf("set default interfaceclass or connectionclass to leave links unlabeled")
		}
	}

	return nm, nil
}

func checkClasses(cfg *Config, nm *NetworkModel) error {
	/*
		- check primary class consistency
		- store primary class attributes on objects
		- check flags (IPAware, Numbered and IPSpaces)
	*/

	defaultIPAware := cfg.DefaultIPAware()
	defaultIPConnect := cfg.DefaultIPConnect()

	var primaryNC string
	primaryICMap := map[*Interface]string{}

	for _, node := range nm.Nodes {
		primaryNC = ""

		// set defaults for nodes without primary class
		node.namePrefix = DefaultNodePrefix

		// add defaults of node loopback ipaware
		for _, space := range defaultIPAware {
			node.ipAware.Add(space)
		}

		// check nodeclass flags
		for _, cls := range node.Labels.ClassLabels {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid NodeClass name %s", cls)
			}

			// check IP aware
			for _, space := range nc.IPAware {
				node.ipAware.Add(space)
			}

			// check numbered
			for _, num := range nc.Numbered {
				node.numbered.Add(num)
			}

			// check primary node class consistency
			if nc.Primary {
				if primaryNC == "" {
					primaryNC = nc.Name
				} else {
					return fmt.Errorf("multiple primary node classes on one node (%s, %s)", primaryNC, nc.Name)
				}
				// add parameters of primary node class
				if node.namePrefix != "" {
					node.namePrefix = nc.Prefix
				}
				if nc.MgmtInterface != "" {
					if mgmtnc, ok := cfg.InterfaceClassByName(nc.MgmtInterface); ok {
						node.mgmtInterfaceClass = mgmtnc
					} else {
						return fmt.Errorf("invalid mgmt interface class name %s", nc.MgmtInterface)
					}
				}
				node.TinetAttr = &nc.TinetAttr
				node.ClabAttr = &nc.ClabAttr
			} else {
				if nc.Prefix != "" {
					return fmt.Errorf("prefix can be specified only in primary class")
				}
				if nc.MgmtInterface != "" {
					return fmt.Errorf("mgmt inteface class can be specified only in primary class")
				}
				if len(nc.TinetAttr) > 0 || len(nc.ClabAttr) > 0 {
					return fmt.Errorf("output-specific attributes can be specified only in primary class")
				}
			}
		}
		if primaryNC == "" {
			fmt.Fprintf(os.Stderr, "warning: no primary node class on node %s", node.Name)
		}

		for _, iface := range node.Interfaces {
			// set defaults for interfaces without primary class
			iface.namePrefix = DefaultInterfacePrefix

			// add defaults of interface ipaware (added by Interface.checkFlags and Connection.checkFlags)
			for _, space := range defaultIPAware {
				iface.ipAware.Add(space)
			}
			// check interfaceclass flags
			for _, cls := range iface.Labels.ClassLabels {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return fmt.Errorf("invalid InterfaceClass name %s", cls)
				}

				// ip spaces
				for _, space := range ic.IPAware {
					iface.ipAware.Add(space)
				}

				// check numbered
				for _, num := range ic.Numbered {
					iface.numbered.Add(num)
				}

				// check primary interface class consistency
				if ic.Primary {
					if picname, exists := primaryICMap[iface]; !exists {
						primaryICMap[iface] = ic.Name
					} else {
						return fmt.Errorf("multiple primary interface classes on one node (%s, %s)", picname, ic.Name)
					}
					if iface.namePrefix != "" {
						iface.namePrefix = ic.Prefix
					}
					iface.TinetAttr = &ic.TinetAttr
					// iface.ClabAttr = &ic.ClabAttr
				} else {
					if ic.Prefix != "" {
						return fmt.Errorf("prefix can be specified only in primary class")
					}
					if len(ic.TinetAttr) > 0 || len(ic.ClabAttr) > 0 {
						return fmt.Errorf("output-specific attributes can be specified only in primary class")
					}
				}
			}
		}
	}

	for _, conn := range nm.Connections {
		for _, space := range defaultIPConnect {
			conn.IPSpaces.Add(space)
		}
		// check connectionclass flags to connections and their interfaces
		for _, cls := range conn.Labels.ClassLabels {
			cc, ok := cfg.connectionClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid ConnectionClass name %s", cls)
			}

			// ip spaces
			for _, space := range cc.IPAware {
				conn.Src.ipAware.Add(space)
				conn.Dst.ipAware.Add(space)
			}

			// ip connection
			for _, space := range cc.IPSpaces {
				conn.IPSpaces.Add(space)
			}

			// check numbered
			for _, num := range cc.Numbered {
				conn.Src.numbered.Add(num)
				conn.Dst.numbered.Add(num)
			}

			// check primary interface class consistency
			if cc.Primary {
				if name, exists := primaryICMap[conn.Src]; !exists {
					primaryICMap[conn.Src] = cc.Name
				} else {
					return fmt.Errorf("multiple primary interface/connection classes on one node (%s, %s)", name, cc.Name)
				}
				conn.Src.TinetAttr = &cc.TinetAttr
				// conn.Src.ClabAttr = &cc.ClabAttr
				if name, exists := primaryICMap[conn.Dst]; !exists {
					primaryICMap[conn.Dst] = cc.Name
				} else {
					return fmt.Errorf("multiple primary interface/connection classes on one node (%s, %s)", name, cc.Name)
				}
				conn.Dst.TinetAttr = &cc.TinetAttr
				// conn.Dst.ClabAttr = &cc.ClabAttr
				if cc.Prefix != "" {
					conn.Src.namePrefix = cc.Prefix
					conn.Dst.namePrefix = cc.Prefix
				}
			} else {
				if cc.Prefix != "" {
					return fmt.Errorf("prefix can be specified only in primary class")
				}
				if len(cc.TinetAttr) > 0 || len(cc.ClabAttr) > 0 {
					return fmt.Errorf("output-specific attributes can be specified only in primary class")
				}
			}
		}
	}

	for _, group := range nm.Groups {
		// check groupclass flags to groups
		for _, cls := range group.Labels.ClassLabels {
			gc, ok := cfg.groupClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid GroupClass name %s", cls)
			}

			// check numbered
			for _, num := range gc.Numbered {
				group.numbered.Add(num)
			}
		}
	}

	return nil
}

func addSpecialInterfaces(cfg *Config, nm *NetworkModel) error {
	// setup management ipspace and interfaces only if management ipspace is specified
	space := cfg.GlobalSettings.ManagementIPSpace
	if space == "" {
		return nil
	}

	ipspace, ok := cfg.IPSpaceDefinitionByName(space)
	if !ok {
		return fmt.Errorf("mgmt IPSpace %v not defined", space)
	}
	cfg.mgmtIPSpace = ipspace

	// set mgmt interfaces on nodes
	name := cfg.GlobalSettings.ManagementInterfaceName
	if name == "" {
		name = ManagementInterfaceName
	}
	for _, node := range nm.Nodes {
		if ic := node.mgmtInterfaceClass; ic != nil {

			// check that mgmtInterfaceClass is not used in topology
			for _, iface := range node.Interfaces {
				for _, cls := range iface.Labels.ClassLabels {
					if cls == ic.Name {
						return fmt.Errorf("mgmt InterfaceClass should not be specified in topology graph (automatically added)")
					}
				}
			}

			// add management interface
			iface := node.newInterface(name)
			iface.Labels = parsedLabels{
				ClassLabels: []string{ic.Name},
			}
			iface.ipAware.Add(cfg.mgmtIPSpace.Name)
			node.mgmtInterface = iface

		}
	}

	return nil
}

// assignNodeNames assign names for unnamed nodes with given name prefix automatically
func assignNodeNames(cfg *Config, nm *NetworkModel) error {
	prefixMap := map[string][]*Node{}
	for _, node := range nm.Nodes {
		prefixMap[node.namePrefix] = append(prefixMap[node.namePrefix], node)
	}

	for prefix, nodes := range prefixMap {
		for i, node := range nodes {
			node.Name = prefix + strconv.Itoa(i+1) // starts with 1
			nm.nodeMap[node.Name] = node
		}
	}

	return nil
}

// assignNodeNames assign names for unnamed interfaces with given name prefix automatically
func assignInterfaceNames(cfg *Config, nm *NetworkModel) error {
	for _, node := range nm.Nodes {
		existingNames := map[string]struct{}{}
		prefixMap := map[string][]*Interface{} // Interfaces to be named automatically
		for _, iface := range node.Interfaces {
			if iface.Name == "" {
				prefixMap[iface.namePrefix] = append(prefixMap[iface.namePrefix], iface)
			} else {
				existingNames[iface.Name] = struct{}{}
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
					i++ // starts with 0, increment by loop
				}
				iface.Name = name
				iface.Node.interfaceMap[iface.Name] = iface
				existingNames[iface.Name] = struct{}{}
				i++
			}
		}
	}

	// confirm all interfaces are named
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			if iface.Name == "" {
				return fmt.Errorf("there still exists unnamed interfaces after assignInterfaceNames")
			}
		}
	}

	return nil
}

func assignIPParameters(cfg *Config, nm *NetworkModel) error {
	for i := range cfg.IPSpaceDefinitions {
		ipspace := &cfg.IPSpaceDefinitions[i]
		if ipspace.LoopbackRange != "" {
			err := assignIPLoopbacks(cfg, nm, ipspace)
			if err != nil {
				return err
			}
		}

		if ipspace.Name == cfg.GlobalSettings.ManagementIPSpace {
			err := assignManagementIPAddresses(cfg, nm, ipspace)
			if err != nil {
				return err
			}
		} else {
			err := assignIPAddresses(cfg, nm, ipspace)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func assignNumbers(cfg *Config, nm *NetworkModel) error {

	// set values in ValueLabels
	for _, node := range nm.Nodes {
		for k, v := range node.Labels.ValueLabels {
			// check existance (ip numbers may already added)
			if _, exists := node.Numbers[k]; !exists {
				node.addNumber(k, v)
			}
		}
		for _, iface := range node.Interfaces {
			for k, v := range iface.Labels.ValueLabels {
				// check existance (ip numbers may already added)
				if _, exists := iface.Numbers[k]; !exists {
					iface.addNumber(k, v)
				}
			}
		}
	}
	for _, conn := range nm.Connections {
		for k, v := range conn.Labels.ValueLabels {
			if _, exists := conn.Src.Numbers[k]; !exists {
				conn.Src.addNumber(k, v)
			}
			if _, exists := conn.Dst.Numbers[k]; !exists {
				conn.Dst.addNumber(k, v)
			}
		}
	}
	for _, group := range nm.Groups {
		for k, v := range group.Labels.ValueLabels {
			if _, exists := group.Numbers[k]; !exists {
				group.addNumber(k, v)
			}
		}
	}

	nodesForNumbers := map[string][]*Node{}
	interfacesForNumbers := map[string][]*Interface{}
	groupsForNumbers := map[string][]*Group{}

	// add object names as numbers, and list up numbered objects
	for _, node := range nm.Nodes {
		node.addNumber(NumberReplacerName, node.Name)
		for num := range node.numbered.Iter() {
			nodesForNumbers[num] = append(nodesForNumbers[num], node)
		}
		for _, iface := range node.Interfaces {
			iface.addNumber(NumberReplacerName, iface.Name)
			for num := range iface.numbered.Iter() {
				interfacesForNumbers[num] = append(interfacesForNumbers[num], iface)
			}
		}
	}
	for _, group := range nm.Groups {
		group.addNumber(NumberReplacerName, group.Name)
		for num := range group.numbered.Iter() {
			groupsForNumbers[num] = append(groupsForNumbers[num], group)
		}
	}

	// add node numbers
	for num, nodes := range nodesForNumbers {
		cnt := len(nodes)
		switch num {
		case NumberAS:
			asnumbers, err := getASNumber(cfg, cnt)
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

	// add interface numbers
	for num, ifaces := range interfacesForNumbers {
		cnt := len(ifaces)
		switch num {
		default:
			// TODO assign customized numbers
			if false {
				fmt.Printf("cnt %v", cnt)
			}
			return fmt.Errorf("not implemented number (%v)", num)
		}
	}

	// add group numbers
	for num, groups := range groupsForNumbers {
		cnt := len(groups)
		switch num {
		case NumberAS:
			asnumbers, err := getASNumber(cfg, cnt)
			if err != nil {
				return err
			}
			for nid, group := range groups {
				if _, exists := group.Numbers[num]; !exists {
					val := strconv.Itoa(asnumbers[nid])
					group.addNumber(num, val)
				}
			}
		default:
			return fmt.Errorf("not implemented number (%v)", num)
		}
	}

	return nil
}

func formatNumbers(nm *NetworkModel) error {
	// Search placelabels for global namespace
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
	for _, group := range nm.Groups {
		if len(group.Labels.PlaceLabels) > 0 {
			for _, plabel := range group.Labels.PlaceLabels {
				if _, exists := numbersForAlias[plabel]; exists {
					return fmt.Errorf("duplicated PlaceLabels %+v", plabel)
				}
				numbersForAlias[plabel] = map[string]string{}

				for k, v := range group.Numbers {
					num := plabel + NumberSeparator + k
					globalNumbers[num] = v
					numbersForAlias[plabel][k] = v
				}
			}
		}
	}

	// generate relative numbers
	for i, node := range nm.Nodes {

		// node self
		for num, val := range node.Numbers {
			node.RelativeNumbers[num] = val
		}

		// node group
		for _, group := range node.Groups {
			// groups: smaller group is forward, larger group is backward
			for k, val := range group.Numbers {
				// prioritize numbers by node-num > smaller-group-num > large-group-num
				num := NumberPrefixGroup + k
				if _, ok := node.RelativeNumbers[num]; !ok {
					node.RelativeNumbers[num] = val
				}
				// alias for group classes (for multi-layer groups)
				for _, label := range group.Labels.ClassLabels {
					cnum := label + "_" + k
					if _, ok := node.RelativeNumbers[cnum]; !ok {
						node.RelativeNumbers[cnum] = val
					}
				}
			}
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

		for _, iface := range node.Interfaces {

			// interface self
			for num, val := range iface.Numbers {
				iface.RelativeNumbers[num] = val
			}

			// node of interface
			for nodenum, val := range node.Numbers {
				num := NumberPrefixNode + nodenum
				iface.RelativeNumbers[num] = val
			}

			// node group of interface
			for _, group := range node.Groups {
				for k, val := range group.Numbers {
					num := NumberPrefixGroup + k
					if _, ok := iface.RelativeNumbers[num]; !ok {
						iface.RelativeNumbers[num] = val
					}
					// alias for group classes (for multi-layer groups)
					for _, label := range group.Labels.ClassLabels {
						cnum := label + "_" + k
						if _, ok := iface.RelativeNumbers[cnum]; !ok {
							iface.RelativeNumbers[cnum] = val
						}
					}
				}
			}

			// opposite interface
			if iface.Connection != nil {
				oppIf := iface.Opposite
				for oppnum, val := range oppIf.Numbers {
					num := NumberPrefixOppositeInterface + oppnum
					iface.RelativeNumbers[num] = val
				}

				// node of opposite interface
				oppNode := oppIf.Node
				for oppnnum, val := range oppNode.Numbers {
					num := NumberPrefixOppositeHeader + NumberPrefixNode + oppnnum
					iface.RelativeNumbers[num] = val
				}

				// node group of opposite interface
				for _, group := range oppNode.Groups {
					for k, val := range group.Numbers {
						num := NumberPrefixOppositeHeader + NumberPrefixGroup + k
						if _, ok := iface.RelativeNumbers[num]; !ok {
							iface.RelativeNumbers[num] = val
						}
						// alias for group classes (for multi-layer groups)
						for _, label := range group.Labels.ClassLabels {
							cnum := NumberPrefixOppositeInterface + label + "_" + k
							if _, ok := iface.RelativeNumbers[cnum]; !ok {
								iface.RelativeNumbers[cnum] = val
							}
						}
					}
				}
			}

			// global namespace of PlaceLabels
			for k, v := range globalNumbers {
				iface.RelativeNumbers[k] = v
			}

			// alias of MetaValueLabels
			if len(iface.Labels.MetaValueLabels) > 0 {
				for mvlabel, target := range iface.Labels.MetaValueLabels {
					if _, ok := numbersForAlias[target]; !ok {
						return fmt.Errorf("invalid MetaValueLabel for PlaceLabel %+v", target)
					}
					for k, v := range numbersForAlias[target] {
						num := mvlabel + NumberSeparator + k
						iface.RelativeNumbers[num] = v
					}
				}
			}
		}
	}

	return nil
}

func generateConfig(cfg *Config, nm *NetworkModel, output string) error {
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
				if !nct.outputSet.Contains(output) {
					continue
				}
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
					if !ict.outputSet.Contains(output) {
						continue
					}
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

			if iface.Connection == nil {
				continue
			}
			for _, cls := range iface.Connection.Labels.ClassLabels {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined ConnectionClass name %v", cls)
				}
				for _, cct := range cc.ConfigTemplates {
					if !cct.outputSet.Contains(output) {
						continue
					}
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
