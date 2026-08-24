package types

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/goccy/go-yaml"
	// "gopkg.in/yaml.v2"
	// "github.com/spf13/viper"
)

// Defaults for settings whose absence must not be read as the zero value.
//
// LoadConfig starts from defaultGlobalSettings rather than an empty struct, and
// the YAML decoder leaves alone whatever the document does not mention, so a
// setting the topology never writes keeps the value below. Keep them together:
// a default buried in the code that reads it cannot be found by someone asking
// what happens when they write nothing.
//
// (DefaultMaxAddressCount lives in pkg/model/address.go, where it is applied to
// a zero value rather than pre-set here.)
const (
	// DefaultAggregateCrossingLinks replaces a shared segment that reaches
	// across machines with one bridge per machine, linked to each other, so that
	// the segment costs one link leaving a machine instead of one per member on
	// the far side. On by default: the alternative is for the author to work the
	// same replacement out by hand for every such segment.
	DefaultAggregateCrossingLinks = true

	// DefaultProvide is how a file reaches its node when the file definition
	// says nothing. Mount is the default because it is the simpler of the two -
	// the platform is shown the file and that is all - and because the files
	// topologies generate are mostly read while the container boots, which only
	// mount can serve.
	DefaultProvide = ProvideMount
)

// The ways a generated file can reach the node's container. See
// FileDefinition.Provide for what each one means.
const (
	ProvideMount string = "mount"
	ProvideCopy  string = "copy"
)

// StagingDirName is where a file provided by copy waits, both in the output
// (r1/staging/etc/motd) and in the container (/staging/etc/motd, mounted read
// only). A copy is made from there to the file's own path once the container is
// up.
//
// The name says what the files are rather than what is done to them: on the
// host the copy has not happened yet, inside the container it already has, and
// a name like to_copy would be wrong on one side or the other. The directory
// does not exist in any image, so mounting it hides nothing.
const StagingDirName = "staging"

func defaultGlobalSettings() GlobalSettings {
	return GlobalSettings{
		AggregateCrossingLinks: DefaultAggregateCrossingLinks,
	}
}

// HookConfigNames are the group names dot2net itself owns: the only bare names
// a topology may write blocks into without declaring the sorter that gathers
// them.
//
// This is where the line between a topology and a module is drawn. A topology
// says what it wants by writing a block into one of these groups; whichever
// module is bringing the lab up gathers that group into its own files, and adds
// what it has to do itself. Every other name belongs to whoever defined it: a
// module's own groups are its business, and a topology writing into one would
// be reaching into a module's insides, which is how the ways the two can talk
// to each other multiply until nobody can say what they are.
//
// The table decides nothing about order or merging - those are the ordinary
// rules for a sorted column. It only says which names are dot2net's.
var HookConfigNames = map[string]bool{
	// startup: commands to run once the node is up. containerlab puts them in
	// exec:, TiNET in cmds:, Kathara in <device>.startup.
	"startup": true,
	// teardown: commands to run in the node while it is still up, before the
	// lab is destroyed. Dumping state, flushing what a program buffers, putting
	// a mounted file's permissions back. The entry script runs them.
	"teardown": true,
	// The four below run on the machine rather than in a node, one for each of
	// the entry script's own commands, and each named after the command it hangs
	// off so that when it runs needs no looking up. What belongs here is what the
	// lab needs of the machine and no platform can do from inside: a bridge, a
	// host interface enslaved to one, a capture taken beside the lab.
	//
	// The platform's own command is a block of the same column, carrying the
	// hook's name as its anchor, so a block says which side of it to run on with
	// after: or before: rather than with a number.
	//
	// worker, because that is what dot2net already calls the machine a lab is
	// deployed onto - see WorkerGroupClassName. A topology that declares no
	// worker groups still has one machine, and these still run on it.
	"worker_deploy":  true,
	"worker_exec":    true,
	"worker_collect": true,
	"worker_destroy": true,
}

// Where a block of a machine-side hook sits is a number, and the platform's own
// command is the origin: below it runs before the platform is asked to act,
// above it runs after.
//
// A module's part is the ground the platform command stands on - the bridge has
// to be there before containerlab will deploy - so it is laid first and, where
// the command releases it rather than uses it, taken up last. These are what a
// module writes on its own blocks; a topology says which side it wants with
// after: or before: and never needs a number.
const (
	PlatformCommandPriority = 0
	ModuleHookPriority      = -100
	ModuleUndoPriority      = 100
)

// MachineHookOrder is the machine-side hooks in the order they are declared
// above, so that a module registering gathering points and the script that
// reads them agree without either listing the names again.
var MachineHookOrder = []string{"worker_deploy", "worker_exec", "worker_collect", "worker_destroy"}

// HookSorter is the gathering point for one hook in one platform's files.
//
// It collects two groups. The bare name is dot2net's, written into by the
// topology and by any module with something to say to every platform; the
// prefixed one is this module's own, and nothing else gathers it - which is
// what keeps containerlab's bridge command out of TiNET's script without any
// block having to say so.
//
// The name is the module's to read: the script embeds {{ .self_clab_startup }}
// and lists it in depends:.
func HookSorter(prefix, hook string) *ConfigTemplate {
	return &ConfigTemplate{
		Name:       prefix + "_" + hook,
		Style:      ConfigTemplateStyleSort,
		SortGroups: []string{hook, prefix + "/" + hook},
	}
}

// MachineHookSlots are the gathering points an entry script needs, one per
// machine-side hook, and the names to embed them by.
func (cfg *Config) MachineHookSlots(prefix string) ([]*ConfigTemplate, []string) {
	cts := make([]*ConfigTemplate, 0, len(MachineHookOrder))
	names := make([]string, 0, len(MachineHookOrder))
	for _, hook := range MachineHookOrder {
		ct := HookSorter(prefix, hook)
		cts = append(cts, ct)
		names = append(names, ct.Name)
	}
	return cts, names
}

// DeclareParamMadeBy says that what this parameter names is put in place by the
// block carrying the given anchor, so a block reading the name has to come
// after it. DeclareParamGoneBy is the other end: the block that takes the thing
// away, which a reader has to come before.
//
// A module publishing the name of something the platform makes says so here,
// and a block that reads the name at the wrong moment is reported while
// generating rather than failing on the machine with an error naming nothing
// the topology wrote.
//
// Both are read within a column and mean nothing outside it: the block that
// makes a bridge is in the deploy column and the one that removes it is in the
// destroy column, so each check finds at most one of them and the other says
// nothing. That is why there is no rule here about the two being mirrors -
// a module states each end where that end happens.
func (cfg *Config) DeclareParamMadeBy(name, anchor string) {
	if cfg.paramMadeBy == nil {
		cfg.paramMadeBy = map[string]string{}
	}
	cfg.paramMadeBy[name] = anchor
}

func (cfg *Config) DeclareParamGoneBy(name, anchor string) {
	if cfg.paramGoneBy == nil {
		cfg.paramGoneBy = map[string]string{}
	}
	cfg.paramGoneBy[name] = anchor
}

// ParamMadeBy and ParamGoneBy report the anchor a parameter's life hangs off,
// for the check that runs once a column is in order.
func (cfg *Config) ParamMadeBy(name string) (string, bool) {
	anchor, ok := cfg.paramMadeBy[name]
	return anchor, ok
}

func (cfg *Config) ParamGoneBy(name string) (string, bool) {
	anchor, ok := cfg.paramGoneBy[name]
	return anchor, ok
}

// ParamRefs is every parameter this template reads, with the prefix that says
// which object it is read from taken off - opp_ for the far end of a link,
// node_ for the node an interface sits on - so that what is named is left.
func (ct *ConfigTemplate) ParamRefs() []string {
	return ct.paramRefs
}

// setParamRefs works ParamRefs out once, while the template is being parsed.
func (ct *ConfigTemplate) setParamRefs() {
	if ct.ParsedTemplate == nil {
		return
	}
	refs := templateFields(ct.ParsedTemplate)
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		name := ref
		for _, prefix := range ReservedPrefixes() {
			if strings.HasPrefix(name, prefix) {
				name = strings.TrimPrefix(name, prefix)
				break
			}
		}
		out = append(out, name)
	}
	ct.paramRefs = out
}

// templateFields is every {{ .name }} a template reads. Taken from the parse
// tree rather than the text, so that spacing and quoting cannot hide one.
func templateFields(tpl *template.Template) []string {
	seen := map[string]bool{}
	var names []string
	var walk func(parse.Node)
	walk = func(n parse.Node) {
		switch v := n.(type) {
		case nil:
			return
		case *parse.ListNode:
			if v == nil {
				return
			}
			for _, c := range v.Nodes {
				walk(c)
			}
		case *parse.ActionNode:
			walk(v.Pipe)
		case *parse.PipeNode:
			if v == nil {
				return
			}
			for _, c := range v.Cmds {
				walk(c)
			}
		case *parse.CommandNode:
			for _, a := range v.Args {
				walk(a)
			}
		case *parse.IfNode:
			walk(v.Pipe)
			walk(v.List)
			walk(v.ElseList)
		case *parse.RangeNode:
			walk(v.Pipe)
			walk(v.List)
			walk(v.ElseList)
		case *parse.WithNode:
			walk(v.Pipe)
			walk(v.List)
			walk(v.ElseList)
		case *parse.FieldNode:
			for _, ident := range v.Ident {
				if !seen[ident] {
					seen[ident] = true
					names = append(names, ident)
				}
			}
		}
	}
	for _, t := range tpl.Templates() {
		if t.Tree != nil {
			walk(t.Tree.Root)
		}
	}
	return names
}

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
const ClassTypeValueHeader string = "value"

// WorkerGroupClassName marks a group that stands for one placement unit: a
// machine that containers are deployed onto, as opposed to a group that exists
// to share parameters (an AS, an OSPF area). A node belongs to several groups
// at once, so the two uses have to be told apart by name.
//
// dot2net owns the word rather than letting each platform module pick its own.
// Fourteen of the bundled examples load containerlab and TiNET together and
// emit both topo.yaml and spec.yaml from one topology; a module-specific name
// like clabHost written in the DOT file would tie that file to one platform.
// The vocabulary is shared and only its interpretation belongs to the module.
// See doc/active/CLASS_SEMANTICS.md ch.3 for the naming survey.
const WorkerGroupClassName string = "worker"

// Deployment forms a class can ask for through its Deploy field. They all
// answer one question - what is this object materialised as - and the set a
// class may choose from is the set of forms that object can take. That is why
// the sets differ: a node has two ways of being put in place by the platform, a
// link has one, and only a wire has the option of being built by the
// configuration that runs inside the nodes rather than by the platform.
const (
	DeployContainer string = "container" // a container of its own (a node's fallback)
	DeployPlatform  string = "platform"  // a facility the platform provides itself
	DeployLink      string = "link"      // wiring the platform lays: a link, or an end of one
	DeployLogical   string = "logical"   // a logical device or link the generated configuration builds
	DeployNone      string = "none"      // nothing is materialised; parameters only
)

