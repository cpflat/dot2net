package types

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

const ClassTypeNetwork string = "network"
const ClassTypeNode string = "node"
const ClassTypeInterface string = "interface"
const ClassTypeConnection string = "connection"
const ClassTypeGroup string = "group"
const ClassTypeSegment string = "segment"
const ClassTypeNeighborHeader string = "neighbor"
const ClassTypeNeighborLayerAny = "any"
const ClassTypeMemberHeader string = "member"
const ClassTypeMemberClassNameAny = "any"

func ClassTypeNeighbor(layer string) string {
	return ClassTypeNeighborHeader + "_" + layer
}

func ClassTypeMember(classType string, className string) string {
	return ClassTypeMemberHeader + "_" + classType + "_" + className
}

const ClassAll string = "all"         // all objects
const ClassDefault string = "default" // all empty objects
const PlaceLabelPrefix string = "@"
const ValueLabelSeparator string = "="
const RelationalClassLabelSeparator string = "#"

const PathSpecificationDefault string = "default" // search files from working directory
const PathSpecificationLocal string = "local"     // search files from the directory with config file

const MountSourcePathAbs string = "abs" // absolute path
const MountSourcePathLocal string = "local"

const ConfigTemplateStyleHierarchy string = "hierarchy" // ConfigTemplate.Style
const ConfigTemplateStyleSort string = "sort"

// IP number replacer: [IPSpace]_[IPReplacerXX]
// const IPLoopbackReplacerFooter string = "loopback"
const IPLoopbackReplacerFooter string = "loopback"
const IPAddressReplacerFooter string = "addr"
const IPNetworkReplacerFooter string = "net"
const IPProtocolReplacerFooter string = "protocol"
const IPPrefixLengthReplacerFooter string = "plen"

const IPPolicyTypeDefault string = "ip"
const IPPolicyTypeLoopback string = "loopback"

const OutputTinet string = "tinet"
const OutputClab string = "clab"
const OutputAsis string = "command"

func AllOutput() []string {
	return []string{OutputTinet, OutputClab, OutputAsis}
}

// config elements

type Config struct {
	Name            string            `yaml:"name" mapstructure:"name"`
	Modules         []string          `yaml:"module" mapstructure:"module"`
	GlobalSettings  GlobalSettings    `yaml:"global" mapstructure:"global"`
	FileDefinitions []*FileDefinition `yaml:"file" mapstructure:"file"`
	FileFormats     []*FileFormat     `yaml:"format,flow" mapstructure:"format,flow"`
	Layers          []*Layer          `yaml:"layer" mapstructure:"layer"`
	ManagementLayer ManagementLayer   `yaml:"mgmt_layer" mapstructure:"mgmt_layer"`
	ParameterRules  []*ParameterRule  `yaml:"param_rule,flow" mapstructure:"param_rule,flow"`

	NetworkClasses    []*NetworkClass    `yaml:"networkclass,flow" mapstructure:"network,flow"`
	NodeClasses       []*NodeClass       `yaml:"nodeclass,flow" mapstructure:"nodes,flow"`
	InterfaceClasses  []*InterfaceClass  `yaml:"interfaceclass,flow" mapstructure:"interfaces,flow"`
	ConnectionClasses []*ConnectionClass `yaml:"connectionclass,flow" mapstructure:"connections,flow"`
	GroupClasses      []*GroupClass      `yaml:"groupclass,flow" mapstructure:"group,flow"`
	SegmentClasses    []*SegmentClass    `yaml:"segmentclass,flow" mapstructure:"segments,flow"`

	fileDefinitionMap map[string]*FileDefinition
	fileFormatMap     map[string]*FileFormat
	layerMap          map[string]*Layer
	policyMap         map[string]*IPPolicy
	parameterRuleMap  map[string]*ParameterRule

	nodeClassMap       map[string]*NodeClass
	interfaceClassMap  map[string]*InterfaceClass
	connectionClassMap map[string]*ConnectionClass
	groupClassMap      map[string]*GroupClass
	segmentClassMap    map[string]*SegmentClass
	neighborClassMap   map[string]map[string][]*NeighborClass // interfaceclass name, ipspace name
	localDir           string

	LoadedModules              []Module           // reference to loaded modules, internal
	SorterConfigTemplateGroups mapset.Set[string] // list of sort-style config template groups
}

