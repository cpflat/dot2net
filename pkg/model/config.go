package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/goccy/go-yaml"
	// "gopkg.in/yaml.v2"
	// "github.com/spf13/viper"
)

const ClassTypeNode string = "nodeclass"
const ClassTypeInterface string = "interfaceclass"
const ClassTypeConnection string = "connectionclass"
const ClassTypeGroup string = "groupclass"
const ClassTypeNeighbor string = "neighborclass"
const ClassTypeMember string = "memberclass"

const ClassAll string = "all"         // all objects
const ClassDefault string = "default" // all empty objects
const PlaceLabelPrefix string = "@"
const ValueLabelSeparator string = "="

const PathSpecificationDefault string = "default" // search files from working directory
const PathSpecificationLocal string = "local"     // search files from the directory with config file

// IP number replacer: [IPSpace]_[IPReplacerXX]
const IPLoopbackReplacerFooter string = "loopback"
const IPAddressReplacerFooter string = "addr"
const IPNetworkReplacerFooter string = "net"
const IPProtocolReplacerFooter string = "protocol"
const IPPrefixLengthReplacerFooter string = "plen"

const OutputTinet string = "tinet"
const OutputClab string = "clab"
const OutputAsis string = "command"

func AllOutput() []string {
	return []string{OutputTinet, OutputClab, OutputAsis}
}

type Config struct {
	Name               string               `yaml:"name" mapstructure:"name"`
	GlobalSettings     GlobalSettings       `yaml:"global" mapstructure:"global"`
	FileDefinitions    []*FileDefinition    `yaml:"file" mapstructure:"file"`
	IPSpaceDefinitions []*IPSpaceDefinition `yaml:"ipspace" mapstructure:"ipspace"`
	NodeClasses        []*NodeClass         `yaml:"nodeclass,flow" mapstructure:"nodes,flow"`
	InterfaceClasses   []*InterfaceClass    `yaml:"interfaceclass,flow" mapstructure:"interfaces,flow"`
	ConnectionClasses  []*ConnectionClass   `yaml:"connectionclass,flow" mapstructure:"connections,flow"`
	GroupClasses       []*GroupClass        `yaml:"groupclass,flow" mapstructure:"group,flow"`

	fileDefinitionMap    map[string]*FileDefinition
	ipSpaceDefinitionMap map[string]*IPSpaceDefinition
	nodeClassMap         map[string]*NodeClass
	interfaceClassMap    map[string]*InterfaceClass
	connectionClassMap   map[string]*ConnectionClass
	groupClassMap        map[string]*GroupClass
	neighborClassMap     map[string]map[string][]*NeighborClass // interfaceclass name, ipspace name
	mgmtIPSpace          *IPSpaceDefinition
	localDir             string
}

func (cfg *Config) FileDefinitionByName(name string) (*FileDefinition, bool) {
	filedef, ok := cfg.fileDefinitionMap[name]
	return filedef, ok
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

func (cfg *Config) NeighborClassesByName(iface string, ipspace string) ([]*NeighborClass, bool) {
	ncs, ok := cfg.neighborClassMap[iface][ipspace]
	return ncs, ok
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

func (cfg *Config) classifyLabels(given []string) *parsedLabels {
	pl := newParsedLabels()
	for _, label := range given {
		if label == "" {
		} else if strings.HasPrefix(label, PlaceLabelPrefix) {
			if strings.Contains(label, ValueLabelSeparator) {
				// with "@" and include "=" -> MetaValueLabel
				sep := strings.SplitN(strings.TrimPrefix(label, PlaceLabelPrefix), ValueLabelSeparator, 2)
				mvlabel := sep[0]
				value := sep[1]
				pl.metaValueLabels[mvlabel] = value
			} else {
				// with "@" -> PlaceLabel
				plabel := strings.TrimPrefix(label, PlaceLabelPrefix)
				pl.placeLabels = append(pl.placeLabels, plabel)
			}
		} else {
			if strings.Contains(label, ValueLabelSeparator) {
				// include "=" -> ValueLabel
				sep := strings.SplitN(label, ValueLabelSeparator, 2)
				vlabel := sep[0]
				value := sep[1]
				pl.valueLabels[vlabel] = value
			} else {
				// ClassLabel
				pl.classLabels = append(pl.classLabels, label)
			}
		}
	}
	return pl
}

func (cfg *Config) getValidClasses(given []string, hasAll bool, hasDefault bool) *parsedLabels {
	pl := cfg.classifyLabels(given)
	classLabels := pl.classLabels

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

	pl.classLabels = classes
	return pl
}

func (cfg *Config) getValidNodeClasses(given []string) *parsedLabels {
	_, hasAllNodeClass := cfg.nodeClassMap[ClassAll]
	_, hasDefaultNodeClass := cfg.nodeClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllNodeClass, hasDefaultNodeClass)
}

func (cfg *Config) getValidInterfaceClasses(given []string) *parsedLabels {
	_, hasAllInterfaceClass := cfg.interfaceClassMap[ClassAll]
	_, hasDefaultInterfaceClass := cfg.interfaceClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllInterfaceClass, hasDefaultInterfaceClass)
}