// A node is materialised by the platform or not at all. It has no DeployLogical
// because configuration runs inside a node, and there is nothing inside a node
// for it to build the node from.
var nodeDeployForms = []string{DeployContainer, DeployPlatform, DeployNone}

// An interface or a connection has one platform form, because the only way the
// platform puts wiring in place is by laying a link. DeployLogical covers what
// the configuration builds instead: a bridge, a dummy, a VRF, a GRE or VXLAN
// tunnel. See doc/ROADMAP.md TODO 85 for why the value is named after the shape
// it takes rather than after what builds it.
var wiringDeployForms = []string{DeployLink, DeployLogical, DeployNone}

// deployClaim validates one class's deployment form and returns what it claims,
// or "" when it claims nothing. Validation lives here so that a typo is reported
// against the class that contains it.
//
// virtual no longer names a deployment form. It says the object's own
// configuration is not written, which is a different question from what puts the
// object in place, and the two are set independently. A class written for v0.7,
// where one flag answered both, is rejected rather than guessed at.
//
// That rejection is a migration aid, not part of the design: it exists so that a
// v0.7 topology stops instead of coming up in a shape nobody asked for. Remove it
// in 0.9.0 - after which virtual on its own simply withholds the configuration.
func deployClaimOf(kind, className, deploy string, virtual bool, forms []string) (string, error) {
	if deploy != "" {
		known := false
		for _, form := range forms {
			if deploy == form {
				known = true
				break
			}
		}
		if !known {
			return "", fmt.Errorf("%s %s: unknown deploy value %q (expected %s)",
				kind, className, deploy, strings.Join(forms, ", "))
		}
	}
	if virtual && deploy == "" {
		return "", fmt.Errorf(
			"%s %s: virtual: true no longer says the object is not deployed, only that its own "+
				"configuration is not written. Write deploy: %s for what virtual meant in v0.7, or "+
				"name the form it takes (%s) if the configuration really is all you meant to suppress",
			kind, className, DeployNone, strings.Join(forms, ", "))
	}
	return deploy, nil
}

// Default format names
const DefaultFormatPhaseFormatName = "DefaultFormatPhaseFormat"
const DefaultMergePhaseFormatName = "DefaultMergePhaseFormat"

// Default format definitions
var DefaultFormatPhaseFormat = &FormatStyle{
	Name:                DefaultFormatPhaseFormatName,
	FormatLineSeparator: "\n",
}

var DefaultMergePhaseFormat = &FormatStyle{
	Name:                DefaultMergePhaseFormatName,
	MergeBlockSeparator: "\n",
}

func ClassTypeNeighbor(layer string) string {
	return ClassTypeNeighborHeader + "_" + layer
}

func ClassTypeMember(classType string, className string) string {
	return ClassTypeMemberHeader + "_" + classType + "_" + className
}

// ClassTypeValue returns the class type string for a Value with given param_rule name
func ClassTypeValue(paramRuleName string) string {
	return ClassTypeValueHeader + "_" + paramRuleName
}

// ClassAll and ClassDefault are the legacy "magic" class names: defining a class
// with one of these names silently changed how it was applied. They are deprecated
// in favour of the class_policy section, which states the same thing explicitly and
// frees up "all" and "default" as ordinary class names. They still work, with a
// warning, so existing configurations keep running.
const ClassAll string = "all"         // all objects
const ClassDefault string = "default" // all empty objects

// ClassPolicyEntry declares which classes apply implicitly for one object type.
//
//   - Base classes apply to every object of that type. They sit in the base tier,
//     so any class the user names on the object overrides them (see ClassTier* in
//     object.go).
//   - Default classes apply only to objects that carry no class label at all. They
//     stand in for user-written classes and share the user tier.
type ClassPolicyEntry struct {
	Base    []string `yaml:"base,flow" mapstructure:"base,flow"`
	Default []string `yaml:"default,flow" mapstructure:"default,flow"`
}

// ClassPolicy replaces the legacy magic class names "all" and "default".
type ClassPolicy struct {
	Node       ClassPolicyEntry `yaml:"node" mapstructure:"node"`
	Interface  ClassPolicyEntry `yaml:"interface" mapstructure:"interface"`
	Connection ClassPolicyEntry `yaml:"connection" mapstructure:"connection"`
	Group      ClassPolicyEntry `yaml:"group" mapstructure:"group"`
	Segment    ClassPolicyEntry `yaml:"segment" mapstructure:"segment"`
}

// EmptySeparator is the explicit "no separator" value for FormatStyle separators.
// An empty string means "not specified" and falls back to the default, so a
// separator that really is empty has to be spelled out.
const EmptySeparator string = "#NONE#"

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

// config elements

type Config struct {
	Name    string   `yaml:"name" mapstructure:"name"`
	Modules []string `yaml:"module" mapstructure:"module"`
	// ModuleConfig holds a section per module, for settings that belong to one
	// platform rather than to the topology. Kept apart from GlobalSettings
	// because what a module offers is its own: containerlab's management
	// network and Kathara's bridged devices sound alike and are not the same
	// thing, so a shared key would be wrong.
	ModuleConfig    map[string]map[string]any `yaml:"module_config" mapstructure:"module_config"`
	ClassPolicy     ClassPolicy               `yaml:"class_policy" mapstructure:"class_policy"`
	GlobalSettings  GlobalSettings            `yaml:"global" mapstructure:"global"`
	FileDefinitions []*FileDefinition         `yaml:"file" mapstructure:"file"`
	FormatStyles    []*FormatStyle            `yaml:"format,flow" mapstructure:"format,flow"`
	Layers          []*Layer                  `yaml:"layer" mapstructure:"layer"`
	ManagementLayer ManagementLayer           `yaml:"mgmt_layer" mapstructure:"mgmt_layer"`
	ParameterRules  []*ParameterRule          `yaml:"param_rule,flow" mapstructure:"param_rule,flow"`

	NetworkClasses    []*NetworkClass    `yaml:"networkclass,flow" mapstructure:"network,flow"`
	NodeClasses       []*NodeClass       `yaml:"nodeclass,flow" mapstructure:"nodes,flow"`
	InterfaceClasses  []*InterfaceClass  `yaml:"interfaceclass,flow" mapstructure:"interfaces,flow"`
	ConnectionClasses []*ConnectionClass `yaml:"connectionclass,flow" mapstructure:"connections,flow"`
	GroupClasses      []*GroupClass      `yaml:"groupclass,flow" mapstructure:"group,flow"`
	SegmentClasses    []*SegmentClass    `yaml:"segmentclass,flow" mapstructure:"segments,flow"`

	// registeringModule is true while a module's UpdateConfig runs, so that the
	// classes it adds are recorded as module-provided.
	registeringModule bool

	// paramMadeBy and paramGoneBy record, for a parameter naming something that
	// comes into being partway through a deployment, the anchor of the block
	// that puts it there and of the one that takes it away. A block reading the
	// name has to sit between them, and each end is only looked for in the
	// column it belongs to.
	paramMadeBy map[string]string
	paramGoneBy map[string]string

	// topologySortGroups are the groups a sorter the topology itself declared
	// gathers. A topology may write into these and into the names dot2net owns;
	// anything else is a module's own group.
	topologySortGroups map[string]bool

	// renamedHooks are the hook names a topology wrote as name: and that were
	// read as group:, so that the deprecation is reported once per name rather
	// than once per class.
	renamedHooks map[string]bool

	fileDefinitionMap map[string]*FileDefinition
	formatStyleMap    map[string]*FormatStyle
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

	// resolvedClassPolicy is class_policy after the legacy "all"/"default" fallback
	// has been applied, keyed by ClassType*. Built once in LoadConfig.
	resolvedClassPolicy map[string]ClassPolicyEntry

	LoadedModules              []Module           // reference to loaded modules, internal
	SorterConfigTemplateGroups mapset.Set[string] // list of sort-style config template groups

	// configTemplateAnchors are the labels any config template carries, used to
	// tell an after:/before: anchor apart from a typo. A label that exists but
	// reaches no column is not an error - a block cannot know which objects
	// its anchor is generated for - so only a label written nowhere is.
	configTemplateAnchors map[string]bool

	// groupContributions are the config templates that write blocks into a
	// group, kept so that the group names can be checked against the sorters
	// once every template is loaded. A contribution to a group no sorter
	// collects is generated and then dropped, so a mistyped name would
	// otherwise take the block out of the output without saying anything.
	groupContributions []*ConfigTemplate

	// nodeNamePrefix separates one deployment of a topology from another. It is
	// set from the command line rather than read from YAML, because it varies
	// per run and not per topology: the same inputs generated twice under two
	// names give two labs that can be up at once.
	//
	// A platform that already namespaces its containers by the lab does not
	// need it - containerlab labels each container with the lab it belongs to -
	// but TiNET names a container after the node and nothing else, and Kathara
	// builds the name out of the device. For those, the node's name is the only
	// place a lab can be told apart, so the prefix goes there. It reaches the
	// platform's own file and nothing else: the model's node is still r1, and so
	// are the directory its files are written to and the hostname inside them.
	nodeNamePrefix string
}

// labNamePattern is what all three platforms can carry. A lab name becomes part
// of a container's name on every one of them, and docker's own rule is the
// narrowest thing they agree on. Kathara narrows it further for a device name,
// which is checked where that name is built.
var labNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// ValidateLabName rejects a name a platform could not use. Caught here rather
// than at deploy time, where it appears as a container runtime error with no
// mention of dot2net.
func ValidateLabName(name string) error {
	if len(name) > 63 {
		return fmt.Errorf("lab name %q is longer than 63 characters", name)
	}
	if !labNamePattern.MatchString(name) {
		return fmt.Errorf(
			"lab name %q cannot be used: it becomes part of a container's name, "+
				"which must start with a letter or digit and hold only letters, "+
				"digits, and the characters _ . -", name)
	}
	return nil
}

// SetLabName gives this run a lab name of its own, replacing the topology's.
// The nodes are namespaced to match, so that two labs generated from one
// topology can be deployed side by side.
func (cfg *Config) SetLabName(name string) {
	cfg.Name = name
	cfg.nodeNamePrefix = name + "_"
}

// LabNamePrefix is what a platform module puts in front of the name it gives a
// lab, when that name is not already this run's own. A lab written per machine
// is named after the machine, so it needs the prefix to tell one run from
// another; a lab written whole is named after the run already, and prefixing it
// again would say the name twice.
func (cfg *Config) LabNamePrefix(perMachine bool) string {
	if !perMachine {
		return ""
	}
	return cfg.nodeNamePrefix
}

// NodeNamePrefix is what a platform module puts in front of a node's name in
// its own file. Empty unless this run was given a lab name, so a topology
// generated the usual way is generated exactly as before.
func (cfg *Config) NodeNamePrefix() string {
	return cfg.nodeNamePrefix
}