func (cfg *Config) FileDefinitionByName(name string) (*FileDefinition, bool) {
	filedef, ok := cfg.fileDefinitionMap[name]
	return filedef, ok
}

func (cfg *Config) FileFormatByName(name string) (*FileFormat, bool) {
	filefmt, ok := cfg.fileFormatMap[name]
	return filefmt, ok
}

func (cfg *Config) LayerByName(name string) (*Layer, bool) {
	layer, ok := cfg.layerMap[name]
	return layer, ok
}

func (cfg *Config) ParameterRuleByName(name string) (*ParameterRule, bool) {
	rule, ok := cfg.parameterRuleMap[name]
	return rule, ok
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

func (cfg *Config) SegmentClassByName(name string) (*SegmentClass, bool) {
	sc, ok := cfg.segmentClassMap[name]
	return sc, ok
}

func (cfg *Config) NeighborClassesByName(iface string, ipspace string) ([]*NeighborClass, bool) {
	ncs, ok := cfg.neighborClassMap[iface][ipspace]
	return ncs, ok
}

func (cfg *Config) DefaultConnectionLayer() []string {
	layers := []string{}
	for _, layer := range cfg.Layers {
		if layer.DefaultConnect {
			layers = append(layers, layer.Name)
		}
	}
	return layers
}

func (cfg *Config) classifyLabels(given []string) *ParsedLabels {
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
			if strings.Contains(label, RelationalClassLabelSeparator) {
				// include "#" -> RelationalClassLabel
				sep := strings.SplitN(label, RelationalClassLabelSeparator, 2)
				rlabel := RelationalClassLabel{ClassType: sep[0], Name: sep[1]}
				pl.rClassLabels = append(pl.rClassLabels, rlabel)
			} else if strings.Contains(label, ValueLabelSeparator) {
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

func (cfg *Config) getValidClasses(given []string, hasAll bool, hasDefault bool) *ParsedLabels {
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

func (cfg *Config) GetValidNodeClasses(given []string) *ParsedLabels {
	_, hasAllNodeClass := cfg.nodeClassMap[ClassAll]
	_, hasDefaultNodeClass := cfg.nodeClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllNodeClass, hasDefaultNodeClass)
}

func (cfg *Config) GetValidInterfaceClasses(given []string) *ParsedLabels {
	_, hasAllInterfaceClass := cfg.interfaceClassMap[ClassAll]
	_, hasDefaultInterfaceClass := cfg.interfaceClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllInterfaceClass, hasDefaultInterfaceClass)
}

func (cfg *Config) GetValidConnectionClasses(given []string) *ParsedLabels {
	_, hasAllConnectionClass := cfg.connectionClassMap[ClassAll]
	_, hasDefaultConnectionClass := cfg.connectionClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllConnectionClass, hasDefaultConnectionClass)
}

func (cfg *Config) GetValidGroupClasses(given []string) *ParsedLabels {
	_, hasAllGroupClass := cfg.groupClassMap[ClassAll]
	_, hasDefaultGroupClass := cfg.groupClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllGroupClass, hasDefaultGroupClass)
}

func (cfg *Config) GetValidSegmentClasses(given []string) *ParsedLabels {
	_, hasAllSegmentClass := cfg.segmentClassMap[ClassAll]
	_, hasDefaultSegmentClass := cfg.segmentClassMap[ClassDefault]
	return cfg.getValidClasses(given, hasAllSegmentClass, hasDefaultSegmentClass)
}

func (cfg *Config) AddFileFormat(filefmt *FileFormat) {
	cfg.FileFormats = append(cfg.FileFormats, filefmt)
	cfg.fileFormatMap[filefmt.Name] = filefmt
}

func (cfg *Config) AddFileDefinition(filedef *FileDefinition) {
	cfg.FileDefinitions = append(cfg.FileDefinitions, filedef)
	cfg.fileDefinitionMap[filedef.Name] = filedef
}

func (cfg *Config) AddNetworkClass(nc *NetworkClass) {
	cfg.NetworkClasses = append(cfg.NetworkClasses, nc)
}