func (cfg *Config) getValidConnectionClasses(given []string) *parsedLabels {
	_, hasAllConnectionClass := cfg.connectionClassMap[ClassAll]
	_, hasDefaultConnectionClass := cfg.connectionClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllConnectionClass, hasDefaultConnectionClass)
}

func (cfg *Config) getValidGroupClasses(given []string) *parsedLabels {
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

type FileDefinition struct {
	Name string `yaml:"name" mapstructure:"name"`
	// Path is the path that the generated file is placed on the node.
	// If empty, the file is generated but not placed on the node.
	Path string `yaml:"path" mapstructure:"path"`
	// Format is used to determine format and the way to aggregate the config blocks
	// The value can be "shell", "frr", etc. "file" in default.
	Format string `yaml:"format" mapstructure:"format"`
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

func (ipspace *IPSpaceDefinition) IPProtocolReplacer() string {
	return ipspace.Name + "_" + IPProtocolReplacerFooter
}

func (ipspace *IPSpaceDefinition) IPLoopbackReplacer() string {
	return ipspace.Name + "_" + IPLoopbackReplacerFooter
}

type ObjectClass interface{}

type NodeClass struct {
	// A node can have only one "primary" node class.
	// Unprimary node classes only have "name", "numbered" and "config". Other attributes are ignored.
	// A virtual node have parameters, but no object nor configuration. It is considered only on parameter assignment.
	Name                  string            `yaml:"name" mapstructure:"name"`
	Primary               bool              `yaml:"primary" mapstructure:"primary"`
	Virtual               bool              `yaml:"virtual" mapstructure:"virtual"`
	IPAware               []string          `yaml:"ipaware" mapstructure:"ipaware"` // aware ip spaces for loopback
	IPAwareIgnoreDefaults bool              `yaml:"ipaware_ignore_defaults" mapstructure:"ipaware_ignore_defaults"`
	Numbered              []string          `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	ConfigTemplates       []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	MemberClasses         []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix        string                 `yaml:"prefix" mapstructure:"prefix"`                 // prefix of auto-naming
	MgmtInterface string                 `yaml:"mgmt_interface" mapstructure:"mgmt_interface"` // InterfaceClass name for mgmt
	TinetAttr     map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`                   // tinet attributes
	ClabAttr      map[string]interface{} `yaml:"clab" mapstructure:"clab"`                     // containerlab attributes
}

type InterfaceClass struct {
	// An interface can have only one of "primary" interface class or "primary" connection class.
	Name                  string            `yaml:"name" mapstructure:"name"`
	Primary               bool              `yaml:"primary" mapstructure:"primary"`
	Virtual               bool              `yaml:"virtual" mapstructure:"virtual"`
	Numbered              []string          `yaml:"numbered,flow" mapstructure:"numbered,flow"`
	IPAware               []string          `yaml:"ipaware" mapstructure:"ipaware"` // aware ip spaces
	IPAwareIgnoreNode     bool              `yaml:"ipaware_ignore_node" mapstructure:"ipaware_ignore_node"`
	IPAwareIgnoreDefaults bool              `yaml:"ipaware_ignore_defaults" mapstructure:"ipaware_ignore_defaults"`
	ConfigTemplates       []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	NeighborClasses       []*NeighborClass  `yaml:"neighbors,flow" mapstructure:"neighbors,flow"`
	MemberClasses         []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix    string                 `yaml:"prefix" mapstructure:"prefix"` // prefix of auto-naming
	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`   // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`     // containerlab attributes
}

// type ConnectionClass struct {
type ConnectionClass struct {
	Name                  string            `yaml:"name" mapstructure:"name"`
	Primary               bool              `yaml:"primary" mapstructure:"primary"`
	Virtual               bool              `yaml:"virtual" mapstructure:"virtual"`
	Numbered              []string          `yaml:"numbered,flow" mapstructure:"numbered,flow"` // Numbers to be assigned automatically
	IPAware               []string          `yaml:"ipaware,flow" mapstructure:"ipaware,flow"`   // aware ip spaces for end interfaces
	IPAwareIgnoreNode     bool              `yaml:"ipaware_ignore_node" mapstructure:"ipaware_ignore_node"`
	IPAwareIgnoreDefaults bool              `yaml:"ipaware_ignore_defaults" mapstructure:"ipaware_ignore_defaults"`
	IPSpaces              []string          `yaml:"ipspaces,flow" mapstructure:"ipspaces,flow"` // Connection is limited to specified spaces
	ConfigTemplates       []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	MemberClasses         []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`
	NeighborClasses       []*NeighborClass  `yaml:"neighbors,flow" mapstructure:"neighbors,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix    string                 `yaml:"prefix" mapstructure:"prefix"` // prefix of interface auto-naming
	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`   // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`     // containerlab attributes
}

type GroupClass struct {
	Name     string   `yaml:"name" mapstructure:"name"`
	Numbered []string `yaml:"numbered,flow" mapstructure:"numbered,flow"`
}

type NeighborClass struct {
	IPSpace         string            `yaml:"ipspace" mapstructure:"ipspace"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
}

type MemberClass struct {
	NodeClass         string            `yaml:"node" mapstructure:"node"`
	NodeClasses       []string          `yaml:"nodes" mapstructure:"nodes"`
	InterfaceClass    string            `yaml:"interface" mapstructure:"interface"`
	InterfaceClasses  []string          `yaml:"interfaces" mapstructure:"interfaces"`
	ConnectionClass   string            `yaml:"connection" mapstructure:"connection"`
	ConnectionClasses []string          `yaml:"connections" mapstructure:"connections"`
	IncludeSelf       bool              `yaml:"include_self" mapstructure:"include_self"`
	ConfigTemplates   []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
}

type ConfigTemplate struct {
	// Target file definition name
	File string `yaml:"file" mapstructure:"file"`
	// add config only for interfaces of nodes belongs to the nodeclass(es)
	// this option is valid only on InterfaceClass, ConnectionClass, and their NeighborClass
	NodeClass   string   `yaml:"node" mapstructure:"node"`
	NodeClasses []string `yaml:"nodes" mapstructure:"nodes"`
	// add config only if the neighbor node belongs to the nodeclass(es)
	// this option is valid only on NeighborClass
	NeighborNodeClass   string   `yaml:"neighbor_node" mapstructure:"neighbor_node"`
	NeighborNodeClasses []string `yaml:"neighbor_nodes" mapstructure:"neighbor_nodes"`
	// This option is valid only on InterfaceClass or ConnectionClass
	// If specified, add config only for included output (e.g., tinet only, clab only, etc)
	Platform []string `yaml:"platform,flow" mapstructure:"platform,flow"`
	// Style is used to iterpret the given config format. Style can be different on one file. As-is in default.
	Style string `yaml:"style" mapstructure:"style"`
	// Priority is a value to be used for sorting config blocks. 0 in default.
	Priority int `yaml:"priority" mapstructure:"priority"`
	// Load config template
	Template []string `yaml:"template" mapstructure:"template"`
	// Load config template from external file
	SourceFile string `yaml:"sourcefile" mapstructure:"sourcefile"`

	parsedTemplate *template.Template
	platformSet    mapset.Set[string]
}

func (ct *ConfigTemplate) NodeClassCheck(node *Node) bool {
	if len(ct.NodeClasses) == 0 {
		if ct.NodeClass == "" {
			// No nodeclass constraint, always true
			return true
		} else {
			return node.HasClass(ct.NodeClass)
		}
	} else {
		ncs := make([]string, 0, len(ct.NodeClasses)+1)
		copy(ncs, ct.NodeClasses)
		if ct.NodeClass != "" {
			ncs = append(ncs, ct.NodeClass)
		}

		for _, nc := range ncs {
			if node.HasClass(nc) {
				return true
			}
		}
		return false
	}
}

func (ct *ConfigTemplate) NeighborNodeClassCheck(node *Node) bool {
	if len(ct.NeighborNodeClasses) == 0 {
		if ct.NeighborNodeClass == "" {
			// No nodeclass constraint, always true
			return true
		} else {
			return node.HasClass(ct.NeighborNodeClass)
		}
	} else {
		ncs := make([]string, 0, len(ct.NeighborNodeClasses)+1)
		copy(ncs, ct.NeighborNodeClasses)
		if ct.NeighborNodeClass != "" {
			ncs = append(ncs, ct.NeighborNodeClass)
		}

		for _, nc := range ncs {
			if node.HasClass(nc) {
				return true
			}
		}
		return false
	}
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

	// add empty filedef for embedded conifg
	cfg.FileDefinitions = append(cfg.FileDefinitions, &FileDefinition{Name: "", Format: "shell"})

	cfg.localDir = filepath.Dir(path)
	cfg.fileDefinitionMap = map[string]*FileDefinition{}
	for _, filedef := range cfg.FileDefinitions {
		cfg.fileDefinitionMap[filedef.Name] = filedef
	}
	cfg.ipSpaceDefinitionMap = map[string]*IPSpaceDefinition{}
	for _, ipspace := range cfg.IPSpaceDefinitions {
		cfg.ipSpaceDefinitionMap[ipspace.Name] = ipspace
	}
	cfg.nodeClassMap = map[string]*NodeClass{}
	for _, node := range cfg.NodeClasses {
		cfg.nodeClassMap[node.Name] = node
	}
	cfg.interfaceClassMap = map[string]*InterfaceClass{}
	cfg.neighborClassMap = map[string]map[string][]*NeighborClass{}
	for _, iface := range cfg.InterfaceClasses {
		cfg.interfaceClassMap[iface.Name] = iface
		for _, neighbor := range iface.NeighborClasses {
			if _, ok := cfg.neighborClassMap[iface.Name]; !ok {
				cfg.neighborClassMap[iface.Name] = map[string][]*NeighborClass{}
			}
			cfg.neighborClassMap[iface.Name][neighbor.IPSpace] = append(
				cfg.neighborClassMap[iface.Name][neighbor.IPSpace], neighbor,
			)
		}
	}
	cfg.connectionClassMap = map[string]*ConnectionClass{}
	for _, conn := range cfg.ConnectionClasses {
		cfg.connectionClassMap[conn.Name] = conn
	}
	cfg.groupClassMap = map[string]*GroupClass{}
	for _, group := range cfg.GroupClasses {
		cfg.groupClassMap[group.Name] = group
	}
	return &cfg, err
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

func initConfigTemplate(cfg *Config, ct *ConfigTemplate) error {
	var outputs []string
	ct.platformSet = mapset.NewSet[string]()
	if len(ct.Platform) == 0 {
		outputs = AllOutput()
	} else {
		outputs = ct.Platform
	}
	for _, output := range outputs {
		ct.platformSet.Add(output)
	}

	// init parsed template object
	path := ""
	if ct.SourceFile != "" {
		path = getPath(ct.SourceFile, cfg)
	}
	tpl, err := loadTemplate(ct.Template, path)
	if err != nil {
		return err
	}
	ct.parsedTemplate = tpl

	return nil
}

func loadTemplates(cfg *Config) (*Config, error) {
	for _, nc := range cfg.NodeClasses {
		for _, ct := range nc.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
		}
		for _, mc := range nc.MemberClasses {
			for _, ct := range mc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, ic := range cfg.InterfaceClasses {
		for _, ct := range ic.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
		}
		for _, nc := range ic.NeighborClasses {
			for _, ct := range nc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
			}
		}
		for _, mc := range ic.MemberClasses {
			for _, ct := range mc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, cc := range cfg.ConnectionClasses {
		for _, ct := range cc.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
		}
		for _, nc := range cc.NeighborClasses {
			for _, ct := range nc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
			}
		}
		for _, mc := range cc.MemberClasses {
			for _, ct := range mc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
			}
		}
	}
	return cfg, nil
}