func (cfg *Config) FileDefinitionByName(name string) (*FileDefinition, bool) {
	filedef, ok := cfg.fileDefinitionMap[name]
	return filedef, ok
}

func (cfg *Config) FormatStyleByName(name string) (*FormatStyle, bool) {
	filefmt, ok := cfg.formatStyleMap[name]
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

// getValidClasses builds the ordered class list for one object. base and def come
// from the class_policy section (or from the legacy magic names, see resolveClassPolicy).
func (cfg *Config) getValidClasses(given []string, base []string, def []string) *ParsedLabels {
	pl := cfg.classifyLabels(given)
	classLabels := pl.classLabels

	cnt := len(classLabels) + len(base)
	if len(classLabels) == 0 {
		cnt = cnt + len(def)
	}
	classes := make([]string, 0, cnt)

	// Strongest first: user-written classes, then the base ("all") class. Module
	// labels are appended after this by SetLabels, which makes them the weakest.
	// Resolution walks this slice and keeps the first value it sees, so the order
	// here *is* the precedence order (see ClassTier* in object.go).
	if len(classLabels) == 0 {
		for _, name := range def {
			classes = append(classes, name)
			pl.setClassTier(name, ClassTierUser)
		}
	} else {
		classes = append(classes, classLabels...)
		for _, name := range classLabels {
			pl.setClassTier(name, ClassTierUser)
		}
	}
	for _, name := range base {
		classes = append(classes, name)
		pl.setClassTier(name, ClassTierBase)
	}

	pl.classLabels = classes
	return pl
}

func (cfg *Config) GetValidNodeClasses(given []string) *ParsedLabels {
	p := cfg.resolvedClassPolicy[ClassTypeNode]
	return cfg.getValidClasses(given, p.Base, p.Default)
}

// deployClaim returns the deployment form this class asks for, or "" when it
// asks for nothing.
func (nc *NodeClass) deployClaim() (string, error) {
	return deployClaimOf("nodeclass", nc.Name, nc.Deploy, nc.Virtual, nodeDeployForms)
}

func (ic *InterfaceClass) deployClaim() (string, error) {
	return deployClaimOf("interfaceclass", ic.Name, ic.Deploy, ic.Virtual, wiringDeployForms)
}

func (cc *ConnectionClass) deployClaim() (string, error) {
	return deployClaimOf("connectionclass", cc.Name, cc.Deploy, cc.Virtual, wiringDeployForms)
}

// resolveDeploy weighs the deployment claims of the classes an object carries.
// Claims are weighed by class tier, so a default coming from a module or from
// the base class loses to anything the user wrote; two classes of the same tier
// asking for different forms is a conflict only the user can resolve. An object
// no class speaks for takes the given fallback.
//
// Tiers are compared rather than relying on the order of ClassLabels, because
// classes pulled in through use: are appended after the ones that named them
// and would otherwise be weighed as if they came last.
//
// claimOf reports the form the named class asks for; a name it does not know is
// answered with ok false, since the labels of an object name classes of more
// than one kind.
func resolveDeploy(lo LabelOwner, fallback string, claimOf func(string) (string, bool, error)) (string, error) {
	deploy := fallback
	claimed := ClassTierModule - 1
	for _, name := range lo.ClassLabels() {
		claim, ok, err := claimOf(name)
		if err != nil {
			return "", err
		}
		if !ok || claim == "" {
			continue
		}
		switch tier := lo.ClassTier(name); {
		case tier > claimed:
			deploy, claimed = claim, tier
		case tier == claimed && claim != deploy:
			return "", fmt.Errorf("%s: classes of the same precedence ask to deploy it as %s and as %s",
				lo.StringForMessage(), deploy, claim)
		}
	}
	return deploy, nil
}

// ResolveDeploy determines what a node is materialised as. A node no class
// speaks for is a DeployContainer.
//
// The result comes from the labels alone so that a module can ask before
// SetClasses has run: ClassifyObjects needs it to pick which node class to
// attach.
func (cfg *Config) ResolveDeploy(n *Node) (string, error) {
	return resolveDeploy(n, DeployContainer, func(name string) (string, bool, error) {
		nc, ok := cfg.NodeClassByName(name)
		if !ok {
			return "", false, nil
		}
		claim, err := nc.deployClaim()
		return claim, true, err
	})
}

// ResolveConnectionDeploy determines what a connection is materialised as. An
// edge in the graph is a wire the platform lays unless a class says otherwise.
func (cfg *Config) ResolveConnectionDeploy(conn *Connection) (string, error) {
	return resolveDeploy(conn, DeployLink, func(name string) (string, bool, error) {
		cc, ok := cfg.ConnectionClassByName(name)
		if !ok {
			return "", false, nil
		}
		claim, err := cc.deployClaim()
		return claim, true, err
	})
}

// ResolveInterfaceDeployClaim reports the form the interface's own classes ask
// for, or "" when they ask for nothing.
//
// An interface that sits on a connection takes the connection's form and is not
// asked: the two describe one event seen from either side, since the platform
// that lays a wire creates both of its ends and a tunnel the configuration
// builds has ends the configuration builds too. Writing the form on an
// interface class is therefore for interfaces that have no connection - the
// management interface the platform supplies on its own, and the devices a node
// builds for itself. Whether the claim agrees with the connection is checked
// where both are known.
func (cfg *Config) ResolveInterfaceDeployClaim(iface *Interface) (string, error) {
	return resolveDeploy(iface, "", func(name string) (string, bool, error) {
		ic, ok := cfg.InterfaceClassByName(name)
		if !ok {
			return "", false, nil
		}
		claim, err := ic.deployClaim()
		return claim, true, err
	})
}

// IsSwitchNode reports whether the node stands for a shared L2 domain that the
// platform provides itself. Modules call this while classifying objects, before
// the classes are checked, so a broken deploy value is reported here as "not a
// switch" and surfaces from SetClasses instead.
func (cfg *Config) IsSwitchNode(n *Node) bool {
	deploy, err := cfg.ResolveDeploy(n)
	return err == nil && deploy == DeployPlatform
}

func (cfg *Config) GetValidInterfaceClasses(given []string) *ParsedLabels {
	p := cfg.resolvedClassPolicy[ClassTypeInterface]
	return cfg.getValidClasses(given, p.Base, p.Default)
}

func (cfg *Config) GetValidConnectionClasses(given []string) *ParsedLabels {
	p := cfg.resolvedClassPolicy[ClassTypeConnection]
	return cfg.getValidClasses(given, p.Base, p.Default)
}

func (cfg *Config) GetValidGroupClasses(given []string) *ParsedLabels {
	p := cfg.resolvedClassPolicy[ClassTypeGroup]
	return cfg.getValidClasses(given, p.Base, p.Default)
}

func (cfg *Config) GetValidSegmentClasses(given []string) *ParsedLabels {
	p := cfg.resolvedClassPolicy[ClassTypeSegment]
	return cfg.getValidClasses(given, p.Base, p.Default)
}

func (cfg *Config) AddFormatStyle(fmtstyle *FormatStyle) {
	cfg.FormatStyles = append(cfg.FormatStyles, fmtstyle)
	cfg.formatStyleMap[fmtstyle.Name] = fmtstyle
}

func (cfg *Config) AddFileDefinition(filedef *FileDefinition) {
	cfg.FileDefinitions = append(cfg.FileDefinitions, filedef)
	cfg.fileDefinitionMap[filedef.Name] = filedef
}

// markModuleTemplates records that these templates came from a module, which is
// what lets a hook name carry a module's part ahead of the topology's.
func markModuleTemplates(cfg *Config, cts []*ConfigTemplate) {
	if !cfg.registeringModule {
		return
	}
	for _, ct := range cts {
		ct.ModuleProvided = true
	}
}

func (cfg *Config) AddNetworkClass(nc *NetworkClass) {
	markModuleTemplates(cfg, nc.ConfigTemplates)
	cfg.NetworkClasses = append(cfg.NetworkClasses, nc)
}

// LoadModuleConfig runs a module's registration, recording that whatever it
// adds came from a module. Knowing the origin is what lets a class keep the
// weakest tier when a user's class pulls it in through use:.
func (cfg *Config) LoadModuleConfig(m Module) error {
	cfg.registeringModule = true
	defer func() { cfg.registeringModule = false }()
	return m.UpdateConfig(cfg)
}

func (cfg *Config) AddNodeClass(nc *NodeClass) {
	markModuleTemplates(cfg, nc.ConfigTemplates)
	nc.ModuleProvided = cfg.registeringModule
	cfg.NodeClasses = append(cfg.NodeClasses, nc)
	cfg.nodeClassMap[nc.Name] = nc
}

func (cfg *Config) AddInterfaceClass(nc *InterfaceClass) {
	markModuleTemplates(cfg, nc.ConfigTemplates)
	nc.ModuleProvided = cfg.registeringModule
	cfg.InterfaceClasses = append(cfg.InterfaceClasses, nc)
	cfg.interfaceClassMap[nc.Name] = nc
}

func (cfg *Config) AddConnectionClass(nc *ConnectionClass) {
	markModuleTemplates(cfg, nc.ConfigTemplates)
	nc.ModuleProvided = cfg.registeringModule
	cfg.ConnectionClasses = append(cfg.ConnectionClasses, nc)
	cfg.connectionClassMap[nc.Name] = nc
}

func (cfg *Config) AddGroupClass(gc *GroupClass) {
	markModuleTemplates(cfg, gc.ConfigTemplates)
	gc.ModuleProvided = cfg.registeringModule
	cfg.GroupClasses = append(cfg.GroupClasses, gc)
	cfg.groupClassMap[gc.Name] = gc
}

func (cfg *Config) AddParameterRule(pr *ParameterRule) {
	cfg.ParameterRules = append(cfg.ParameterRules, pr)
	cfg.parameterRuleMap[pr.Name] = pr
}

// HasManagementLayer reports whether a management layer is configured. Note the
// heuristic: a management layer is considered present only when its address
// range is set, so a mgmt_layer block that omits "range" is silently treated as
// absent. (Distinguishing "declared but incomplete" from "not declared" would
// require making ManagementLayer a pointer.)
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
	// MaxAddressCount caps how many addresses/prefixes are enumerated when an
	// address pool is expanded fully (loopback / segment / reservation handling).
	// It bounds internal count-up so an oversized (e.g. IPv6) pool cannot blow up
	// memory. 0 or negative uses DefaultMaxAddressCount.
	MaxAddressCount int `yaml:"max_address_count" mapstructure:"max_address_count"`
	// IgnoreUndefinedClass controls how a class label that does not match any
	// defined class is handled. false (default): it is an error. true: it is
	// silently skipped (useful when e.g. a subgraph label is meant for display
	// rather than as a group class).
	IgnoreUndefinedClass bool `yaml:"ignore_undefined_class" mapstructure:"ignore_undefined_class"`
	// SplitModuleOutput puts each module's own files in a directory named after
	// it - containerlab/topo.yaml, tinet/spec.yaml - instead of side by side at
	// the output root. Files the topology defines stay where they are: they are
	// often read by more than one platform, and splitting them would mean
	// copying them.
	//
	// Off by default. It is for a lab whose output is large enough that the
	// platforms get in each other's way, or where two of them would otherwise
	// want the same file name.
	SplitModuleOutput bool `yaml:"split_module_output" mapstructure:"split_module_output"`
	// AggregateCrossingLinks cuts the number of links that leave a machine.
	//
	// A shared segment with members on several machines has to reach all of
	// them. Left alone, every member on a machine other than the segment's own
	// costs a link that leaves a machine - and a link leaving a machine costs a
	// VLAN from a finite pool. Replacing the segment with one bridge per machine,
	// linked to each other, brings that down to one link per pair of machines
	// however many members there are.
	//
	// True by default. Set it to false for a platform that stretches a segment
	// across machines itself, or to write the bridges out by hand.
	AggregateCrossingLinks bool `yaml:"aggregate_crossing_links" mapstructure:"aggregate_crossing_links"`
	// OutputGroupClass names the group class that splits the output directory.
	//
	// The value is an ordinary group class of the topology's own choosing, not a
	// reserved word: this setting says nothing about what the class means, only
	// which one the layout follows. Splitting by AS is as valid as splitting by
	// host, and a topology whose groups are placement units still has to point
	// this at them explicitly.
	//
	// When set, every group carrying that class gets a subdirectory, and all
	// files belonging to it - its own group-scope files and the files of its
	// member nodes - are written below that subdirectory:
	//
	//	clabhost1/topo.yaml, clabhost1/r1/frr.conf, clabhost1/r2/frr.conf
	//
	// This is a single global axis rather than a per-file setting on purpose:
	// packaging a host means archiving its directory, so every file of a node
	// has to land under the same directory. Nodes are commonly in several
	// groups at once (an AS and a host, say), and only this class decides the
	// directory; a node in two groups of this class is an error.
	//
	// Empty (default) keeps the flat layout: node directories at the top level.
	OutputGroupClass string `yaml:"output_group_class" mapstructure:"output_group_class"`
}