func (cfg *Config) AddNodeClass(nc *NodeClass) {
	cfg.NodeClasses = append(cfg.NodeClasses, nc)
	cfg.nodeClassMap[nc.Name] = nc
}

func (cfg *Config) AddInterfaceClass(nc *InterfaceClass) {
	cfg.InterfaceClasses = append(cfg.InterfaceClasses, nc)
	cfg.interfaceClassMap[nc.Name] = nc
}

func (cfg *Config) AddConnectionClass(nc *ConnectionClass) {
	cfg.ConnectionClasses = append(cfg.ConnectionClasses, nc)
	cfg.connectionClassMap[nc.Name] = nc
}

func (cfg *Config) HasManagementLayer() bool {
	return cfg.ManagementLayer.AddrRange != ""
}

func (cfg *Config) MountSourcePath(path string) (string, error) {
	if cfg.GlobalSettings.MountSourcePath == MountSourcePathAbs {
		return filepath.Abs(path)
	} else {
		return path, nil
	}
}

type GlobalSettings struct {
	PathSpecification string `yaml:"path" mapstructure:"path"`
	MountSourcePath   string `yaml:"mountsourcepath" mapstructure:"mountsourcepath"`
	NodeAutoRename    bool   `yaml:"nodeautoname" mapstructure:"nodeautoname"`
	// ASNumberMin and ASNumberMAX are optional, considered in AssignASNumbers if specified
	ASNumberMin int `yaml:"asnumber_min" mapstructure:"asnumber_min"`
	ASNumberMax int `yaml:"asnumber_max" mapstructure:"asnumber_max"`

	ClabAttr map[string]interface{} `yaml:"clab" mapstructure:"clab"` // containerlab attributes
}

type FileDefinition struct {
	// Name is used as the filename of generated file.
	Name string `yaml:"name" mapstructure:"name"`
	// Path is the path that the generated file is placed on the node.
	// If empty, the file is generated but not placed on the node.
	Path string `yaml:"path" mapstructure:"path"`
	// Format is used to determine the way to format lines in generated config text.
	Format  string   `yaml:"format" mapstructure:"format"`
	Formats []string `yaml:"formats,flow" mapstructure:"formats,flow"`
	// Scope specifies the scope of file creation.
	// For example, if Scope = "node", the file is created for each node.
	Scope string `yaml:"scope" mapstructure:"scope"`
	//// Shared flag is used to determine the file is shared among nodes or not.
	//// If true, the file is placed on the same directory as primary config file. -> To be removed
	// Shared bool `yaml:"shared" mapstructure:"shared"`
}

func (fd *FileDefinition) GetFormats() []string {
	ret := []string{}
	if fd.Format != "" {
		ret = append(ret, fd.Format)
	}
	if len(fd.Formats) > 0 {
		ret = append(ret, fd.Formats...)
	}
	return ret
}

type FileFormat struct {
	Name           string `yaml:"name" mapstructure:"name"`
	LinePrefix     string `yaml:"lineprefix" mapstructure:"lineprefix"`
	LineSuffix     string `yaml:"linesuffix" mapstructure:"linesuffix"`
	LineSeparator  string `yaml:"lineseparator" mapstructure:"lineseparator"`
	BlockPrefix    string `yaml:"blockprefix" mapstructure:"blockprefix"`
	BlockSuffix    string `yaml:"blocksuffix" mapstructure:"blocksuffix"`
	BlockSeparator string `yaml:"blockseparator" mapstructure:"blockseparator"`
}

type Layerer interface {
	IPAddressReplacer() string
	IPNetworkReplacer() string
	IPPrefixLengthReplacer() string
}

type Layer struct {
	Name string `yaml:"name" mapstructure:"name"`
	// If default_connect is true, ConnectionClasses without ipspaces field are considered as connected on this Layer
	DefaultConnect bool        `yaml:"default_connect" mapstructure:"default_connect"`
	Policies       []*IPPolicy `yaml:"policy" mapstructure:"policy"`

	Layerer

	IPPolicy       []*IPPolicy
	LoopbackPolicy []*IPPolicy
}

func (layer *Layer) IPAddressReplacer() string {
	return layer.Name + "_" + IPAddressReplacerFooter
}

func (layer *Layer) IPNetworkReplacer() string {
	return layer.Name + "_" + IPNetworkReplacerFooter
}

func (layer *Layer) IPPrefixLengthReplacer() string {
	return layer.Name + "_" + IPPrefixLengthReplacerFooter
}

func (layer *Layer) IPProtocolReplacer() string {
	return layer.Name + "_" + IPProtocolReplacerFooter
}

func (layer *Layer) IPLoopbackReplacer() string {
	return layer.Name + "_" + IPLoopbackReplacerFooter
}

type ManagementLayer struct {
	Name      string `yaml:"name" mapstructure:"name"`
	AddrRange string `yaml:"range" mapstructure:"range"`
	// gateway is used only for management network or external network
	// the address is avoided in automated IPaddress assignment
	ExternalGateway string `yaml:"gateway" mapstructure:"gateway"`
	InterfaceName   string `yaml:"interface_name" mapstructure:"mgmt_name"`

	Layerer
}

func (layer *ManagementLayer) IPAddressReplacer() string {
	return layer.Name + "_" + IPAddressReplacerFooter
}

func (layer *ManagementLayer) IPNetworkReplacer() string {
	return layer.Name + "_" + IPNetworkReplacerFooter
}

func (layer *ManagementLayer) IPPrefixLengthReplacer() string {
	return layer.Name + "_" + IPPrefixLengthReplacerFooter
}

type IPPolicy struct {
	Name string `yaml:"name" mapstructure:"name"`
	// type: ip (deafult), loopback, mgmt
	Type                string `yaml:"type" mapstructure:"type"`
	AddrRange           string `yaml:"range" mapstructure:"range"`
	DefaultPrefixLength int    `yaml:"prefix" mapstructure:"prefix"`

	layer *Layer
}

type ParameterRule struct {
	Name string `yaml:"name" mapstructure:"name"`
	// object (in default) or segment
	Assign string `yaml:"assign" mapstructure:"assign"`
	// layer is used only when the assign option is "segment"
	Layer string `yaml:"layer" mapstructure:"layer"`
	// integer (in default) or file
	Type string `yaml:"type" mapstructure:"type"`
	// for type integer
	Max    int    `yaml:"max" mapstructure:"max"`
	Min    int    `yaml:"min" mapstructure:"min"`
	Header string `yaml:"header" mapstructure:"header"`
	Footer string `yaml:"footer" mapstructure:"footer"`
	// for type file
	SourceFile string `yaml:"sourcefile" mapstructure:"soucefile"`
}

// interfaces and abstracted structs for object classes

type ObjectClass interface{}

type LabelOwnerClass interface {
	GetGivenValues() map[string]string
}

// object classes

type NetworkClass struct {
	Name            string            `yaml:"name" mapstructure:"name"`
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	LabelOwnerClass
}

func (nc *NetworkClass) GetGivenValues() map[string]string {
	return nc.Values
}

type NodeClass struct {
	// A node can have only one "primary" node class.
	// Unprimary node classes only have "name", "numbered" and "config". Other attributes are ignored.
	// A virtual node have parameters, but no object nor configuration. It is considered only on parameter assignment.
	Name              string            `yaml:"name" mapstructure:"name"`
	Primary           bool              `yaml:"primary" mapstructure:"primary"`
	Virtual           bool              `yaml:"virtual" mapstructure:"virtual"`
	IPPolicy          []string          `yaml:"policy,flow" mapstructure:"policy,flow"`
	Parameters        []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values            map[string]string `yaml:"values" mapstructure:"values"`
	InterfaceIPPolicy []string          `yaml:"interface_policy,flow" mapstructure:"interface_policy,flow"`
	ConfigTemplates   []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	MemberClasses     []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix        string                 `yaml:"prefix" mapstructure:"prefix"`                           // prefix of auto-naming
	MgmtInterface string                 `yaml:"mgmt_interfaceclass" mapstructure:"mgmt_interfaceclass"` // InterfaceClass name for mgmt
	TinetAttr     map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`                             // tinet attributes
	ClabAttr      map[string]interface{} `yaml:"clab" mapstructure:"clab"`                               // containerlab attributes

	LabelOwnerClass
}

func (nc *NodeClass) GetGivenValues() map[string]string {
	return nc.Values
}

type InterfaceClass struct {
	// An interface can have only one of "primary" interface class or "primary" connection class.
	Name            string            `yaml:"name" mapstructure:"name"`
	Primary         bool              `yaml:"primary" mapstructure:"primary"`
	Virtual         bool              `yaml:"virtual" mapstructure:"virtual"`
	IPPolicy        []string          `yaml:"policy,flow" mapstructure:"policy,flow"`
	Parameters      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	NeighborClasses []*NeighborClass  `yaml:"neighbors,flow" mapstructure:"neighbors,flow"`
	MemberClasses   []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix    string                 `yaml:"prefix" mapstructure:"prefix"` // prefix of auto-naming
	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`   // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`     // containerlab attributes

	LabelOwnerClass
}