type FileDefinition struct {
	// Name is used as the filename of generated file.
	// If empty and NamePrefix/NameSuffix are specified, filename is generated as:
	//   {NamePrefix}{object_name}{NameSuffix}
	Name string `yaml:"name" mapstructure:"name"`
	// Subdir places the file in a directory below where it would otherwise go.
	// A module sets it to its own name when GlobalSettings.SplitModuleOutput is
	// on; it is not something a topology writes.
	Subdir string `yaml:"-" mapstructure:"-"`
	// NamePrefix is prepended to the object name when Name is empty.
	NamePrefix string `yaml:"name_prefix" mapstructure:"name_prefix"`
	// NameSuffix is appended to the object name when Name is empty.
	NameSuffix string `yaml:"name_suffix" mapstructure:"name_suffix"`
	// Path is the path that the generated file is placed on the node.
	// If empty, the file is generated but not placed on the node.
	Path string `yaml:"path" mapstructure:"path"`
	// Executable marks a file meant to be run. A generated script that has to
	// be chmod'ed before it works is a script that will be run wrong once.
	Executable bool `yaml:"executable" mapstructure:"executable"`
	// Provide says how the file reaches the node's container. The two ways are
	// not one better than the other:
	//
	//   - "mount" (the default): the container is shown the generated file
	//     itself. It is there before the container's first process runs, which
	//     is the only way to reach software that reads its configuration while
	//     booting. What the container writes reaches the generated file, and
	//     what the container's own startup does to ownership and permissions
	//     reaches it too.
	//   - "copy": the container gets its own copy, placed once the container is
	//     up. Nothing the container does comes back, so the generated file stays
	//     as it was generated - which is what a file the software rewrites in
	//     the course of a run needs. It cannot serve a file read while booting.
	//
	// Empty means "mount".
	Provide string `yaml:"provide" mapstructure:"provide"`
	// Format is used to determine the way to format lines in generated config text.
	Format  string   `yaml:"format" mapstructure:"format"`
	Formats []string `yaml:"formats,flow" mapstructure:"formats,flow"`
	// Scope specifies the scope of file creation.
	// Available values:
	//   - "network": File is created at network level (root directory)
	//   - "group": File is created for each group (group_name/file_name)
	//   - "node": File is created for each node (node_name/file_name)
	//   - "" (empty): Defaults to "node" scope for backward compatibility
	// Examples:
	//   - Scope = "network": Creates "spec.yaml", "topo.yaml" at root
	//   - Scope = "group": Creates "clabhost1/topo.yaml", "clabhost2/topo.yaml"
	//   - Scope = "node" or "": Creates "r1/frr.conf", "r2/frr.conf", etc.
	Scope string `yaml:"scope" mapstructure:"scope"`
	// Output specifies where the file is placed in the output directory.
	// Available values:
	//   - "root": File is placed at root directory
	//   - "group": File is placed in group subdirectory (group_name/file_name)
	//   - "node": File is placed in node subdirectory (node_name/file_name)
	//   - "" (empty): Defaults based on Scope (network->root, group->group, node->node)
	// This is useful for Kathara-style startup files that need to be at root
	// but are generated per-node (e.g., r1.startup, r2.startup).
	//
	// For a node-scope file, "root" is relative to the directory of the node's
	// group when GlobalSettings.OutputGroupClass is in effect, not to the top
	// level: a host's files must all stay inside the host's directory so that
	// it can be archived as a unit. Only network-scope files, and files
	// explicitly given "root" at group scope, sit at the true top level - and
	// the latter then need NamePrefix/NameSuffix to stay distinct per group.
	Output string `yaml:"output" mapstructure:"output"`
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

// GetFileName returns the output filename for this file definition.
// If NamePrefix or NameSuffix is specified, it generates filename as:
//
//	{NamePrefix}{objectName}{NameSuffix}
//
// Otherwise, it returns Name directly.
// This allows Name to be used as an identifier for referencing,
// while NamePrefix/NameSuffix control the actual output filename.
func (fd *FileDefinition) GetFileName(objectName string) string {
	if fd.NamePrefix != "" || fd.NameSuffix != "" {
		return fd.NamePrefix + objectName + fd.NameSuffix
	}
	return fd.Name
}

// GetOutputLocation returns the effective output location.
// If Output is specified, it returns Output.
// If Output is empty, it returns the default based on Scope:
//   - "network" -> "root"
//   - "group" -> "group"
//   - "node" or "" -> "node"
//
// GetProvide returns how the file reaches its node, filling in the default.
func (fd *FileDefinition) GetProvide() string {
	if fd.Provide == "" {
		return DefaultProvide
	}
	return fd.Provide
}

// checkProvide rejects a value the config does not know, and a file that says
// how it should reach a node without saying where it goes.
func (fd *FileDefinition) checkProvide() error {
	switch fd.Provide {
	case "", ProvideMount, ProvideCopy:
	default:
		return fmt.Errorf(
			"file %s: provide is %q, but the ways a file can reach a node are %q and %q",
			fd.Name, fd.Provide, ProvideMount, ProvideCopy)
	}
	if fd.Provide != "" && fd.Path == "" {
		return fmt.Errorf(
			"file %s sets provide but no path, so there is nowhere for it to reach; "+
				"a file without path is generated and left in the output", fd.Name)
	}
	return nil
}

func (fd *FileDefinition) GetOutputLocation() string {
	if fd.Output != "" {
		return fd.Output
	}
	switch fd.Scope {
	case ClassTypeNetwork:
		return "root"
	case ClassTypeGroup:
		return ClassTypeGroup
	default:
		return ClassTypeNode
	}
}

// FormatStyle defines how to format configuration blocks
// Renamed from FileFormat to align with YAML `format:` section
type FormatStyle struct {
	Name string `yaml:"name" mapstructure:"name"`

	// Format Phase (block生成時の装飾)
	FormatLinePrefix    string `yaml:"format_lineprefix" mapstructure:"format_lineprefix"`
	FormatLineSuffix    string `yaml:"format_linesuffix" mapstructure:"format_linesuffix"`
	FormatLineSeparator string `yaml:"format_lineseparator" mapstructure:"format_lineseparator"`
	FormatBlockPrefix   string `yaml:"format_blockprefix" mapstructure:"format_blockprefix"`
	FormatBlockSuffix   string `yaml:"format_blocksuffix" mapstructure:"format_blocksuffix"`

	// Merge Phase (block結合時の処理)
	MergeBlockSeparator string `yaml:"merge_blockseparator" mapstructure:"merge_blockseparator"`
	MergeResultPrefix   string `yaml:"merge_resultprefix" mapstructure:"merge_resultprefix"`
	MergeResultSuffix   string `yaml:"merge_resultsuffix" mapstructure:"merge_resultsuffix"`
}

// Format Phase Getters
func (fs *FormatStyle) GetFormatLinePrefix() string {
	return fs.FormatLinePrefix
}

func (fs *FormatStyle) GetFormatLineSuffix() string {
	return fs.FormatLineSuffix
}

func (fs *FormatStyle) GetFormatLineSeparator() string {
	return fs.FormatLineSeparator
}

func (fs *FormatStyle) GetFormatBlockPrefix() string {
	return fs.FormatBlockPrefix
}

func (fs *FormatStyle) GetFormatBlockSuffix() string {
	return fs.FormatBlockSuffix
}

// Merge Phase Getters
func (fs *FormatStyle) GetMergeBlockSeparator() string {
	return fs.MergeBlockSeparator
}

func (fs *FormatStyle) GetMergeResultPrefix() string {
	return fs.MergeResultPrefix
}

func (fs *FormatStyle) GetMergeResultSuffix() string {
	return fs.MergeResultSuffix
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

// ParameterRuleMode constants
const (
	// ParameterRuleModeDistribute distributes one parameter per object (default, legacy behavior)
	ParameterRuleModeDistribute = "distribute"
	// ParameterRuleModeAttach attaches multiple Values to one object
	ParameterRuleModeAttach = "attach"
)

// ParameterRuleSource defines the source for generating Value lists
type ParameterRuleSource struct {
	// Type specifies the source type: range, sequence, list, file
	Type string `yaml:"type" mapstructure:"type"`
	// Start is used for range type
	Start int `yaml:"start" mapstructure:"start"`
	// End is used for range type
	End int `yaml:"end" mapstructure:"end"`
	// Values is used for list type (inline values)
	Values []map[string]interface{} `yaml:"values" mapstructure:"values"`
	// File is used for file type
	File string `yaml:"file" mapstructure:"file"`
	// Format specifies the file format: yaml, json, csv, text (default: auto-detect from extension)
	Format string `yaml:"format" mapstructure:"format"`
}

type ParameterRule struct {
	Name string `yaml:"name" mapstructure:"name"`
	// Mode: "distribute" (default) or "attach"
	// distribute: N objects get 1 parameter each (legacy behavior)
	// attach: 1 object gets N Values attached
	Mode string `yaml:"mode" mapstructure:"mode"`
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
	SourceFile string `yaml:"sourcefile" mapstructure:"sourcefile"`

	// === attach mode fields ===
	// Source defines how to generate Value list (for attach mode)
	Source *ParameterRuleSource `yaml:"source" mapstructure:"source"`
	// Generator specifies a module-provided generator (e.g., "clab.bindmounts")
	Generator string `yaml:"generator" mapstructure:"generator"`
	// ParamFormat defines how to format source values into Value parameters
	ParamFormat map[string]string `yaml:"param_format" mapstructure:"param_format"`
	// ConfigTemplates defines config blocks for Values
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
}

// GetMode returns the mode, defaulting to "distribute" if not specified
func (pr *ParameterRule) GetMode() string {
	if pr.Mode == "" {
		return ParameterRuleModeDistribute
	}
	return pr.Mode
}

// IsAttachMode returns true if this rule uses attach mode
func (pr *ParameterRule) IsAttachMode() bool {
	return pr.GetMode() == ParameterRuleModeAttach
}

// interfaces and abstracted structs for object classes

type ObjectClass interface{}

type LabelOwnerClass interface {
	GetGivenValues() map[string]string
	// ClassName is what lets a caller look the class's tier up on the object.
	// GetClasses hands back the classes in label order, which is not the order
	// of precedence once use: has appended classes to the end.
	ClassName() string
}

// ComposableClass is implemented by the class types that can pull in other
// classes of the same type through their use: list.
type ComposableClass interface {
	UsedClasses() []string
	IsModuleProvided() bool
}

func (nc *NodeClass) UsedClasses() []string  { return nc.Use }
func (nc *NodeClass) IsModuleProvided() bool { return nc.ModuleProvided }

func (ic *InterfaceClass) UsedClasses() []string  { return ic.Use }
func (ic *InterfaceClass) IsModuleProvided() bool { return ic.ModuleProvided }

func (cc *ConnectionClass) UsedClasses() []string  { return cc.Use }
func (cc *ConnectionClass) IsModuleProvided() bool { return cc.ModuleProvided }

func (gc *GroupClass) UsedClasses() []string  { return gc.Use }
func (gc *GroupClass) IsModuleProvided() bool { return gc.ModuleProvided }

func (sc *SegmentClass) UsedClasses() []string  { return sc.Use }
func (sc *SegmentClass) IsModuleProvided() bool { return sc.ModuleProvided }

// object classes

type NetworkClass struct {
	Name            string            `yaml:"name" mapstructure:"name"`
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	LabelOwnerClass
}

func (nc *NetworkClass) ClassName() string { return nc.Name }

func (nc *NetworkClass) GetGivenValues() map[string]string {
	return nc.Values
}

type NodeClass struct {
	// Use names other classes of the same type that an object carrying this
	// class also carries. It only attaches labels: no field is merged or
	// overridden, so composition follows the ordinary multi-class rules.
	Use []string `yaml:"use,flow" mapstructure:"use,flow"`
	// ModuleProvided marks a class registered by a module rather than written
	// by the user. It decides the tier a class keeps when another class pulls
	// it in through Use, so that a module's defaults still lose to the user.
	ModuleProvided bool   `yaml:"-" mapstructure:"-"`
	Name           string `yaml:"name" mapstructure:"name"`
	// Virtual withholds the configuration of the object carrying this class. It
	// says nothing about whether the object is deployed - Deploy answers that -
	// so a virtual node still gets its container unless a class says otherwise.
	// virtual: false claims nothing, which is how the boolean has always behaved.
	Virtual bool `yaml:"virtual" mapstructure:"virtual"`
	// Deploy states what a node carrying this class is materialised as:
	// DeployContainer, DeployPlatform or DeployNone.
	//
	// An empty value is not a claim. A class that stays silent leaves the choice
	// to the other classes on the node, and a node no class speaks for is a
	// container. The zero value must therefore not be normalised to
	// DeployContainer at load time: every silent class would then claim
	// "container" and collide with the one class that asked for something else.
	//
	// What each form looks like is up to the platform module: containerlab
	// writes a bridge node, TiNET a switches: entry, Kathara a collision domain.
	// dot2net itself only knows the node is not a container of its own.
	Deploy     string            `yaml:"deploy" mapstructure:"deploy"`
	IPPolicy   []string          `yaml:"policy,flow" mapstructure:"policy,flow"`
	Parameters []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values     map[string]string `yaml:"values" mapstructure:"values"`
	// Collect names files inside the node to copy out before the lab is
	// destroyed. Each is a template, so a path that follows a value stays right
	// when the value is changed. The entry script does the copying, which is
	// why a topology that collects anything needs one.
	Collect           []string          `yaml:"collect,flow" mapstructure:"collect,flow"`
	InterfaceIPPolicy []string          `yaml:"interface_policy,flow" mapstructure:"interface_policy,flow"`
	ConfigTemplates   []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	MemberClasses     []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	Prefix        string `yaml:"prefix" mapstructure:"prefix"`                           // prefix of auto-naming
	MgmtInterface string `yaml:"mgmt_interfaceclass" mapstructure:"mgmt_interfaceclass"` // InterfaceClass name for mgmt

	LabelOwnerClass
}

func (nc *NodeClass) ClassName() string { return nc.Name }

func (nc *NodeClass) GetGivenValues() map[string]string {
	return nc.Values
}

type InterfaceClass struct {
	// Use names other classes of the same type that an object carrying this
	// class also carries. It only attaches labels: no field is merged or
	// overridden, so composition follows the ordinary multi-class rules.
	Use []string `yaml:"use,flow" mapstructure:"use,flow"`
	// ModuleProvided marks a class registered by a module rather than written
	// by the user. It decides the tier a class keeps when another class pulls
	// it in through Use, so that a module's defaults still lose to the user.
	ModuleProvided bool   `yaml:"-" mapstructure:"-"`
	Name           string `yaml:"name" mapstructure:"name"`
	// Virtual withholds this interface's own configuration. It says nothing
	// about whether the interface exists: an interface can be wired and still
	// have nothing written for it.
	Virtual bool `yaml:"virtual" mapstructure:"virtual"`
	// Deploy states what an interface carrying this class is materialised as:
	// DeployLink (an end of wiring the platform lays), DeployLogical (a device
	// the generated configuration builds inside the node) or DeployNone.
	//
	// An empty value is not a claim. An interface no class speaks for takes the
	// form of its connection, so that the usual case needs nothing written.
	Deploy          string            `yaml:"deploy" mapstructure:"deploy"`
	IPPolicy        []string          `yaml:"policy,flow" mapstructure:"policy,flow"`
	Layers          []string          `yaml:"layers,flow" mapstructure:"layers,flow"` // Interface connection is limited to specified layers
	Parameters      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	NeighborClasses []*NeighborClass  `yaml:"neighbors,flow" mapstructure:"neighbors,flow"`
	MemberClasses   []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	Prefix string `yaml:"prefix" mapstructure:"prefix"` // prefix of auto-naming

	LabelOwnerClass
}

func (ic *InterfaceClass) ClassName() string { return ic.Name }

func (ic *InterfaceClass) GetGivenValues() map[string]string {
	return ic.Values
}

type ConnectionClass struct {
	// Use names other classes of the same type that an object carrying this
	// class also carries. It only attaches labels: no field is merged or
	// overridden, so composition follows the ordinary multi-class rules.
	Use []string `yaml:"use,flow" mapstructure:"use,flow"`
	// ModuleProvided marks a class registered by a module rather than written
	// by the user. It decides the tier a class keeps when another class pulls
	// it in through Use, so that a module's defaults still lose to the user.
	ModuleProvided bool   `yaml:"-" mapstructure:"-"`
	Name           string `yaml:"name" mapstructure:"name"`
	// Virtual withholds this connection's own configuration. Whether the
	// platform lays a wire for it is Deploy's answer, not this one.
	Virtual bool `yaml:"virtual" mapstructure:"virtual"`
	// Deploy states what a connection carrying this class is materialised as:
	// DeployLink (a wire the platform lays), DeployLogical (a tunnel or overlay
	// the generated configuration builds, which reaches the same two ends
	// without the platform wiring anything) or DeployNone.
	//
	// An empty value is not a claim, and a connection no class speaks for is a
	// DeployLink: an edge in the graph is a wire unless something says otherwise.
	Deploy          string            `yaml:"deploy" mapstructure:"deploy"`
	IPPolicy        []string          `yaml:"policy,flow" mapstructure:"policy,flow"`
	Layers          []string          `yaml:"layers,flow" mapstructure:"layers,flow"` // Connection is limited to specified layers
	Parameters      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	MemberClasses   []*MemberClass    `yaml:"classmembers,flow" mapstructure:"classmembers,flow"`

	// Connection naming
	Prefix string `yaml:"prefix" mapstructure:"prefix"` // prefix of connection auto-naming

	LabelOwnerClass
}

func (cc *ConnectionClass) ClassName() string { return cc.Name }

func (cc *ConnectionClass) GetGivenValues() map[string]string {
	return cc.Values
}

type GroupClass struct {
	// Use names other classes of the same type that an object carrying this
	// class also carries. It only attaches labels: no field is merged or
	// overridden, so composition follows the ordinary multi-class rules.
	Use []string `yaml:"use,flow" mapstructure:"use,flow"`
	// ModuleProvided marks a class registered by a module rather than written
	// by the user. It decides the tier a class keeps when another class pulls
	// it in through Use, so that a module's defaults still lose to the user.
	ModuleProvided bool   `yaml:"-" mapstructure:"-"`
	Name           string `yaml:"name" mapstructure:"name"`
	Virtual        bool   `yaml:"virtual" mapstructure:"virtual"`
	// BoundaryCrossingConnectionClass names a connection class attached to every
	// connection that leaves a group of this class. Whether the two ends sit in
	// the same group follows from the topology, so the alternative - annotating
	// each edge - would state twice what is already written once, and the two
	// can disagree.
	//
	// The feature knows nothing about hosts: setting it on an "as" class marks
	// the eBGP sessions just as setting it on the worker class marks the links
	// that leave a machine.
	BoundaryCrossingConnectionClass string            `yaml:"boundary_crossing_connection_class" mapstructure:"boundary_crossing_connection_class"`
	Parameters                      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	Values                          map[string]string `yaml:"values" mapstructure:"values"`
	ConfigTemplates                 []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`

	LabelOwnerClass
}

func (gc *GroupClass) ClassName() string { return gc.Name }

// IsWorkerGroup reports whether the group stands for one machine that
// containers are deployed onto. Modules ask this to decide what to split per
// machine; the concept itself carries no platform meaning.
func (cfg *Config) IsWorkerGroup(g *Group) bool {
	return g.HasClass(WorkerGroupClassName)
}

func (gc *GroupClass) GetGivenValues() map[string]string {
	return gc.Values
}

type SegmentClass struct {
	// Use names other classes of the same type that an object carrying this
	// class also carries. It only attaches labels: no field is merged or
	// overridden, so composition follows the ordinary multi-class rules.
	Use []string `yaml:"use,flow" mapstructure:"use,flow"`
	// ModuleProvided marks a class registered by a module rather than written
	// by the user. It decides the tier a class keeps when another class pulls
	// it in through Use, so that a module's defaults still lose to the user.
	ModuleProvided  bool              `yaml:"-" mapstructure:"-"`
	Name            string            `yaml:"name" mapstructure:"name"`
	Layer           string            `yaml:"layer" mapstructure:"layer"`
	Values          map[string]string `yaml:"values" mapstructure:"values"`
	Parameters      []string          `yaml:"params,flow" mapstructure:"params,flow"` // Parameter policies
	ConfigTemplates []*ConfigTemplate `yaml:"config,flow" mapstructure:"config,flow"`
	Prefix          string            `yaml:"prefix" mapstructure:"prefix"` // prefix of segment auto-naming

	LabelOwnerClass
}

func (sc *SegmentClass) ClassName() string { return sc.Name }

func (sc *SegmentClass) GetGivenValues() map[string]string {
	return sc.Values
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

// BlocksConfig defines config blocks to be merged before/after the template
type BlocksConfig struct {
	Before []string `yaml:"before" mapstructure:"before"`
	After  []string `yaml:"after" mapstructure:"after"`
}

// PlacementConfig is where a block sits among the others of its column, given
// as the anchors it comes after and the ones it comes before. Several of either
// is ordinary - a column can carry more than one thing worth sitting next to -
// and the block goes after all of the first and before all of the second.
type PlacementConfig struct {
	Before []string `yaml:"before" mapstructure:"before"`
	After  []string `yaml:"after" mapstructure:"after"`
}

// Given reports whether this config says where it goes.
func (p PlacementConfig) Given() bool {
	return len(p.Before) > 0 || len(p.After) > 0
}

// anchors is every label named, whichever side it was named on.
func (p PlacementConfig) anchors() []string {
	return append(append([]string{}, p.After...), p.Before...)
}

type ConfigTemplate struct {
	// Config block aggregation styles
	// hierarchy (default): specify child config templates in the template description as parameter
	// sort: merge child config templates of SortTarget groups into the "sort" config templates
	Style     string `yaml:"style" mapstructure:"style"`
	SortGroup string `yaml:"sort_group" mapstructure:"sort_group"`
	// SortGroups is sort_group for a sorter that gathers more than one group
	// into the same column. The blocks of every named group are put in one
	// list and ordered together, so a group is a place blocks are written
	// from rather than a section of the result.
	//
	// It exists because who may write a block and where the block ends up are
	// different questions. A group only its own writer knows the name of and a
	// group anything may write into can both feed one file, and that is not
	// expressible with a single name.
	SortGroups []string `yaml:"sort_groups" mapstructure:"sort_groups"`
	// Target file definition name
	// Config templates with file will generate a file of generated text
	File string `yaml:"file" mapstructure:"file"`
	// Name is used by parent objects to specify as childs in templates
	// Config templates with name will form a parameter that can be embeded in other hierarchy templates
	Name string `yaml:"name" mapstructure:"name"`
	// ModuleProvided marks a template registered by a module rather than
	// written by the topology. It decides the order when a module and the
	// topology both contribute to one of the hook names below: what the module
	// has to do comes first.
	ModuleProvided bool `yaml:"-" mapstructure:"-"`
	// Group is used for sort config templates
	// A sort config template will aggregate all config blocks generated in child (or grandchild) objects of the same group
	Group string `yaml:"group" mapstructure:"group"`
	// Anchor labels this block so that others can be placed relative to it.
	//
	// It is not Name. A name makes a namespace parameter, so it has to be
	// unique among everything written on the object, and two platforms writing
	// the block that stands for "the platform's own command" would collide over
	// it. A label is only read within the column it appears in, so each of them
	// can carry the same one and a topology can say `placed: {after: [...]}`
	// without knowing which platform is being written.
	Anchor string `yaml:"anchor" mapstructure:"anchor"`
	// Placed puts this block relative to the anchors of the same column,
	// instead of at a number on the priority axis. An anchor no block in the
	// column carries is nothing to sit next to, and the block keeps its place.
	//
	// A number is a poor contract between a topology and a module: it only
	// works while both agree on what the numbers mean, and the module cannot
	// move its own blocks afterwards without breaking the topology. A label is
	// the module's to keep.
	//
	// This is not blocks:, which merges other blocks into this template's
	// output, nor depends:, which says what has to be generated first. The two
	// read alike and are told apart by voice: blocks: lists what is put around
	// this one, placed: says where this one is put. It is refused on a config
	// that writes into no group.
	Placed PlacementConfig `yaml:"placed" mapstructure:"placed"`
	// Priority orders config blocks that are gathered into one place. Smaller is
	// earlier, so a negative value puts a block at the top of a file its group
	// is sorted into.
	//
	// A machine-side hook reads it as well, and there it means where the block
	// sits relative to the command the entry script gives the platform: below
	// zero runs before that command, above it runs after. Zero says nothing -
	// there is no useful block at the command's own position - so a block that
	// leaves it out gets the default for its kind. See HookPriority.
	Priority int `yaml:"priority" mapstructure:"priority"`
	// Used for hierarchy config templates
	// Config template names on same object that need to be embeded
	// Required for ordering config template generation considering the dependency
	Depends []string `yaml:"depends" mapstructure:"depends"`
	// Blocks configuration for flexible config block merging
	Blocks BlocksConfig `yaml:"blocks" mapstructure:"blocks"`

	// Format specifications
	// NamespaceFormat: format applied when registering config blocks to namespace
	NamespaceFormat  string   `yaml:"namespace_format" mapstructure:"namespace_format"`
	NamespaceFormats []string `yaml:"namespace_formats" mapstructure:"namespace_formats"`
	// AssemblyFormat: format applied when assembling config blocks (Sort, blocks)
	AssemblyFormat  string   `yaml:"assembly_format" mapstructure:"assembly_format"`
	AssemblyFormats []string `yaml:"assembly_formats" mapstructure:"assembly_formats"`

	// Condition related fields
	// add config only for interfaces of nodes belongs to the nodeclass(es)
	// this option is valid only on InterfaceClass, ConnectionClass, and their NeighborClass
	NodeClass   string   `yaml:"node" mapstructure:"node"`
	NodeClasses []string `yaml:"nodes" mapstructure:"nodes"`
	// add config only if the neighbor node belongs to the nodeclass(es)
	// this option is valid only on NeighborClass
	NeighborNodeClass   string   `yaml:"neighbor_node" mapstructure:"neighbor_node"`
	NeighborNodeClasses []string `yaml:"neighbor_nodes" mapstructure:"neighbor_nodes"`
	// RequiredParams specifies parameters that must exist for this template to generate output
	// If any of the specified parameters are missing, the entire block is skipped
	RequiredParams []string `yaml:"required_params,flow" mapstructure:"required_params,flow"`
	// RequiredLink marks a template whose output tells the *platform* to create an
	// actual link (containerlab "links:", TiNET "interfaces:"). Such a template must
	// not be emitted for a connection that is only a modelling device - one that puts
	// interfaces in the same segment without a wire existing, e.g. two bridges joined
	// by a VXLAN overlay. Configuration executed *on the device* is unaffected: the
	// interfaces themselves are real and still need their commands.
	//
	// Set by modules on their own wiring templates; users normally never write it.
	RequiredLink bool `yaml:"required_link" mapstructure:"required_link"`

	// PlatformEntry marks a template whose output is what puts the object in
	// place for the platform: containerlab's entry under nodes:, TiNET's under
	// nodes: or switches:, Kathara's device line in lab.conf. It is not
	// configuration written into the object, so virtual - which withholds an
	// object's configuration - does not withhold it. Whether it is written is
	// deploy's answer alone.
	//
	// RequiredLink marks the same kind of output for wiring, with the extra
	// condition that the connection has to be a link; the two together are what
	// PlatformDeclaration reports.
	//
	// Set by modules on their own; users never write it.
	PlatformEntry bool `yaml:"-" mapstructure:"-"`

	Format  string   `yaml:"format" mapstructure:"format"`
	Formats []string `yaml:"formats" mapstructure:"formats"`
	// Load config template
	Template []string `yaml:"template" mapstructure:"template"`
	// Load config template from external file
	SourceFile string `yaml:"sourcefile" mapstructure:"sourcefile"`
	// Raw hands the source file through untouched instead of reading it as a
	// template. Use it for a file that is material rather than a template - one
	// that dot2net has nothing to fill in, and that may contain {{ of its own
	// meant for whoever reads the file later.
	//
	// Only for sourcefile: telling dot2net not to expand a template written
	// inline would be asking it to ignore what the entry is. Write the literal
	// text with Delimiters instead.
	Raw bool `yaml:"raw" mapstructure:"raw"`
	// Delimiters replaces {{ and }} for this template alone, so that text meant
	// for a downstream tool passes through untouched. Generating a file that is
	// itself a template - an Ansible playbook, a TENTOU infra.yml - otherwise
	// means escaping every one of its actions, and an action that happens to
	// name a parameter dot2net knows would be swallowed without a word.
	//
	// Two entries: the opening and closing marks, e.g. ["[[", "]]"].
	Delimiters []string `yaml:"delimiters,flow" mapstructure:"delimiters,flow"`

	ParsedTemplate *template.Template
	rawContent     *string
	paramRefs      []string
	className      string
	classType      string
}

// RawContent returns the text of a raw entry, which is handed through as read
// rather than expanded. The second value says whether this is such an entry.
func (ct *ConfigTemplate) RawContent() (string, bool) {
	if ct.rawContent == nil {
		return "", false
	}
	return *ct.rawContent, true
}

// SortGroupNames lists the groups this sorter gathers. Giving both sort_group
// and sort_groups is refused when the config is read, so the one that is set is
// the whole answer.
func (ct *ConfigTemplate) SortGroupNames() []string {
	if ct.SortGroup != "" {
		return []string{ct.SortGroup}
	}
	return ct.SortGroups
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

// GetNamespaceFormats returns formats to apply when registering config blocks to namespace
// Falls back to GetFormats() for backward compatibility, then to default format
func (ct *ConfigTemplate) GetNamespaceFormats() []string {
	ret := []string{}
	if ct.NamespaceFormat != "" {
		ret = append(ret, ct.NamespaceFormat)
	}
	if len(ct.NamespaceFormats) > 0 {
		ret = append(ret, ct.NamespaceFormats...)
	}
	// Fallback to old format fields for backward compatibility
	if len(ret) == 0 {
		ret = ct.GetFormats()
	}
	// Ultimate fallback to default format
	if len(ret) == 0 {
		return []string{DefaultFormatPhaseFormatName}
	}
	return ret
}

// GetAssemblyFormats returns formats to apply when assembling config blocks (Sort, blocks)
// Falls back to GetFormats() for backward compatibility, then to default format
func (ct *ConfigTemplate) GetAssemblyFormats() []string {
	ret := []string{}
	if ct.AssemblyFormat != "" {
		ret = append(ret, ct.AssemblyFormat)
	}
	if len(ct.AssemblyFormats) > 0 {
		ret = append(ret, ct.AssemblyFormats...)
	}
	// Fallback to old format fields for backward compatibility
	if len(ret) == 0 {
		ret = ct.GetFormats()
	}
	// Ultimate fallback to default format
	if len(ret) == 0 {
		return []string{DefaultMergePhaseFormatName}
	}
	return ret
}

// HasRequiredParams checks if all required parameters exist and are non-empty in the given params map.
// Returns true if RequiredParams is empty or all required parameters exist with non-empty values.
// Returns false if any required parameter is missing or has an empty value.
func (ct *ConfigTemplate) HasRequiredParams(params map[string]string) bool {
	for _, p := range ct.RequiredParams {
		if v, exists := params[p]; !exists || v == "" {
			return false
		}
	}
	return true
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
		ncs = append(ncs, ct.NodeClasses...)
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
		ncs = append(ncs, ct.NeighborNodeClasses...)
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

// DecodeModuleConfig fills dst from the module_config section named for a
// module, and says whether there was one. The section is decoded with the same
// YAML decoder the rest of the file uses, so a module describes its settings
// with an ordinary struct and yaml tags.
func (cfg *Config) DecodeModuleConfig(name string, dst any) (bool, error) {
	raw, ok := cfg.ModuleConfig[name]
	if !ok {
		return false, nil
	}
	bytes, err := yaml.Marshal(raw)
	if err != nil {
		return false, fmt.Errorf("module_config %s: %w", name, err)
	}
	// Same reasoning as LoadConfig: a module's own settings are worth no less
	// care than the rest of the file.
	if err := yaml.UnmarshalWithOptions(bytes, dst, yaml.DisallowUnknownField()); err != nil {
		return false, fmt.Errorf("module_config %s: %w", name, err)
	}
	return true, nil
}

// CheckModuleConfigNames rejects a section naming a module the topology does
// not load. Such a section does nothing, and the reason is nearly always a typo
// or a module removed from the list while its settings stayed behind.
func (cfg *Config) CheckModuleConfigNames() error {
	loaded := map[string]bool{}
	for _, name := range cfg.Modules {
		loaded[name] = true
	}
	names := make([]string, 0, len(cfg.ModuleConfig))
	for name := range cfg.ModuleConfig {
		if !loaded[name] {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return fmt.Errorf(
		"module_config names %s, which this topology does not load; add it to module: or remove the section",
		strings.Join(names, ", "))
}

func LoadConfig(path string) (*Config, error) {

	cfg := Config{GlobalSettings: defaultGlobalSettings()}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// A key this does not know is a key that would be dropped in silence, and a
	// setting that is dropped in silence looks exactly like one that had no
	// effect: example/address_reservation wrote management_layer for mgmt_layer
	// and went a year with its management network switched off. A duplicate key
	// is the same kind of quiet loss, with the later one winning.
	err = yaml.UnmarshalWithOptions(bytes, &cfg,
		yaml.DisallowUnknownField(), yaml.DisallowDuplicateKey())
	if err != nil {
		return nil, err
	}

	// add empty filedef for embedded conifg
	cfg.FileDefinitions = append(cfg.FileDefinitions, &FileDefinition{Name: "", Format: "shell"})

	cfg.localDir = filepath.Dir(path)
	cfg.fileDefinitionMap = map[string]*FileDefinition{}
	for _, filedef := range cfg.FileDefinitions {
		if err := filedef.checkProvide(); err != nil {
			return nil, err
		}
		cfg.fileDefinitionMap[filedef.Name] = filedef
	}
	cfg.formatStyleMap = map[string]*FormatStyle{}
	// Add default formats
	cfg.formatStyleMap[DefaultFormatPhaseFormatName] = DefaultFormatPhaseFormat
	cfg.formatStyleMap[DefaultMergePhaseFormatName] = DefaultMergePhaseFormat
	for _, fmtstyle := range cfg.FormatStyles {
		cfg.formatStyleMap[fmtstyle.Name] = fmtstyle
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
		// Check for reserved parameter names
		if msg := CheckReservedParamName(prule.Name); msg != "" {
			return nil, fmt.Errorf("in 'param_rule' section (name: %s): %s", prule.Name, msg)
		}
		if _, dup := cfg.parameterRuleMap[prule.Name]; dup {
			return nil, fmt.Errorf("duplicate param_rule name %q", prule.Name)
		}
		cfg.parameterRuleMap[prule.Name] = prule
	}

	cfg.nodeClassMap = map[string]*NodeClass{}
	for _, node := range cfg.NodeClasses {
		if _, dup := cfg.nodeClassMap[node.Name]; dup {
			return nil, fmt.Errorf("duplicate nodeclass name %q", node.Name)
		}
		cfg.nodeClassMap[node.Name] = node
	}
	cfg.interfaceClassMap = map[string]*InterfaceClass{}
	cfg.neighborClassMap = map[string]map[string][]*NeighborClass{}
	for _, iface := range cfg.InterfaceClasses {
		if _, dup := cfg.interfaceClassMap[iface.Name]; dup {
			return nil, fmt.Errorf("duplicate interfaceclass name %q", iface.Name)
		}
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
		if _, dup := cfg.connectionClassMap[conn.Name]; dup {
			return nil, fmt.Errorf("duplicate connectionclass name %q", conn.Name)
		}
		cfg.connectionClassMap[conn.Name] = conn
	}
	cfg.groupClassMap = map[string]*GroupClass{}
	for _, group := range cfg.GroupClasses {
		if _, dup := cfg.groupClassMap[group.Name]; dup {
			return nil, fmt.Errorf("duplicate groupclass name %q", group.Name)
		}
		cfg.groupClassMap[group.Name] = group
	}
	cfg.segmentClassMap = map[string]*SegmentClass{}
	for _, segment := range cfg.SegmentClasses {
		if _, dup := cfg.segmentClassMap[segment.Name]; dup {
			return nil, fmt.Errorf("duplicate segmentclass name %q", segment.Name)
		}
		cfg.segmentClassMap[segment.Name] = segment
	}
	if err := cfg.resolveClassPolicy(); err != nil {
		return nil, err
	}

	cfg.SorterConfigTemplateGroups = mapset.NewSet[string]()

	return &cfg, err
}

// resolveClassPolicy fills resolvedClassPolicy from the class_policy section,
// falling back to the legacy magic class names ("all" / "default") when a slot is
// not configured. Using a legacy name prints a deprecation warning once.
//
// A class named in class_policy must exist, otherwise the policy would silently do
// nothing; that is reported as a configuration error.
func (cfg *Config) resolveClassPolicy() error {
	cfg.resolvedClassPolicy = map[string]ClassPolicyEntry{}

	types := []struct {
		classType string
		entry     ClassPolicyEntry
		exists    func(string) bool
	}{
		{ClassTypeNode, cfg.ClassPolicy.Node, func(n string) bool { _, ok := cfg.nodeClassMap[n]; return ok }},
		{ClassTypeInterface, cfg.ClassPolicy.Interface, func(n string) bool { _, ok := cfg.interfaceClassMap[n]; return ok }},
		{ClassTypeConnection, cfg.ClassPolicy.Connection, func(n string) bool { _, ok := cfg.connectionClassMap[n]; return ok }},
		{ClassTypeGroup, cfg.ClassPolicy.Group, func(n string) bool { _, ok := cfg.groupClassMap[n]; return ok }},
		{ClassTypeSegment, cfg.ClassPolicy.Segment, func(n string) bool { _, ok := cfg.segmentClassMap[n]; return ok }},
	}

	legacy := map[string][]string{} // legacy name -> class types still relying on it

	for _, t := range types {
		resolved := ClassPolicyEntry{Base: t.entry.Base, Default: t.entry.Default}

		if len(resolved.Base) == 0 && t.exists(ClassAll) {
			resolved.Base = []string{ClassAll}
			legacy[ClassAll] = append(legacy[ClassAll], t.classType)
		}
		if len(resolved.Default) == 0 && t.exists(ClassDefault) {
			resolved.Default = []string{ClassDefault}
			legacy[ClassDefault] = append(legacy[ClassDefault], t.classType)
		}

		names := append(append([]string{}, resolved.Base...), resolved.Default...)
		for _, name := range names {
			if !t.exists(name) {
				return fmt.Errorf("class_policy for %s refers to undefined %sclass %q", t.classType, t.classType, name)
			}
		}
		cfg.resolvedClassPolicy[t.classType] = resolved
	}

	for _, name := range []string{ClassAll, ClassDefault} {
		if classTypes, ok := legacy[name]; ok {
			slot := "base"
			if name == ClassDefault {
				slot = "default"
			}
			fmt.Fprintf(os.Stderr,
				"warning: class name %q is deprecated as an implicit rule (used for: %s). "+
					"Declare it explicitly instead:\n  class_policy:\n    <type>:\n      %s: [%s]\n",
				name, strings.Join(classTypes, ", "), slot, name)
		}
	}

	return nil
}

func loadTemplate(tpl []string, path string, delims []string) (*template.Template, error) {
	t := template.New("")
	if len(delims) > 0 {
		t = t.Delims(delims[0], delims[1])
	}
	if len(tpl) == 0 && path == "" {
		return t.Parse("")
	} else if len(tpl) == 0 {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return t.Parse(convertLineFeed(string(bytes), "\n"))
	}
	return t.Parse(strings.Join(tpl, "\n"))
}

func initConfigTemplate(cfg *Config, ct *ConfigTemplate) error {
	// check if the config template is sort-style
	if ct.Style == ConfigTemplateStyleSort {
		if ct.SortGroup != "" && len(ct.SortGroups) > 0 {
			return fmt.Errorf(
				"config %s gives both sort_group and sort_groups; put every group in sort_groups", ct)
		}
		groups := ct.SortGroupNames()
		if len(groups) == 0 {
			return fmt.Errorf(
				"sort-style config template %s should have a sort_group (or sort_groups) attribute", ct)
		}
		seen := mapset.NewSet[string]()
		for _, group := range groups {
			if group == "" {
				return fmt.Errorf("config %s has an empty group name in sort_groups", ct)
			}
			if !seen.Add(group) {
				return fmt.Errorf(
					"config %s names group %q twice in sort_groups, which would gather its blocks twice",
					ct, group)
			}
			if group == ct.Group {
				return fmt.Errorf(
					"config %s gathers group %q and writes its result back into it", ct, group)
			}
			cfg.SorterConfigTemplateGroups.Add(group)
			if !ct.ModuleProvided {
				if cfg.topologySortGroups == nil {
					cfg.topologySortGroups = map[string]bool{}
				}
				cfg.topologySortGroups[group] = true
			}
		}
	}
	if ct.Group != "" {
		cfg.groupContributions = append(cfg.groupContributions, ct)
	}

	if ct.Anchor != "" {
		if cfg.configTemplateAnchors == nil {
			cfg.configTemplateAnchors = map[string]bool{}
		}
		cfg.configTemplateAnchors[ct.Anchor] = true
	}

	if ct.Anchor != "" && ct.Group == "" {
		return fmt.Errorf(
			"config %s carries the anchor %q, which is a label read within a sorted column, "+
				"but the config writes into no group", ct, ct.Anchor)
	}

	if ct.Placed.Given() {
		if ct.Group == "" {
			return fmt.Errorf(
				"config %s says placed:, which puts a block among the others of a sorted "+
					"column, but the config writes into no group. To order the generation of "+
					"templates instead, use depends:; to merge other blocks into this one, "+
					"use blocks:", ct)
		}
		if ct.Priority != 0 {
			return fmt.Errorf(
				"config %s says both a priority and placed:; a block sits either at a number "+
					"or next to an anchor, not both", ct)
		}
		for _, anchor := range ct.Placed.anchors() {
			if anchor == "" {
				return fmt.Errorf("config %s has an empty anchor name in placed:", ct)
			}
			if anchor == ct.Anchor {
				return fmt.Errorf("config %s is placed relative to itself", ct)
			}
		}
	}

	// A config entry is one thing or the other. Holding both would leave the
	// order between them to be decided somewhere, and it was decided silently:
	// the inline lines came first and the file after, which nothing said and
	// nothing used. Two entries express the same thing, and blocks: puts them
	// in the order the author wants.
	if len(ct.Template) > 0 && ct.SourceFile != "" {
		return fmt.Errorf(
			"config %s names both template and sourcefile; write them as two config entries "+
				"and order them with blocks:", ct)
	}
	if ct.Raw && ct.SourceFile == "" {
		return fmt.Errorf(
			"config %s is marked raw but names no sourcefile; raw hands a file through "+
				"untouched, so there is nothing for it to do here. To write literal {{ in a "+
				"template, set delimiters instead", ct)
	}
	if len(ct.Delimiters) > 0 && len(ct.Delimiters) != 2 {
		return fmt.Errorf(
			"config %s gives %d delimiters; it takes two, the opening and closing marks, "+
				"e.g. [\"[[\", \"]]\"]", ct, len(ct.Delimiters))
	}
	if ct.Raw && len(ct.Delimiters) > 0 {
		return fmt.Errorf(
			"config %s is both raw and given delimiters; a file handed through untouched has "+
				"no actions to delimit", ct)
	}

	// init parsed template object
	path := ""
	if ct.SourceFile != "" {
		path = GetRelativeFilePath(ct.SourceFile, cfg)
	}
	if ct.Raw {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read raw source of %s: %w", ct, err)
		}
		content := convertLineFeed(string(bytes), "\n")
		ct.rawContent = &content
		return nil
	}
	tpl, err := loadTemplate(ct.Template, path, ct.Delimiters)
	if err != nil {
		return fmt.Errorf("failed to load template %+v: %w", ct, err)
	}
	ct.ParsedTemplate = tpl
	ct.setParamRefs()

	return nil
}

// forEachConfigTemplate walks every config template a config holds, telling the
// callback which class it came from. Two passes need the same walk and they run
// at different moments, so the walk is written once.
func forEachConfigTemplate(cfg *Config, fn func(ct *ConfigTemplate, classType, className string) error) error {
	visit := func(cts []*ConfigTemplate, classType, className string) error {
		for _, ct := range cts {
			if err := fn(ct, classType, className); err != nil {
				return err
			}
		}
		return nil
	}

	for _, networkClass := range cfg.NetworkClasses {
		if err := visit(networkClass.ConfigTemplates, ClassTypeNetwork, networkClass.Name); err != nil {
			return err
		}
	}
	for _, nc := range cfg.NodeClasses {
		if err := visit(nc.ConfigTemplates, ClassTypeNode, nc.Name); err != nil {
			return err
		}
		for _, mc := range nc.MemberClasses {
			if err := visit(mc.ConfigTemplates, ClassTypeMember(ClassTypeNode, ""), ""); err != nil {
				return err
			}
		}
	}
	for _, ic := range cfg.InterfaceClasses {
		if err := visit(ic.ConfigTemplates, ClassTypeInterface, ic.Name); err != nil {
			return err
		}
		for _, nc := range ic.NeighborClasses {
			if err := visit(nc.ConfigTemplates, ClassTypeNeighbor(""), ""); err != nil {
				return err
			}
		}
		for _, mc := range ic.MemberClasses {
			if err := visit(mc.ConfigTemplates, ClassTypeMember(ClassTypeInterface, ""), ""); err != nil {
				return err
			}
		}
	}
	for _, cc := range cfg.ConnectionClasses {
		if err := visit(cc.ConfigTemplates, ClassTypeConnection, cc.Name); err != nil {
			return err
		}
		for _, mc := range cc.MemberClasses {
			if err := visit(mc.ConfigTemplates, ClassTypeMember(ClassTypeConnection, ""), ""); err != nil {
				return err
			}
		}
	}
	for _, sc := range cfg.SegmentClasses {
		if err := visit(sc.ConfigTemplates, ClassTypeSegment, sc.Name); err != nil {
			return err
		}
	}
	for _, gc := range cfg.GroupClasses {
		if err := visit(gc.ConfigTemplates, ClassTypeGroup, gc.Name); err != nil {
			return err
		}
	}
	// ConfigTemplates in ParameterRules (for attach mode)
	for _, pr := range cfg.ParameterRules {
		if err := visit(pr.ConfigTemplates, ClassTypeValue(pr.Name), ""); err != nil {
			return err
		}
	}
	return nil
}

// NormalizeHookNames reads a hook written the old way - a config template named
// after the hook - as what it means now, a block written into the hook's group.
//
// It runs before the classes are checked over rather than with the rest of the
// template loading, because the check that two classes do not define one name
// runs there: a name that is really a group has to have stopped being a name by
// then. Two classes writing into one group is ordinary, and the reason the
// duplicate-name check no longer has a hook-shaped hole in it.
func NormalizeHookNames(cfg *Config) {
	_ = forEachConfigTemplate(cfg, func(ct *ConfigTemplate, classType, className string) error {
		if ct.Name != "" && ct.Group == "" && HookConfigNames[ct.Name] {
			ct.Group = ct.Name
			ct.Name = ""
			if cfg.renamedHooks == nil {
				cfg.renamedHooks = map[string]bool{}
			}
			cfg.renamedHooks[ct.Group] = true
		}
		return nil
	})
}

func LoadTemplates(cfg *Config) (*Config, error) {
	// className is set only for LabelOwners, for checking config template
	// conditions of classnames
	err := forEachConfigTemplate(cfg, func(ct *ConfigTemplate, classType, className string) error {
		ct.className = className
		ct.classType = classType
		return initConfigTemplate(cfg, ct)
	})
	if err != nil {
		return nil, err
	}

	if err := checkSortGroups(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ownedHookNames lists the names dot2net owns, in a fixed order, for messages.
func ownedHookNames() []string {
	names := make([]string, 0, len(HookConfigNames))
	for name := range HookConfigNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// checkSortGroups rejects a config template that writes into a group no
// sort-style config template collects.
//
// Such a block is generated and then goes nowhere: the aggregator keeps blocks
// per sorter, so a group with no sorter has nothing to hand them to. The usual
// cause is a typo in either name, and until this check the only symptom was
// output that quietly lacked a section.
//
// It runs once every template is loaded, modules included, because a topology
// may write into a group a module declares the sorter for, and either may be
// read first.
func checkSortGroups(cfg *Config) error {
	if len(cfg.renamedHooks) > 0 {
		names := make([]string, 0, len(cfg.renamedHooks))
		for name := range cfg.renamedHooks {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Fprintf(os.Stderr,
			"warning: %s written as name: is read as group: (deprecated). A hook is a group "+
				"that whichever platform is bringing the lab up gathers, so write it as\n"+
				"  - group: %s\n",
			strings.Join(names, ", "), names[0])
	}

	for _, ct := range cfg.groupContributions {
		if HookConfigNames[ct.Group] {
			// A name dot2net owns is gathered by whatever module is writing the
			// files, and by none when no module is. The second is worth saying
			// plainly rather than as a missing group.
			if !cfg.SorterConfigTemplateGroups.Contains(ct.Group) {
				return fmt.Errorf(
					"config %s writes into %s, but nothing is being generated that would run it: "+
						"a hook is gathered by the platform's own files, and none are being "+
						"written. Turn on module_config.<module>.generate_scripts, or take the "+
						"block out",
					ct, ct.Group)
			}
			continue
		}
		if cfg.SorterConfigTemplateGroups.Contains(ct.Group) {
			// Gathered by someone - but a topology may only write into what it
			// gathers itself, or into a name dot2net owns. Anything else is a
			// module's own group, and writing into it reaches into the module's
			// insides.
			if !ct.ModuleProvided && !cfg.topologySortGroups[ct.Group] {
				return fmt.Errorf(
					"config %s writes into group %q, which belongs to a module: a topology may "+
						"write into the groups it sorts itself and into the ones dot2net owns (%s)",
					ct, ct.Group, strings.Join(ownedHookNames(), ", "))
			}
			continue
		}
		known := cfg.SorterConfigTemplateGroups.ToSlice()
		sort.Strings(known)
		classType, className := ct.GetClassInfo()
		where := fmt.Sprintf("%s %s", classType, className)
		if className == "" {
			where = classType
		}
		if len(known) == 0 {
			return fmt.Errorf(
				"config %s in %s writes into group %q, but no config template sorts any group; "+
					"a sorter is a config template with style: sort and sort_group: %s",
				ct, where, ct.Group, ct.Group)
		}
		return fmt.Errorf(
			"config %s in %s writes into group %q, which no config template sorts; "+
				"sorted groups are: %s",
			ct, where, ct.Group, strings.Join(known, ", "))
	}

	for _, ct := range cfg.groupContributions {
		for _, anchor := range ct.Placed.anchors() {
			if cfg.configTemplateAnchors[anchor] {
				continue
			}
			return fmt.Errorf(
				"config %s is placed relative to %q, which no config template carries as an anchor",
				ct, anchor)
		}
	}

	return nil
}

// PlatformDeclaration reports whether this template's output is the platform's
// own record that the object exists, rather than configuration written into it.
// The distinction is what keeps the two axes apart at the point of output:
// deploy decides whether an object is put in place, virtual decides whether its
// configuration is written, and a template of this kind answers to the first.
func (ct *ConfigTemplate) PlatformDeclaration() bool {
	return ct.PlatformEntry || ct.RequiredLink
}