func (ic *InterfaceClass) GetGivenValues() map[string]string {
	return ic.Values
}

type ConnectionClass struct {
	Name            string            `yaml:"name" mapstructure:"name"`
	Primary         bool              `yaml:"primary" mapstructure:"primary"`
	Virtual         bool              `yaml:"virtual" mapstructure:"virtual"`
	IPPolicy        []string          `yaml:"policy,flow" mapstructure:"policy,flow"`
	Layers          []string          `yaml:"layers,flow" mapstructure:"layers,flow"` // Connection is limited to specified layers
	Parameters      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	MemberClasses   []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`
	NeighborClasses []*NeighborClass  `yaml:"neighbors,flow" mapstructure:"neighbors,flow"`

	// Following attributes are valid only on primary interface classes.
	Prefix    string                 `yaml:"prefix" mapstructure:"prefix"` // prefix of interface auto-naming
	TinetAttr map[string]interface{} `yaml:"tinet" mapstructure:"tinet"`   // tinet attributes
	ClabAttr  map[string]interface{} `yaml:"clab" mapstructure:"clab"`     // containerlab attributes

	LabelOwnerClass
}

func (cc *ConnectionClass) GetGivenValues() map[string]string {
	return cc.Values
}

type GroupClass struct {
	Name            string            `yaml:"name" mapstructure:"name"`
	Virtual         bool              `yaml:"virtual" mapstructure:"virtual"`
	Parameters      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	LabelOwnerClass
}

func (gc *GroupClass) GetGivenValues() map[string]string {
	return gc.Values
}

type SegmentClass struct {
	Name            string            `yaml:"name" mapstructure:"name"`
	Layer           string            `yaml:"layer" mapstructure:"layer"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
}

type NeighborClass struct {
	Layer           string            `yaml:"layer" mapstructure:"layer"`
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

// check MemberClass description and return (classtype, classnames)
func (mc *MemberClass) GetSpecifiedClasses() (string, []string, error) {
	classes := []string{}
	if mc.NodeClass != "" || len(mc.NodeClasses) > 0 {
		if mc.InterfaceClass != "" || len(mc.InterfaceClasses) > 0 {
			return "", nil, fmt.Errorf("nodeClass and interfaceClass cannot be specified at the same time")
		}
		if mc.ConnectionClass != "" || len(mc.ConnectionClasses) > 0 {
			return "", nil, fmt.Errorf("nodeClass and connectionClass cannot be specified at the same time")
		}
		if mc.NodeClass != "" {
			classes = append(classes, mc.NodeClass)
		}
		classes = append(classes, mc.NodeClasses...)
		return ClassTypeNode, classes, nil
	} else if mc.InterfaceClass != "" || len(mc.InterfaceClasses) > 0 {
		if mc.ConnectionClass != "" || len(mc.ConnectionClasses) > 0 {
			return "", nil, fmt.Errorf("interfaceClass and connectionClass cannot be specified at the same time")
		}
		if mc.InterfaceClass != "" {
			classes = append(classes, mc.InterfaceClass)
		}
		classes = append(classes, mc.InterfaceClasses...)
		return ClassTypeInterface, classes, nil
	} else if mc.ConnectionClass != "" || len(mc.ConnectionClasses) > 0 {
		if mc.ConnectionClass != "" {
			classes = append(classes, mc.ConnectionClass)
		}
		classes = append(classes, mc.ConnectionClasses...)
		return ClassTypeConnection, classes, nil
	} else {
		return "", nil, fmt.Errorf("no class specified for MemberClass")
	}
}

type ConfigTemplate struct {
	// Config block aggregation styles
	// hierarchy (default): specify child config templates in the template description as parameter
	// sort: merge child config templates of SortTarget groups into the "sort" config templates
	Style     string `yaml:"style" mapstructure:"style"`
	SortGroup string `yaml:"sort_group" mapstructure:"sort_group"`
	// Target file definition name
	// Config templates with file will generate a file of generated text
	File string `yaml:"file" mapstructure:"file"`
	// Name is used by parent objects to specify as childs in templates
	// Config templates with name will form a parameter that can be embeded in other hierarchy templates
	Name string `yaml:"name" mapstructure:"name"`
	// Group is used for sort config templates
	// A sort config template will aggregate all config blocks generated in child (or grandchild) objects of the same group
	Group string `yaml:"group" mapstructure:"group"`
	// Priority is used to reorder config templates in sort style
	// Config blocks with smaller priority should be on the top of generated config files
	// Default is 0, so users should specify negative values to make a config block top of a file
	Priority int `yaml:"priority" mapstructure:"priority"`
	// Used for hierarchy config templates
	// Config template names on same object that need to be embeded
	// Required for ordering config template generation considering the dependency
	Depends []string `yaml:"depends" mapstructure:"depends"`

	// Condition related fields
	// add config only for interfaces of nodes belongs to the nodeclass(es)
	// this option is valid only on InterfaceClass, ConnectionClass, and their NeighborClass
	NodeClass   string   `yaml:"node" mapstructure:"node"`
	NodeClasses []string `yaml:"nodes" mapstructure:"nodes"`
	// add config only if the neighbor node belongs to the nodeclass(es)
	// this option is valid only on NeighborClass
	NeighborNodeClass   string   `yaml:"neighbor_node" mapstructure:"neighbor_node"`
	NeighborNodeClasses []string `yaml:"neighbor_nodes" mapstructure:"neighbor_nodes"`
	// put empty file or namespace if conditions are not satisfied
	Empty bool `yaml:"empty" mapstructure:"empty"`

	// This option is valid only on InterfaceClass or ConnectionClass
	// If specified, add config only for included output (e.g., tinet only, clab only, etc)
	Platform []string `yaml:"platform,flow" mapstructure:"platform,flow"`
	// Style is used to iterpret the given config format. Style can be different on one file. As-is in default.
	//Style string `yaml:"style" mapstructure:"style"`
	Format  string   `yaml:"format" mapstructure:"format"`
	Formats []string `yaml:"formats" mapstructure:"formats"`
	// Priority is a value to be used for sorting config blocks. 0 in default.
	// Priority int `yaml:"priority" mapstructure:"priority"`
	// Load config template
	Template []string `yaml:"template" mapstructure:"template"`
	// Load config template from external file
	SourceFile string `yaml:"sourcefile" mapstructure:"sourcefile"`

	ParsedTemplate *template.Template
	platformSet    mapset.Set[string]
	className      string
	classType      string
}

func (ct *ConfigTemplate) String() string {
	info := []string{}
	if ct.Name != "" {
		info = append(info, fmt.Sprintf("name:%s", ct.Name))
	}
	if ct.File != "" {
		info = append(info, fmt.Sprintf("file:%s", ct.File))
	}
	if ct.Group != "" {
		info = append(info, fmt.Sprintf("group:%s", ct.Group))
	}
	if len(info) == 0 {
		return "ConfigTemplate(no info)"
	} else {
		return fmt.Sprintf("ConfigTemplate(%s)", strings.Join(info, ","))
	}
}

func (ct *ConfigTemplate) GetClassInfo() (string, string) {
	return ct.classType, ct.className
}

func (ct *ConfigTemplate) GetFormats() []string {
	ret := []string{}
	if ct.Format != "" {
		ret = append(ret, ct.Format)
	}
	if len(ct.Formats) > 0 {
		ret = append(ret, ct.Formats...)
	}
	return ret
}

// return true if conditions satisfied
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

func GetRelativeFilePath(path string, cfg *Config) string {
	pathspec := cfg.GlobalSettings.PathSpecification
	if pathspec == "local" {
		return filepath.Join(cfg.localDir, path)
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
	cfg.fileFormatMap = map[string]*FileFormat{}
	for _, filefmt := range cfg.FileFormats {
		cfg.fileFormatMap[filefmt.Name] = filefmt
	}
	cfg.layerMap = map[string]*Layer{}
	cfg.policyMap = map[string]*IPPolicy{}
	for _, layer := range cfg.Layers {
		cfg.layerMap[layer.Name] = layer
		for _, policy := range layer.Policies {
			policy.layer = layer
			cfg.policyMap[policy.Name] = policy
			switch policy.Type {
			case IPPolicyTypeDefault:
				layer.IPPolicy = append(layer.IPPolicy, policy)
			case IPPolicyTypeLoopback:
				layer.LoopbackPolicy = append(layer.LoopbackPolicy, policy)
			default:
				layer.IPPolicy = append(layer.IPPolicy, policy)
			}
		}
	}
	cfg.parameterRuleMap = map[string]*ParameterRule{}
	for _, prule := range cfg.ParameterRules {
		cfg.parameterRuleMap[prule.Name] = prule
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
			cfg.neighborClassMap[iface.Name][neighbor.Layer] = append(
				cfg.neighborClassMap[iface.Name][neighbor.Layer], neighbor,
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
	cfg.segmentClassMap = map[string]*SegmentClass{}
	cfg.SorterConfigTemplateGroups = mapset.NewSet[string]()

	return &cfg, err
}

func loadTemplate(tpl []string, path string) (*template.Template, error) {
	if len(tpl) == 0 && path == "" {
		return template.New("").Parse("")
		//return nil, fmt.Errorf("empty config template")
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

	// check if the config template is sort-style
	if ct.Style == ConfigTemplateStyleSort {
		if ct.SortGroup == "" {
			return fmt.Errorf("sort-style config template should have sort_group attribute")
		} else {
			cfg.SorterConfigTemplateGroups.Add(ct.SortGroup)
		}
	}

	// init parsed template object
	path := ""
	if ct.SourceFile != "" {
		path = GetRelativeFilePath(ct.SourceFile, cfg)
	}
	tpl, err := loadTemplate(ct.Template, path)
	if err != nil {
		return fmt.Errorf("failed to load template %+v: %w", ct, err)
	}
	ct.ParsedTemplate = tpl

	return nil
}

func LoadTemplates(cfg *Config) (*Config, error) {
	// className is set only for LabelOwners, for checking config template conditions of classnames
	for _, networkClass := range cfg.NetworkClasses {
		for _, ct := range networkClass.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
			ct.className = networkClass.Name
			ct.classType = ClassTypeNetwork
		}
	}
	for _, nc := range cfg.NodeClasses {
		for _, ct := range nc.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
			ct.className = nc.Name
			ct.classType = ClassTypeNode
		}
		for _, mc := range nc.MemberClasses {
			for _, ct := range mc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
				ct.className = ""
				ct.classType = ClassTypeMember(ClassTypeNode, "")
			}
		}
	}
	for _, ic := range cfg.InterfaceClasses {
		for _, ct := range ic.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
			ct.className = ic.Name
			ct.classType = ClassTypeInterface
		}
		for _, nc := range ic.NeighborClasses {
			for _, ct := range nc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
				ct.className = ""
				ct.classType = ClassTypeNeighbor("")
			}
		}
		for _, mc := range ic.MemberClasses {
			for _, ct := range mc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
				ct.className = ""
				ct.classType = ClassTypeMember(ClassTypeInterface, "")
			}
		}
	}
	for _, cc := range cfg.ConnectionClasses {
		for _, ct := range cc.ConfigTemplates {
			if err := initConfigTemplate(cfg, ct); err != nil {
				return nil, err
			}
			ct.className = cc.Name
			ct.classType = ClassTypeConnection
		}
		for _, nc := range cc.NeighborClasses {
			for _, ct := range nc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
				ct.className = ""
				ct.classType = ClassTypeNeighbor("")
			}
		}
		for _, mc := range cc.MemberClasses {
			for _, ct := range mc.ConfigTemplates {
				if err := initConfigTemplate(cfg, ct); err != nil {
					return nil, err
				}
				ct.className = ""
				ct.classType = ClassTypeMember(ClassTypeConnection, "")
			}
		}
	}
	return cfg, nil
}
