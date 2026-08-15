package types

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"text/template"

	mapset "github.com/deckarep/golang-set/v2"
)

const DefaultNodePrefix string = "node"

// DefaultInterfacePrefix names the interfaces a topology does not name itself.
// eth is what a Linux container calls them and what Kathara requires - it
// derives the name from the index in lab.conf and offers no way to change it -
// so this is the one prefix every platform accepts.
//
// The exception is containerlab's management network, which takes eth0 for
// itself and refuses a data interface by that name. A topology that turns it on
// (module_config.containerlab.management_network) has to name its interfaces
// something else.
const DefaultInterfacePrefix string = "eth"
const DefaultConnectionPrefix string = "conn"
const DefaultSegmentPrefix string = "seg"

// abstracted module

// Module is the minimum every module must provide: injecting its classes,
// templates, formats and file definitions into the configuration, plus the class
// label registry that StandardModule implements.
//
// Everything a module does beyond that is an optional interface (ParameterProvider,
// RequirementChecker, ParameterGenerator, ...). A module implements only what it
// actually does and declares it with a compile-time assertion at the top of its
// file, so "what does this module provide?" is answerable by reading the module
// rather than by reading empty method bodies:
//
//	// Capabilities provided by this module.
//	var (
//		_ types.Module            = (*ClabModule)(nil)
//		_ types.ParameterProvider = (*ClabModule)(nil)
//	)
//
// Keeping the optional parts out of Module also means a new hook can be added
// without breaking every existing module.
type Module interface {
	UpdateConfig(cfg *Config) error
	AddModuleNodeClassLabel(label string)
	GetModuleNodeClassLabels() []string
	AddModuleInterfaceClassLabel(label string)
	GetModuleInterfaceClassLabels() []string
	AddModuleConnectionClassLabel(label string)
	GetModuleConnectionClassLabels() []string
	AddModuleGroupClassLabel(label string)
	GetModuleGroupClassLabels() []string
}

// ObjectClassifier is optionally implemented by modules that need to look at the
// topology and classify or reshape it *before* class labels are resolved.
//
// It runs between skeleton construction and class resolution. That position is the
// whole point: class labels are turned into class objects once, in checkClasses,
// so a label added after that would never be resolved. A module that wants an
// object to be picked up by a connectionclass, param_rule or template condition has
// to add the label here.
//
// At this point objects exist and carry their DOT labels, but they are not named
// yet and have almost no parameters - naming and parameter assignment come later.
// So this hook is for deciding *what an object is*, not for computing values; use
// ParameterProvider for that.
type ObjectClassifier interface {
	ClassifyObjects(cfg *Config, nm *NetworkModel) error
}

// ParameterProvider is optionally implemented by modules that supply parameters
// while the network model is being built. It runs after object naming and before
// address and param_rule assignment, so it can also override values that came from
// DOT labels.
type ParameterProvider interface {
	GenerateParameters(cfg *Config, nm *NetworkModel) error
}

// RequirementChecker is optionally implemented by modules that validate the model
// before configuration files are generated (required parameters, naming rules the
// target platform imposes, ...).
type RequirementChecker interface {
	CheckModuleRequirements(cfg *Config, nm *NetworkModel) error
}

// ParameterGenerator is optionally implemented by modules that can generate Value parameter lists
// for attach mode param_rules with generator specification (e.g., "clab.filemounts")
//
// Note: unlike ParameterProvider and RequirementChecker, this is not a pipeline
// hook. It is looked up *by name* from the YAML (`generator: clab.filemounts`), so
// only the module named there is called. That difference in dispatch, not an
// oversight, is why it is shaped differently from the hooks above.
type ParameterGenerator interface {
	// GenerateValueParameters generates a list of parameter sets for creating Values
	// generatorName: the generator name after the module prefix (e.g., "filemounts" for "clab.filemounts")
	// target: the target object (Node, Interface, etc.)
	// cfg: the configuration
	// nm: the network model
	// Returns: a list of parameter maps, one per Value to be created
	GenerateValueParameters(
		generatorName string,
		target ValueOwner,
		cfg *Config,
		nm *NetworkModel,
	) ([]map[string]string, error)
}

type StandardModule struct {
	NodeClassLabels       []string
	InterfaceClassLabels  []string
	ConnectionClassLabels []string
	GroupClassLabels      []string
}

func NewStandardModule() *StandardModule {
	return &StandardModule{
		NodeClassLabels:       []string{},
		InterfaceClassLabels:  []string{},
		ConnectionClassLabels: []string{},
		GroupClassLabels:      []string{},
	}
}

func (m *StandardModule) AddModuleNodeClassLabel(label string) {
	m.NodeClassLabels = append(m.NodeClassLabels, label)
}

func (m *StandardModule) GetModuleNodeClassLabels() []string {
	return m.NodeClassLabels
}

func (m *StandardModule) AddModuleInterfaceClassLabel(label string) {
	m.InterfaceClassLabels = append(m.InterfaceClassLabels, label)
}

func (m *StandardModule) GetModuleInterfaceClassLabels() []string {
	return m.InterfaceClassLabels
}

func (m *StandardModule) AddModuleConnectionClassLabel(label string) {
	m.ConnectionClassLabels = append(m.ConnectionClassLabels, label)
}

func (m *StandardModule) GetModuleConnectionClassLabels() []string {
	return m.ConnectionClassLabels
}

func (m *StandardModule) AddModuleGroupClassLabel(label string) {
	m.GroupClassLabels = append(m.GroupClassLabels, label)
}

func (m *StandardModule) GetModuleGroupClassLabels() []string {
	return m.GroupClassLabels
}

// abstracted structures

type ObjectInstance interface {
	StringForMessage() string // just for debug messages
}

// FileGenerator is an interface for objects that can generate files.
// For the list of implementers, see the interface assertions placed next to each
// type definition (do not enumerate them here: such lists go stale).
type FileGenerator interface {
	// FilesToGenerate returns a list of file names that this object will generate based on its classes.
	FilesToGenerate(cfg *Config) []string
}

// NameSpacer is an element of top-down network model
// A namespacer owns parameter namespace and generates configuration blocks
// For the list of implementers, see the interface assertions placed next to each
// type definition (do not enumerate them here: such lists go stale).
type NameSpacer interface {
	// Methods to trace top-down network model
	ChildClasses() ([]string, error)
	Childs(c string) ([]NameSpacer, error)
	// Methods for dependency graph processing
	DependClasses() ([]string, error)
	Depends(c string) ([]NameSpacer, error)

	setParamFlag(k string)
	hasParamFlag(k string) bool
	IterateFlaggedParams() <-chan string
	AddParam(k, v string)
	HasParam(k string) bool
	setParams(map[string]string)
	// BuildRelativeNameSpace() error
	BuildRelativeNameSpace(globalParams map[string]map[string]string) error
	SetRelativeParam(k, v string)
	HasRelativeParam(k string) bool
	SetRelativeParams(map[string]string)
	GetParams() map[string]string
	GetRelativeParams() map[string]string
	GetParamValue(string) (string, error)

	GetConfigTemplates(cfg *Config) []*ConfigTemplate
	GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate

	ObjectInstance
}

// Namespace only implements parameter related methods, but does not provide top-down structure
type NameSpace struct {
	paramFlags     mapset.Set[string]
	params         map[string]string
	relativeParams map[string]string
}

func newNameSpace() *NameSpace {
	return &NameSpace{
		paramFlags:     mapset.NewSet[string](),
		params:         map[string]string{},
		relativeParams: map[string]string{},
	}
}

func (ns *NameSpace) setParamFlag(k string) {
	ns.paramFlags.Add(k)
}

func (ns *NameSpace) hasParamFlag(k string) bool {
	return ns.paramFlags.Contains(k)
}

func (ns *NameSpace) IterateFlaggedParams() <-chan string {
	return ns.paramFlags.Iter()
}

// AddParam sets parameter k to v with last-write-wins semantics: it performs a
// plain map assignment and neither checks for an existing key nor rejects
// reserved names / prefixes. Parameter assignment is therefore order-dependent;
// the model build establishes the values in a fixed sequence (network -> node
// -> connection -> interface -> segment, see assign*Parameters in
// pkg/model/parameter.go), and callers relying on a value must run after the
// producer that sets it.
func (ns *NameSpace) AddParam(k, v string) {
	ns.params[k] = v
}

func (ns *NameSpace) HasParam(k string) bool {
	_, ok := ns.params[k]
	return ok
}

func (ns *NameSpace) setParams(given map[string]string) {
	if len(ns.params) == 0 {
		ns.params = given
	} else {
		for k, v := range given {
			ns.params[k] = v
		}
	}
}

func (ns *NameSpace) GetParams() map[string]string {
	return ns.params
}

func (ns *NameSpace) SetRelativeParam(k, v string) {
	ns.relativeParams[k] = v
}

func (ns *NameSpace) HasRelativeParam(k string) bool {
	_, ok := ns.relativeParams[k]
	return ok
}

func (ns *NameSpace) SetRelativeParams(given map[string]string) {
	if len(ns.relativeParams) == 0 {
		ns.relativeParams = given
	} else {
		for k, v := range given {
			ns.relativeParams[k] = v
		}
	}
}

func (ns *NameSpace) GetRelativeParams() map[string]string {
	return ns.relativeParams
}

// GetParamValue returns a parameter value from relative namespace
func (ns *NameSpace) GetParamValue(key string) (string, error) {
	val, ok := ns.relativeParams[key]
	if ok {
		return val, nil
	} else {
		return val, fmt.Errorf("unknown key %v", key)
	}
}

// LabelOwner is implemented by objects that carry class labels from the DOT input.
// For the list of implementers, see the interface assertions placed next to each
// type definition (do not enumerate them here: such lists go stale).
type LabelOwner interface {
	ClassLabels() []string
	RelationalClassLabels() []RelationalClassLabel
	PlaceLabels() []string
	ValueLabels() map[string]string
	MetaValueLabels() map[string]string
	SetLabels(cfg *Config, labels []string, moduleLabels []string) error
	AddClassLabels(labels ...string)
	AddModuleClassLabels(labels ...string)

	HasClass(string) bool
	GetClasses() []ObjectClass
	ClassTier(string) int

	SetVirtual(bool)
	IsVirtual() bool

	SetDeployForm(string)
	DeployForm() string
	IsMaterialised() bool

	ClassDefinition(cfg *Config, cls string) (interface{}, error)

	ObjectInstance
}

type RelationalClassLabel struct {
	ClassType string
	Name      string
}

// Class tiers determine which class wins when several classes give different
// values for the same attribute. Larger is stronger.
//
// Weakest to strongest: module-provided -> base -> user-written. Modules are the
// weakest on purpose: they supply defaults, and anything the user writes must win
// over them. A module that has to *force* a value (for example the Kathara module
// pinning the interface prefix to "eth", because Kathara derives interface names
// from lab.conf indices) must not win silently - it should check the value and
// report a module-specific error instead.
//
// DOT value labels are not part of this scale; they are applied before any class
// value and therefore beat all of them (see setGivenParameters).
const (
	ClassTierModule = iota // classes injected by modules
	ClassTierBase          // the "base" (formerly "all") class
	ClassTierUser          // classes named by the user, and the "default" class
)

type ParsedLabels struct {
	classLabels     []string
	rClassLabels    []RelationalClassLabel
	placeLabels     []string
	valueLabels     map[string]string
	metaValueLabels map[string]string
	Classes         []ObjectClass
	// classTiers maps a class name to its tier. Keyed by name rather than kept
	// parallel to Classes because Classes is rebuilt in several places and skips
	// entries that cannot be resolved, which would desynchronise a parallel slice.
	classTiers map[string]int
	// virtual withholds the object's own configuration. It says nothing about
	// whether the object exists; deploy answers that.
	virtual bool
	// deploy is what the object is materialised as, resolved once the classes
	// and the objects around it have been weighed. Empty for a group, which
	// nothing materialises.
	deploy string
}

func newParsedLabels() *ParsedLabels {
	return &ParsedLabels{
		classLabels:     []string{},
		rClassLabels:    []RelationalClassLabel{},
		placeLabels:     []string{},
		valueLabels:     map[string]string{},
		metaValueLabels: map[string]string{},
		classTiers:      map[string]int{},
	}
}

// ClassTier returns the tier of the named class. Unknown classes are treated as
// user-written, which is the safe default: an unexpected class then conflicts
// with other user classes instead of silently overriding them.
// CloneLabels returns a copy that shares nothing with the receiver, so a label
// added to one does not turn up on the other. Replicating an object needs this:
// a bridge split across machines starts from the same classes as the original
// and then goes its own way.
func (l *ParsedLabels) CloneLabels() *ParsedLabels {
	c := newParsedLabels()
	c.classLabels = append(c.classLabels, l.classLabels...)
	c.rClassLabels = append(c.rClassLabels, l.rClassLabels...)
	c.placeLabels = append(c.placeLabels, l.placeLabels...)
	for k, v := range l.valueLabels {
		c.valueLabels[k] = v
	}
	for k, v := range l.metaValueLabels {
		c.metaValueLabels[k] = v
	}
	for k, v := range l.classTiers {
		c.classTiers[k] = v
	}
	c.Classes = append(c.Classes, l.Classes...)
	c.virtual = l.virtual
	return c
}

func (l *ParsedLabels) ClassTier(name string) int {
	if l.classTiers == nil {
		return ClassTierUser
	}
	if tier, ok := l.classTiers[name]; ok {
		return tier
	}
	return ClassTierUser
}

// setClassTier records the tier of a class label.
func (l *ParsedLabels) setClassTier(name string, tier int) {
	if l.classTiers == nil {
		l.classTiers = map[string]int{}
	}
	l.classTiers[name] = tier
}

func (l *ParsedLabels) ClassLabels() []string {
	return l.classLabels
}

func (l *ParsedLabels) RelationalClassLabels() []RelationalClassLabel {
	return l.rClassLabels
}

func (l *ParsedLabels) PlaceLabels() []string {
	return l.placeLabels
}

func (l *ParsedLabels) ValueLabels() map[string]string {
	return l.valueLabels
}

func (l *ParsedLabels) MetaValueLabels() map[string]string {
	return l.metaValueLabels
}

// AddClassLabels adds labels that originate from the user's input (relational
// class labels of a connection, segment classes, ...), so they share the user
// tier.
func (l *ParsedLabels) AddClassLabels(labels ...string) {
	l.addClassLabels(ClassTierUser, labels...)
}

// AddModuleClassLabels adds labels on behalf of a module, at the module tier.
// A module classifying objects after SetLabels - through the ObjectClassifier
// hook - has to come in here rather than through AddClassLabels: modules supply
// defaults and must lose to anything the user wrote, and filing their labels as
// user-written would turn that into a same-tier conflict instead.
func (l *ParsedLabels) AddModuleClassLabels(labels ...string) {
	l.addClassLabels(ClassTierModule, labels...)
}

func (l *ParsedLabels) addClassLabels(tier int, labels ...string) {
	l.classLabels = append(l.classLabels, labels...)
	// setClassTier is skipped for names that already have a tier: a label the
	// user also wrote keeps the tier it was first recorded with.
	for _, name := range labels {
		if _, ok := l.classTiers[name]; !ok {
			l.setClassTier(name, tier)
		}
	}
}

// tieredValues resolves attribute values contributed by several classes.
//
// The strongest class to set a key wins, whatever order the classes are visited
// in; only two classes of the same tier setting different values are a conflict
// the user has to resolve, because the order inside one tier comes from the
// order of labels in the DOT file and is not meaningful.
//
// Resolution must not depend on visit order, because ClassLabels does not list
// classes strongest-first: expandUsedClasses appends the classes reached through
// use: after everything else, keeping the tier they were defined at. A base
// class visited before a used user class would otherwise either win outright or,
// worse, be reported as a same-tier conflict with it.
type tieredValues struct {
	values map[string]string
	tiers  map[string]int
}

func newTieredValues() *tieredValues {
	return &tieredValues{values: map[string]string{}, tiers: map[string]int{}}
}

// set records value for key. It reports the conflicting value and false when a
// class in the same tier already set a different value.
func (t *tieredValues) set(key, value string, tier int) (string, bool) {
	prev, seen := t.values[key]
	switch {
	case !seen, t.tiers[key] < tier:
		t.values[key] = value
		t.tiers[key] = tier
		return "", true
	case t.tiers[key] > tier:
		return "", true // a stronger class has already spoken
	case prev != value:
		return prev, false
	}
	return "", true
}

// get returns the winning value for key, if any.
func (t *tieredValues) get(key string) (string, bool) {
	v, ok := t.values[key]
	return v, ok
}

// conflictName identifies an interface in error messages. SetClasses runs before
// interface names are assigned, so the name is often still empty at this point.
func (iface *Interface) conflictName() string {
	if iface.Name == "" {
		return iface.Node.Name + ".<unnamed>"
	}
	return iface.Node.Name + "." + iface.Name
}

// classConflictError reports two classes of the same tier disagreeing on an
// attribute. Only same-tier clashes reach this point: a stronger class silently
// overrides a weaker one by design.
func classConflictError(objType, objName, attr, existing, conflicting string) error {
	return fmt.Errorf(
		"configuration conflict detected on %s '%s': classes of the same precedence define different %s ('%s' vs '%s'). "+
			"Classes named in the DOT label have no order between them, so this cannot be resolved automatically. "+
			"Either give them the same value, leave one empty, or move the differing value to a more specific class",
		objType, objName, attr, existing, conflicting)
}

func (l *ParsedLabels) HasClass(name string) bool {
	for _, cls := range l.classLabels {
		if cls == name {
			return true
		}
	}
	return false
}

func (l *ParsedLabels) GetClasses() []ObjectClass {
	return l.Classes
}

func (l *ParsedLabels) SetVirtual(flag bool) {
	l.virtual = flag
}

func (l *ParsedLabels) IsVirtual() bool {
	return l.virtual
}

// SetDeployForm records what the object is materialised as, once the classes it
// carries have been weighed and the forms of the objects around it have been
// taken into account. resolveDeployForms does that, and nothing else writes it.
func (l *ParsedLabels) SetDeployForm(form string) {
	l.deploy = form
}

// DeployForm reports what the object is materialised as: one of the Deploy
// constants, or "" for an object the question does not apply to. A group is the
// only such object - it is a scope, and neither the platform nor the generated
// configuration puts anything in place for one.
func (l *ParsedLabels) DeployForm() string {
	return l.deploy
}

// IsMaterialised reports whether anything puts this object in place. It is the
// question virtual used to be asked to answer, and the two have been separate
// since v0.8: virtual withholds an object's configuration, while this says
// whether the object is there at all.
func (l *ParsedLabels) IsMaterialised() bool {
	return l.deploy != DeployNone
}

// classMemberReferer includes Node, Interface
// commented out because it currently does not have abstracted usage (explicitly addressed)
// Note: MemberClass is defined in config.go
type MemberReferrer interface {
	LabelOwner
	NameSpacer
	ObjectInstance

	AddMemberClass(*MemberClass)
	GetMemberClasses() []*MemberClass
	AddMember(*Member)
	GetMembers() []*Member
}

type memberReference struct {
	memberClasses []*MemberClass
	members       []*Member
}

func newMemberReference() *memberReference {
	return &memberReference{
		memberClasses: []*MemberClass{},
		members:       []*Member{},
	}
}

func (mr *memberReference) AddMemberClass(mc *MemberClass) {
	mr.memberClasses = append(mr.memberClasses, mc)
}

func (mr *memberReference) GetMemberClasses() []*MemberClass {
	return mr.memberClasses
}

func (mr *memberReference) AddMember(m *Member) {
	mr.members = append(mr.members, m)
}

func (mr *memberReference) GetMembers() []*Member {
	return mr.members
}

// ValueOwner is an interface for objects that can have Values attached.
// Values are virtual objects generated by param_rule with mode: attach.
// For the list of implementers, see the interface assertions placed next to each
// type definition (do not enumerate them here: such lists go stale).
type ValueOwner interface {
	NameSpacer

	// SortKey returns a string key for deterministic sorting of objects.
	// Used to ensure stable parameter assignment order regardless of map iteration order.
	SortKey() string
	// AddValue adds a Value to this owner
	AddValue(v *Value)
	// GetValues returns all Values attached to this owner
	GetValues() []*Value
	// GetValuesByParamRule returns Values generated by the specified param_rule
	GetValuesByParamRule(paramRuleName string) []*Value
}

// valueReference is embedded in ValueOwner implementations to provide
// common Value management functionality
type valueReference struct {
	values []*Value
}

func newValueReference() *valueReference {
	return &valueReference{
		values: []*Value{},
	}
}

// AddValue adds a Value to the reference
func (vr *valueReference) AddValue(v *Value) {
	vr.values = append(vr.values, v)
}

// GetValues returns all Values
func (vr *valueReference) GetValues() []*Value {
	return vr.values
}

// GetValuesByParamRule returns Values generated by the specified param_rule
func (vr *valueReference) GetValuesByParamRule(paramRuleName string) []*Value {
	result := []*Value{}
	for _, v := range vr.values {
		if v.ParamRuleName == paramRuleName {
			result = append(result, v)
		}
	}
	return result
}

// addressOwner includes Node, Interface
// commented out because it currently does not have abstracted usage (explicitly addressed)
// type addressOwner interface {
// 	setAware(string)
// 	IsAware(string) bool
// }

// layerAwareObject holds Layer (IP address space) awareness and IP policies.
// Embedded by Node, Interface, and Connection.
type layerAwareObject struct {
	layerPolicy map[string]*IPPolicy
	layers      mapset.Set[string]
}

func newLayerAwareObject() layerAwareObject {
	return layerAwareObject{
		layerPolicy: map[string]*IPPolicy{},
		layers:      mapset.NewSet[string](),
	}
}

func (a layerAwareObject) AwareLayer(layer string) bool {
	return a.layers.Contains(layer)
}

func (a layerAwareObject) GetLayerPolicy(layer string) *IPPolicy {
	val, ok := a.layerPolicy[layer]
	if ok {
		return val
	} else {
		return nil
	}
}

func (a layerAwareObject) setPolicy(layer *Layer, policy *IPPolicy) {
	a.layerPolicy[layer.Name] = policy
	a.layers.Add(layer.Name)
}

// configFileOwner includes Network and Node
// configFileOwner can generate configuration file in corresponding granuralities
// currently there is no abstracted functions

// type configFileOwner interface {
// }
//
// type configFileGenerator struct {
// 	Files *ConfigFiles
// }

// meta structures

// type classMemberMapper interface {
// 	addClassMember(string, *Node)
// 	hasClassMember(string) bool
// 	getClassMembers(string) []*Node
// }

type classMemberMap struct {
	mapper map[string][]NameSpacer
}

func (m classMemberMap) addClassMember(name string, ns NameSpacer) {
	m.mapper[name] = append(m.mapper[name], ns)
}

func (m classMemberMap) hasClassMember(name string) bool {
	_, ok := m.mapper[name]
	return ok
}

func (m classMemberMap) getClassMembers(name string) []NameSpacer {
	if m.hasClassMember(name) {
		return m.mapper[name]
	} else {
		return []NameSpacer{}
	}
}

// instance structures

type NetworkModel struct {
	Name        string
	Nodes       []*Node
	Connections []*Connection
	Groups      []*Group
	Classes     []*NetworkClass

	*NameSpace
	*valueReference
	// configFileGenerator

	NetworkSegments map[string][]*NetworkSegment
	//Files           *ConfigFiles

	nodeMap  map[string]*Node
	groupMap map[string]*Group
	// Class member maps back the "classmembers" (MemberClass) feature: a class can
	// pull in every object belonging to another class. MemberClass.GetSpecifiedClasses
	// only accepts node, interface and connection classes, so there are exactly three
	// maps here. Group and segment classes have none on purpose - adding one would be
	// dead weight until MemberClass learns to reference them.
	nodeClassMemberMap       classMemberMap
	interfaceClassMemberMap  classMemberMap
	connectionClassMemberMap classMemberMap
}

// Interfaces implemented by NetworkModel. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var (
	_ NameSpacer    = (*NetworkModel)(nil)
	_ FileGenerator = (*NetworkModel)(nil)
	_ ValueOwner    = (*NetworkModel)(nil)
)

func NewNetworkModel() *NetworkModel {
	nm := &NetworkModel{
		NetworkSegments: map[string][]*NetworkSegment{},
		//Files:                    newConfigFiles(),
		NameSpace:                newNameSpace(),
		valueReference:           newValueReference(),
		nodeMap:                  map[string]*Node{},
		groupMap:                 map[string]*Group{},
		nodeClassMemberMap:       classMemberMap{mapper: map[string][]NameSpacer{}},
		interfaceClassMemberMap:  classMemberMap{mapper: map[string][]NameSpacer{}},
		connectionClassMemberMap: classMemberMap{mapper: map[string][]NameSpacer{}},
	}
	return nm
}

func (nm *NetworkModel) SortKey() string {
	return nm.Name
}

func (nm *NetworkModel) NewNode(name string) *Node {
	node := newNode(name)
	// node := &Node{
	// 	Name:            name,
	// 	NameSpace:       newNameSpace(),
	// 	layerAwareObject: newLayerAwareObject(),
	// 	interfaceMap:    map[string]*Interface{},
	// 	memberReference: newMemberReference(),
	// }
	nm.Nodes = append(nm.Nodes, node)
	nm.nodeMap[name] = node

	return node
}

func (nm *NetworkModel) NewConnection(src *Interface, dst *Interface) *Connection {
	conn := newConnection(src, dst)
	// conn := &Connection{
	// 	Src:    src,
	// 	Dst:    dst,
	// 	Layers: mapset.NewSet[string](),
	// }
	nm.Connections = append(nm.Connections, conn)
	src.Connection = conn
	dst.Connection = conn
	return conn
}

func (nm *NetworkModel) NewGroup(name string) *Group {
	group := newGroup(name)
	// group := &Group{
	// 	Name:      name,
	// 	Nodes:     []*Node{},
	// 	NameSpace: newNameSpace(),
	// }
	nm.Groups = append(nm.Groups, group)
	nm.groupMap[name] = group

	return group
}

func (nm *NetworkModel) RenameNode(node *Node, oldName string, newName string) {
	if oldName != "" {
		delete(nm.nodeMap, oldName)
	}
	nm.nodeMap[newName] = node
}

func (nm *NetworkModel) StringForMessage() string {
	return fmt.Sprintf("network:%s", nm.Name)
}

func (nm *NetworkModel) NodeByName(name string) (*Node, bool) {
	node, ok := nm.nodeMap[name]
	return node, ok
}

func (nm *NetworkModel) GroupByName(name string) (*Group, bool) {
	group, ok := nm.groupMap[name]
	return group, ok
}

func (nm *NetworkModel) NodeClassMembers(cls string) []NameSpacer {
	return nm.nodeClassMemberMap.getClassMembers(cls)
}

func (nm *NetworkModel) InterfaceClassMembers(cls string) []NameSpacer {
	return nm.interfaceClassMemberMap.getClassMembers(cls)
}

func (nm *NetworkModel) ConnectionClassMembers(cls string) []NameSpacer {
	return nm.connectionClassMemberMap.getClassMembers(cls)
}

func (nm *NetworkModel) ChildClasses() ([]string, error) {
	return []string{ClassTypeNode, ClassTypeGroup, ClassTypeConnection, ClassTypeSegment}, nil
}

func (nm *NetworkModel) DependClasses() ([]string, error) {
	return nm.ChildClasses()
}

func (nm *NetworkModel) Childs(c string) ([]NameSpacer, error) {
	switch c {
	case ClassTypeNode:
		var nodes []NameSpacer
		for _, n := range nm.Nodes {
			nodes = append(nodes, n)
		}
		return nodes, nil
	case ClassTypeGroup:
		var groups []NameSpacer
		for _, g := range nm.Groups {
			groups = append(groups, g)
		}
		return groups, nil
	case ClassTypeConnection:
		var connections []NameSpacer
		for _, conn := range nm.Connections {
			connections = append(connections, conn)
		}
		return connections, nil
	case ClassTypeSegment:
		var segments []NameSpacer
		for _, segmentList := range nm.NetworkSegments {
			for _, seg := range segmentList {
				segments = append(segments, seg)
			}
		}
		return segments, nil
	default:
		return nil, fmt.Errorf("invalid class type %s for networkModel.Childs()", c)
	}
}

func (nm *NetworkModel) Depends(c string) ([]NameSpacer, error) {
	return nm.Childs(c)
}

// func (nm *NetworkModel) traceChilds(target NameSpacer) []NameSpacer {
// 	// 常に子供が親より前に来るようにする -> 子供(とそのさらに子供)を追加して、最後に自分を追加
// 	var childs []NameSpacer
// 	classes, err := target.ChildClasses()
// 	if err != nil {
// 		return nil
// 	}
// 	for _, cls := range classes {
// 		objs, err := target.Childs(cls)
// 		if err != nil {
// 			return nil
// 		}
// 		for _, obj := range objs {
// 			childs = append(childs, nm.traceChilds(obj)...)
// 		}
// 	}
// 	return childs
// }

func (nm *NetworkModel) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, nc := range cfg.NetworkClasses {
		configTemplates = append(configTemplates, nc.ConfigTemplates...)
	}
	return configTemplates
}

func (nm *NetworkModel) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	return nm.GetConfigTemplates(cfg)
}

func (nm *NetworkModel) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {
	// global params (place lanels)
	setGlobalParams(nm, globalParams)

	// self
	for key, val := range nm.GetParams() {
		nm.SetRelativeParam(key, val)
	}

	return nil
}

// func (nm *NetworkModel) PostOrderTraversal() []NameSpacer {
// 	var traverse func(target NameSpacer) []NameSpacer
// 	traverse = func(target NameSpacer) []NameSpacer {
// 		var result []NameSpacer
// 		classes, err := target.ChildClasses()
// 		if err != nil {
// 			return nil
// 		}
// 		for _, cls := range classes {
// 			objs, err := target.Childs(cls)
// 			if err != nil {
// 				return nil
// 			}
// 			for _, obj := range objs {
// 				result = append(result, traverse(obj)...)
// 			}
// 		}
// 		result = append(result, target)
// 		return result
// 	}
//
// 	return traverse(nm)
// }

func (nm *NetworkModel) NameSpacers() (result []NameSpacer) {
	// Network, Node, Interface, Connection, Neighbor, Member, Group

	// network
	result = append(result, nm)
	// node
	for _, n := range nm.Nodes {
		result = append(result, n)
		// iface
		for _, iface := range n.Interfaces {
			result = append(result, iface)
			// neighbor
			for _, neighbors := range iface.Neighbors {
				for _, neighbor := range neighbors {
					result = append(result, neighbor)
				}
			}
		}
	}
	// connection
	for _, conn := range nm.Connections {
		result = append(result, conn)
	}
	for _, segs := range nm.NetworkSegments {
		for _, seg := range segs {
			result = append(result, seg)
		}
	}
	// member
	for _, mr := range nm.MemberReferrers() {
		for _, m := range mr.GetMembers() {
			result = append(result, m)
		}
	}
	// group
	for _, g := range nm.Groups {
		result = append(result, g)
	}
	// values (from all ValueOwners)
	for _, vo := range nm.ValueOwners() {
		for _, v := range vo.GetValues() {
			result = append(result, v)
		}
	}
	return result
}

func (nm *NetworkModel) LabelOwners() (result []LabelOwner) {
	// TODO: consider orders of iteration
	// current order: connection, interface, node, group

	for _, conn := range nm.Connections {
		result = append(result, conn)
	}
	for _, n := range nm.Nodes {
		for _, iface := range n.Interfaces {
			result = append(result, iface)
		}
		result = append(result, n)
	}
	for _, g := range nm.Groups {
		result = append(result, g)
	}
	return result
}

func (nm *NetworkModel) MemberReferrers() (result []MemberReferrer) {
	for _, n := range nm.Nodes {
		result = append(result, n)
		for _, iface := range n.Interfaces {
			result = append(result, iface)
		}
	}
	for _, conn := range nm.Connections {
		result = append(result, conn)
	}
	for _, segments := range nm.NetworkSegments {
		for _, seg := range segments {
			result = append(result, seg)
		}
	}
	return result
}

// ValueOwners returns all objects that can have Values attached
func (nm *NetworkModel) ValueOwners() (result []ValueOwner) {
	// NetworkModel itself
	result = append(result, nm)
	// Nodes
	for _, n := range nm.Nodes {
		result = append(result, n)
		// Interfaces
		for _, iface := range n.Interfaces {
			result = append(result, iface)
		}
	}
	// Connections
	for _, conn := range nm.Connections {
		result = append(result, conn)
	}
	// Groups
	for _, g := range nm.Groups {
		result = append(result, g)
	}
	// Segments
	for _, segments := range nm.NetworkSegments {
		for _, seg := range segments {
			result = append(result, seg)
		}
	}
	return result
}

// FilesToGenerate returns a list of file names that the network will generate based on its NetworkClasses.
func (nm *NetworkModel) FilesToGenerate(cfg *Config) []string {
	fileSet := make(map[string]bool)
	// Collect all file names from NetworkClass ConfigTemplates
	for _, nc := range cfg.NetworkClasses {
		for _, ct := range nc.ConfigTemplates {
			if ct.File != "" {
				fileSet[ct.File] = true
			}
		}
	}
	// Convert to slice
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

//func (nm *NetworkModel) StringAllObjectClasses(cfg *Config) string {
//	// network class
//	classNames := []string{}
//	for _, cls := range nm.Classes {
//		classNames = append(classNames, cls.Name)
//	}
//	ret := []string{
//		"Object Classes:",
//		//fmt.Sprintf(" %s: %v %v", nm.StringForMessage(), classNames, classes),
//		fmt.Sprintf(" %s: %v", nm.StringForMessage(), classNames),
//	}
//	// LabelOwners
//	//for _, o := range nm.LabelOwners() {
//	//	//ret = append(ret, fmt.Sprintf(" %s %v %v", o.StringForMessage(), o.ClassLabels(), o.GetClasses()))
//	//	ret = append(ret, fmt.Sprintf(" %s %v", o.StringForMessage(), o.ClassLabels()))
//	//}
//	return strings.Join(ret, "\n") + "\n"
//}

type Node struct {
	Name       string
	Interfaces []*Interface
	Groups     []*Group
	//Virtual    bool
	// Files      *ConfigFiles

	*NameSpace
	*ParsedLabels
	*memberReference
	*valueReference
	layerAwareObject
	// configFileGenerator

	NamePrefix string

	mgmtInterface      *Interface
	mgmtInterfaceClass *InterfaceClass
	interfaceMap       map[string]*Interface
}

// Interfaces implemented by Node. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var (
	_ NameSpacer     = (*Node)(nil)
	_ LabelOwner     = (*Node)(nil)
	_ FileGenerator  = (*Node)(nil)
	_ MemberReferrer = (*Node)(nil)
	_ ValueOwner     = (*Node)(nil)
)

func newNode(name string) *Node {
	node := &Node{
		Name:             name,
		NameSpace:        newNameSpace(),
		layerAwareObject: newLayerAwareObject(),
		interfaceMap:     map[string]*Interface{},
		memberReference:  newMemberReference(),
		valueReference:   newValueReference(),
	}
	return node
}

func (n *Node) SortKey() string {
	return n.Name
}

func (n *Node) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	n.ParsedLabels = cfg.GetValidNodeClasses(labels)
	n.ParsedLabels.classLabels = append(n.ParsedLabels.classLabels, moduleLabels...)
	// Module-provided classes are the weakest tier (see ClassTier* above):
	// they supply defaults and anything the user writes overrides them.
	for _, name := range moduleLabels {
		n.ParsedLabels.setClassTier(name, ClassTierModule)
	}
	if err := n.resolveClasses(cfg); err != nil {
		return err
	}
	return nil
}

// resolveClasses turns the class labels into class definitions. It runs both from
// SetLabels and at the start of SetClasses, because labels can still be added in
// between: AddClassLabels for relational classes, and modules through the
// ObjectClassifier hook. Resolving only in SetLabels would silently drop those.
// recordPolicy resolves which IP policy wins for a layer when several classes
// name one, by the same tier rule the values follow. Applying them as they are
// visited would let the last class win, and classes are visited strongest
// first, so the weakest one would have taken the layer.
//
// It reports whether policyName names a policy at all: the parameter lists mix
// policy names with plain parameter flags, and the caller tells them apart by
// this.
func recordPolicy(t *tieredValues, cfg *Config, policyName string, tier int) (isPolicy bool, other string, ok bool) {
	policy, found := cfg.policyMap[policyName]
	if !found {
		return false, "", true
	}
	other, ok = t.set(policy.layer.Name, policyName, tier)
	return true, other, ok
}

// applyPolicies hands the winning policies to the object.
func applyPolicies(t *tieredValues, cfg *Config, set func(*Layer, *IPPolicy)) {
	for _, policyName := range t.values {
		policy := cfg.policyMap[policyName]
		set(policy.layer, policy)
	}
}

// recordConfigNames reports two classes on the same object defining a config
// template under the same name. Left to config generation, the clash surfaces
// as a duplicated namespace parameter that names neither class - hard to trace
// when one of them arrived through use:.
// configNameOrigin remembers who defined a config template name on an object,
// and whether that was a module - which is what decides if a second definition
// of the same name is a meeting or a collision.
type configNameOrigin struct {
	className      string
	moduleProvided bool
}

func recordConfigNames(seen map[string]configNameOrigin, objType, objName, className string, cts []*ConfigTemplate) error {
	for _, ct := range cts {
		if ct.Name == "" {
			continue
		}
		prev, ok := seen[ct.Name]
		if !ok {
			seen[ct.Name] = configNameOrigin{className: className, moduleProvided: ct.ModuleProvided}
			continue
		}
		// A hook name is the one place a module and the topology are meant to
		// meet: the module adds what it has to do, the topology says what it
		// wants, and the two are merged in that order. Two topology classes
		// naming one hook is still the accident this check is here for.
		if HookConfigNames[ct.Name] && (ct.ModuleProvided || prev.moduleProvided) {
			// Keep the topology's own class in the record, so that a third
			// definition from the topology is still reported against it.
			if prev.moduleProvided && !ct.ModuleProvided {
				seen[ct.Name] = configNameOrigin{className: className}
			}
			continue
		}
		other := prev.className
		return fmt.Errorf(
			"%s %s: classes %s and %s both define a config template named %q. "+
				"use: attaches a class rather than letting it be overridden, so rename one of them "+
				"or drop the use:",
			objType, objName, other, className, ct.Name)
	}
	return nil
}

// expandUsedClasses attaches the classes that the already-attached ones name in
// their use: list, transitively. A used class is attached exactly as if it had
// been listed alongside, so composition follows the ordinary multi-class rules:
// nothing is merged or overridden here.
//
// The tier comes from where a class was defined, not from the class that named
// it. A module's class stays the weakest even when a user's class pulls it in,
// which is what lets the user's own values win over the module's defaults.
func expandUsedClasses(l *ParsedLabels, classType string, byName func(string) (ComposableClass, bool), ignoreUndefined bool) error {
	attached := map[string]bool{}
	for _, name := range l.ClassLabels() {
		attached[name] = true
	}
	// Breadth-first over the labels attached so far; attached[] doubles as the
	// visited set, so a cycle in use: terminates instead of looping.
	queue := append([]string{}, l.ClassLabels()...)
	for len(queue) > 0 {
		def, ok := byName(queue[0])
		namedBy := queue[0]
		queue = queue[1:]
		if !ok {
			// An undefined label is reported by the caller's own resolution.
			continue
		}
		for _, used := range def.UsedClasses() {
			if attached[used] {
				continue
			}
			usedDef, ok := byName(used)
			if !ok {
				if ignoreUndefined {
					continue
				}
				return fmt.Errorf("%sclass %s uses undefined %sclass %s", classType, namedBy, classType, used)
			}
			attached[used] = true
			if usedDef.IsModuleProvided() {
				l.AddModuleClassLabels(used)
			} else {
				l.AddClassLabels(used)
			}
			queue = append(queue, used)
		}
	}
	return nil
}

func (n *Node) resolveClasses(cfg *Config) error {
	if err := expandUsedClasses(n.ParsedLabels, ClassTypeNode, func(name string) (ComposableClass, bool) {
		def, ok := cfg.NodeClassByName(name)
		return def, ok
	}, cfg.GlobalSettings.IgnoreUndefinedClass); err != nil {
		return err
	}
	return n.resolveClassDefinitions(cfg)
}

func (n *Node) resolveClassDefinitions(cfg *Config) error {
	n.ParsedLabels.Classes = []ObjectClass{}
	for _, cls := range n.ClassLabels() {
		def, ok := cfg.NodeClassByName(cls)
		if !ok {
			if cfg.GlobalSettings.IgnoreUndefinedClass {
				continue
			}
			return fmt.Errorf("invalid nodeclass name %s", cls)
		}
		n.ParsedLabels.Classes = append(n.ParsedLabels.Classes, def)
	}
	return nil
}
func (n *Node) SetClasses(cfg *Config, nm *NetworkModel) error {
	if err := n.resolveClasses(cfg); err != nil {
		return err
	}
	// Resolve attributes contributed by several classes: the strongest class to
	// set a key wins, and only a clash inside one tier is an error (see
	// tieredValues).
	values := newTieredValues()
	single := newTieredValues()
	nodePolicies := newTieredValues()
	ifacePolicies := newTieredValues()
	configNames := map[string]configNameOrigin{}

	// set defaults for nodes without class
	n.NamePrefix = DefaultNodePrefix

	for _, cls := range n.GetClasses() {
		nc := cls.(*NodeClass)
		// nc, ok := cfg.NodeClassByName(cls)
		// if !ok {
		// 	return fmt.Errorf("invalid nodeclass name %s", cls)
		// }
		// n.ParsedLabels.Classes = append(n.ParsedLabels.Classes, nc)
		nm.nodeClassMemberMap.addClassMember(nc.Name, n)
		if err := recordConfigNames(configNames, "node", n.Name, nc.Name, nc.ConfigTemplates); err != nil {
			return err
		}

		// check virtual
		if nc.Virtual {
			n.SetVirtual(true)
		}

		tier := n.ClassTier(nc.Name)

		// check ippolicy flags
		for _, p := range nc.IPPolicy {
			isPolicy, other, ok := recordPolicy(nodePolicies, cfg, p, tier)
			if !isPolicy {
				return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
			}
			if !ok {
				return classConflictError("node", n.Name, "policy", other, p)
			}
		}

		// check interface_policy flags
		for _, p := range nc.InterfaceIPPolicy {
			isPolicy, other, ok := recordPolicy(ifacePolicies, cfg, p, tier)
			if !isPolicy {
				return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
			}
			if !ok {
				return classConflictError("node", n.Name, "interface policy", other, p)
			}
		}

		// check parameter flags
		for _, num := range nc.Parameters {
			isPolicy, other, ok := recordPolicy(nodePolicies, cfg, num, tier)
			if !isPolicy {
				n.setParamFlag(num)
				continue
			}
			if !ok {
				return classConflictError("node", n.Name, "policy", other, num)
			}
		}

		// check MemberClasses
		for i := range nc.MemberClasses {
			n.AddMemberClass(nc.MemberClasses[i])
		}

		// Check for value conflicts
		for key, value := range nc.Values {
			if other, ok := values.set(key, value, tier); !ok {
				return classConflictError("node", n.Name, "values for '"+key+"'", other, value)
			}
		}

		// Check for prefix conflicts (only if both are non-empty and different)
		if nc.Prefix != "" {
			if other, ok := single.set("prefix", nc.Prefix, tier); !ok {
				return classConflictError("node", n.Name, "prefix", other, nc.Prefix)
			}
		}

		// Check for mgmt interface conflicts (only if both are non-empty and different)
		if nc.MgmtInterface != "" {
			if other, ok := single.set("mgmt_interfaceclass", nc.MgmtInterface, tier); !ok {
				return classConflictError("node", n.Name, "management interface class", other, nc.MgmtInterface)
			}
		}

	}

	applyPolicies(nodePolicies, cfg, n.setPolicy)
	applyPolicies(ifacePolicies, cfg, func(l *Layer, p *IPPolicy) {
		for _, iface := range n.Interfaces {
			iface.setPolicy(l, p)
		}
	})

	// What the node is materialised as is settled once every class has been
	// applied, by resolveDeployForms, because it also decides the form of the
	// connections and interfaces around it. Only the value is validated here, so
	// that a bad one is reported against the node that carries the class.
	if _, err := cfg.ResolveDeploy(n); err != nil {
		return err
	}

	// Apply the winning single-valued attributes. Assigning after the loop (rather
	// than on every class) is what makes the tier order authoritative.
	if prefix, ok := single.get("prefix"); ok {
		n.NamePrefix = prefix
	}
	if name, ok := single.get("mgmt_interfaceclass"); ok {
		mgmtnc, found := cfg.InterfaceClassByName(name)
		if !found {
			return fmt.Errorf("invalid mgmt interface class name %s", name)
		}
		n.mgmtInterfaceClass = mgmtnc
	}

	return nil
}

func (n *Node) String() string {
	return n.Name
}

func (n *Node) StringForMessage() string {
	return fmt.Sprintf("node:%s", n.Name)
}

// AdoptInterface moves an interface here from the node currently holding it,
// leaving the wire it belongs to untouched. Splitting a shared medium across
// machines needs exactly this: the interface facing a member has to end up on
// the replica that stands on that member's machine.
func (n *Node) AdoptInterface(iface *Interface) {
	from := iface.Node
	if from == n {
		return
	}
	if from != nil {
		for i, held := range from.Interfaces {
			if held == iface {
				from.Interfaces = append(from.Interfaces[:i], from.Interfaces[i+1:]...)
				break
			}
		}
		if iface.Name != "" {
			delete(from.interfaceMap, iface.Name)
		}
	}
	iface.Node = n
	n.Interfaces = append(n.Interfaces, iface)
	if iface.Name != "" {
		n.interfaceMap[iface.Name] = iface
	}
}

func (n *Node) NewInterface(name string) *Interface {
	iface := newInterface(n, name)
	// iface := &Interface{
	// 	Name:             name,
	// 	Node:             n,
	// 	Neighbors:        map[string][]*Neighbor{},
	// 	NameSpace:        newNameSpace(),
	// 	layerAwareObject:  newLayerAwareObject(),
	// 	memberReference:  newMemberReference(),
	// 	hasNeighborClass: map[string]bool{},
	// }
	n.Interfaces = append(n.Interfaces, iface)
	if name != "" {
		n.interfaceMap[iface.Name] = iface
	}
	return iface
}

func (n *Node) InterfaceByName(name string) (*Interface, bool) {
	iface, ok := n.interfaceMap[name]
	return iface, ok
}

func (n *Node) RenameInterface(iface *Interface, oldName string, newName string) {
	if oldName != "" {
		delete(n.interfaceMap, oldName)
	}
	n.interfaceMap[newName] = iface
}

func (n *Node) CreateManagementInterface(cfg *Config, name string) (*Interface, error) {
	ic := n.mgmtInterfaceClass
	if ic == nil {
		return nil, fmt.Errorf("mgmt InterfaceClass is not appropriately specified")
	} else {
		// check that mgmtInterfaceClass is not used in topology
		for _, iface := range n.Interfaces {
			for _, cls := range iface.ClassLabels() {
				if cls == ic.Name {
					return nil, fmt.Errorf("mgmt InterfaceClass should not be specified in topology graph (automatically added)")
				}
			}
		}

		// add management interface
		iface := n.NewInterface(name)
		if err := iface.SetLabels(cfg, []string{ic.Name}, []string{}); err != nil {
			return nil, err
		}
		iface.ParsedLabels.Classes = append(iface.ParsedLabels.Classes, ic)
		// iface.parsedLabels = newParsedLabels()
		// iface.parsedLabels.classLabels = append(iface.parsedLabels.classLabels, ic.Name)
		n.mgmtInterface = iface
		return iface, nil
	}
}

func (n *Node) GetManagementInterface() *Interface {
	return n.mgmtInterface
}

func (n *Node) ChildClasses() ([]string, error) {
	classes := []string{ClassTypeInterface}
	for _, mc := range n.GetMemberClasses() {
		classType, classNames, err := mc.GetSpecifiedClasses()
		if err != nil {
			return nil, err
		}
		for _, cn := range classNames {
			classes = append(classes, ClassTypeMember(classType, cn))
		}
	}
	return classes, nil
}

func (n *Node) Childs(c string) ([]NameSpacer, error) {
	objs := []NameSpacer{}
	tmp := strings.SplitN(c, "_", 3) // Maximum 3 splits for Member

	switch tmp[0] {
	case ClassTypeInterface:
		for _, i := range n.Interfaces {
			objs = append(objs, i)
		}
		return objs, nil
	case ClassTypeMemberHeader:
		if len(tmp) < 3 {
			return nil, fmt.Errorf("invalid member reference %q (expected <header>_<classType>_<className>)", c)
		}
		classType := tmp[1]
		className := tmp[2]
		for _, m := range n.GetMembers() {
			if m.ClassType == classType && m.ClassName == className {
				objs = append(objs, m)
			}
		}
		if len(objs) == 0 {
			return nil, fmt.Errorf("no child objects that match %s", c)
		}
		return objs, nil
	default:
		return nil, fmt.Errorf("invalid class type %s for node.Childs()", c)
	}
}

func (n *Node) DependClasses() ([]string, error) {
	return n.ChildClasses()
}

func (n *Node) Depends(c string) ([]NameSpacer, error) {
	return n.Childs(c)
}

func (n *Node) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, cls := range n.GetClasses() {
		nc := cls.(*NodeClass)
		configTemplates = append(configTemplates, nc.ConfigTemplates...)
	}
	return configTemplates
}

func (n *Node) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	cts := []*ConfigTemplate{}
	for _, nc := range cfg.NodeClasses {
		cts = append(cts, nc.ConfigTemplates...)
	}
	return cts
}

// func (n *Node) setAwareLayers(aware []string, defaults []string, ignoreDefaults bool) {
// 	var givenset mapset.Set[string]
// 	var defaultset mapset.Set[string]
// 	if ignoreDefaults {
// 		defaultset = mapset.NewSet[string]()
// 	} else {
// 		defaultset = mapset.NewSet(defaults...)
// 	}
// 	givenset = mapset.NewSet(aware...)
//
// 	n.layerAwareObject.AwareLayers = defaultset.Union(givenset)
// }

func (n *Node) HasAwareInterface(layer string) bool {
	for _, iface := range n.Interfaces {
		if iface.AwareLayer(layer) {
			return true
		}
	}
	return false
}

func (n *Node) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	nc, ok := cfg.nodeClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid NodeClass name %s", cls)
	}
	return nc, nil
}

func (n *Node) GivenIPLoopback(layer *Layer) (string, bool) {
	for k, v := range n.valueLabels {
		if k == layer.IPLoopbackReplacer() {
			return v, true
		}
	}
	return "", false
}

func (n *Node) setNodeBaseRelativeNameSpace(
	ns NameSpacer, globalParams map[string]map[string]string, header string) error {
	// self
	for k, val := range n.GetParams() {
		key := header + k
		ns.SetRelativeParam(key, val)
	}

	// group params
	for _, group := range n.Groups {
		group.SetGroupRelativeParams(ns, header)
	}

	// meta value labels
	return setMetaValueLabelNameSpace(ns, n, globalParams, header)
}

func (n *Node) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {
	// global params (place lanels)
	setGlobalParams(n, globalParams)

	// base params
	return n.setNodeBaseRelativeNameSpace(n, globalParams, "")
}

// OutputPath returns where filedef's file for this node is written, relative to
// the output root and always slash-separated. It is the single source of truth
// for that location: the file is written there, listed there, and referred to
// there by the bind mounts a platform module emits, and those three drifting
// apart is exactly how a bind ends up pointing at a file that is not there.
func (n *Node) OutputPath(cfg *Config, filedef *FileDefinition) (string, error) {
	dirname, err := n.OutputDir(cfg)
	if err != nil {
		return "", err
	}
	if filedef.GetOutputLocation() == "root" {
		return path.Join(dirname, filedef.Subdir, filedef.GetFileName(n.Name)), nil
	}
	dirname = path.Join(dirname, n.Name)
	// A file that names where it goes inside the container is laid out that way
	// on disk too: r1/etc/frr/frr.conf for /etc/frr/frr.conf. Kathara asks for
	// exactly this - it has no way to mount a single file, so what is mounted is
	// the directory - and it costs the others nothing, since their bind and
	// mount lines name the source path anyway.
	//
	// A file provided by copy waits under staging/ instead, mirroring the same
	// path: r1/staging/etc/motd for /etc/motd. It has to sit apart from the
	// mounted files, because the whole staging directory is mounted into the
	// container as one, and anything else under the node directory would ride
	// along into it.
	if filedef.Path != "" {
		if filedef.GetProvide() == ProvideCopy {
			dirname = path.Join(dirname, StagingDirName)
		}
		return path.Join(dirname, strings.TrimPrefix(filedef.Path, "/")), nil
	}
	return path.Join(dirname, filedef.GetFileName(n.Name)), nil
}

// StagedCopy is one file waiting in the staging directory, named as the
// container sees it: Source is where the mount puts it, Target where it belongs,
// Dir the directory Target sits in. The command that moves it is the platform's
// to write; what has to be copied where is not.
type StagedCopy struct {
	Source string
	Target string
	Dir    string
}

// StagedCopies lists the copies a node needs once its container is up, in the
// order the file definitions are written so that two runs produce one file.
func StagedCopies(cfg *Config, node *Node) ([]StagedCopy, error) {
	generated := make(map[string]bool)
	for _, name := range node.FilesToGenerate(cfg) {
		generated[name] = true
	}

	var copies []StagedCopy
	for _, filedef := range cfg.FileDefinitions {
		if filedef.Path == "" || !generated[filedef.Name] {
			continue
		}
		if filedef.GetProvide() != ProvideCopy {
			continue
		}
		target := filedef.Path
		copies = append(copies, StagedCopy{
			Source: path.Join("/"+StagingDirName, target),
			Target: target,
			Dir:    path.Dir(target),
		})
	}
	return copies, nil
}

// StagingDir returns where this node's copy-provided files wait, relative to the
// output root, and whether the node has any. The directory is what a module
// mounts into the container; the files inside it are copied to their own paths
// once it is up.
func (n *Node) StagingDir(cfg *Config) (string, bool, error) {
	staged := false
	for _, name := range n.FilesToGenerate(cfg) {
		filedef, ok := cfg.FileDefinitionByName(name)
		if !ok || filedef.Path == "" {
			continue
		}
		if filedef.GetProvide() == ProvideCopy {
			staged = true
			break
		}
	}
	if !staged {
		return "", false, nil
	}
	dirname, err := n.OutputDir(cfg)
	if err != nil {
		return "", false, err
	}
	return path.Join(dirname, n.Name, StagingDirName), true, nil
}

// FilesToGenerate returns a list of file names that the node will generate based on its classes.
// It examines NodeClass ConfigTemplates.
func (n *Node) FilesToGenerate(cfg *Config) []string {
	fileSet := make(map[string]bool)

	// Collect file names from NodeClass ConfigTemplates
	for _, classLabel := range n.ClassLabels() {
		for _, nc := range cfg.NodeClasses {
			if nc.Name == classLabel {
				for _, ct := range nc.ConfigTemplates {
					if ct.File != "" {
						fileSet[ct.File] = true
					}
				}
			}
		}
	}

	// Convert to slice
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

type Interface struct {
	Name       string
	Node       *Node
	Virtual    bool
	Connection *Connection
	Opposite   *Interface
	Neighbors  map[string][]*Neighbor
	NamePrefix string

	*NameSpace
	*ParsedLabels
	*memberReference
	*valueReference
	layerAwareObject

	//hasNeighborClass map[string]bool // key: layer
	neighborClassMap map[string][]*NeighborClass
}

// Interfaces implemented by Interface. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var (
	_ NameSpacer     = (*Interface)(nil)
	_ LabelOwner     = (*Interface)(nil)
	_ MemberReferrer = (*Interface)(nil)
	_ ValueOwner     = (*Interface)(nil)
)

func newInterface(node *Node, name string) *Interface {
	iface := &Interface{
		Name:             name,
		Node:             node,
		Neighbors:        map[string][]*Neighbor{},
		NameSpace:        newNameSpace(),
		layerAwareObject: newLayerAwareObject(),
		memberReference:  newMemberReference(),
		valueReference:   newValueReference(),
		// hasNeighborClass: map[string]bool{},
		neighborClassMap: map[string][]*NeighborClass{},
	}
	return iface
}

func (iface *Interface) SortKey() string {
	return iface.Name
}

func (iface *Interface) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	iface.ParsedLabels = cfg.GetValidInterfaceClasses(labels)
	iface.ParsedLabels.classLabels = append(iface.ParsedLabels.classLabels, moduleLabels...)
	// Module-provided classes are the weakest tier (see ClassTier* above):
	// they supply defaults and anything the user writes overrides them.
	for _, name := range moduleLabels {
		iface.ParsedLabels.setClassTier(name, ClassTierModule)
	}
	if err := iface.resolveClasses(cfg); err != nil {
		return err
	}
	return nil
}

// resolveClasses turns the class labels into class definitions. It runs both from
// SetLabels and at the start of SetClasses, because labels can still be added in
// between: AddClassLabels for relational classes, and modules through the
// ObjectClassifier hook. Resolving only in SetLabels would silently drop those.
func (iface *Interface) resolveClasses(cfg *Config) error {
	if err := expandUsedClasses(iface.ParsedLabels, ClassTypeInterface, func(name string) (ComposableClass, bool) {
		def, ok := cfg.InterfaceClassByName(name)
		return def, ok
	}, cfg.GlobalSettings.IgnoreUndefinedClass); err != nil {
		return err
	}
	return iface.resolveClassDefinitions(cfg)
}

func (iface *Interface) resolveClassDefinitions(cfg *Config) error {
	iface.ParsedLabels.Classes = []ObjectClass{}
	for _, cls := range iface.ClassLabels() {
		def, ok := cfg.InterfaceClassByName(cls)
		if !ok {
			if cfg.GlobalSettings.IgnoreUndefinedClass {
				continue
			}
			return fmt.Errorf("invalid interfaceclass name %s", cls)
		}
		iface.ParsedLabels.Classes = append(iface.ParsedLabels.Classes, def)
	}
	return nil
}
func (iface *Interface) SetClasses(cfg *Config, nm *NetworkModel) error {
	if err := iface.resolveClasses(cfg); err != nil {
		return err
	}
	// Track conflicting values
	// Resolve attributes contributed by several classes (see tieredValues).
	values := newTieredValues()
	single := newTieredValues()

	// A virtual node no longer makes its interfaces virtual. virtual withholds
	// the configuration of the object it is written on and nothing more; that
	// the interfaces of a node nobody deploys are not there either is a fact
	// about deployment, and resolveDeployForms is where it is worked out.

	// set defaults for interfaces without class
	iface.NamePrefix = DefaultInterfacePrefix
	configNames := map[string]configNameOrigin{}
	// Connection and interface classes are resolved separately, and the
	// interface's own classes are applied last so that they stay the more
	// specific of the two. Tiers only order classes of the same kind.
	connPolicies := newTieredValues()
	ifacePolicies := newTieredValues()

	// check connectionclass flags
	for _, cls := range iface.Connection.ClassLabels() {
		cc, ok := cfg.ConnectionClassByName(cls)
		if !ok {
			return fmt.Errorf("invalid connectionclass name %s", cls)
		}
		nm.connectionClassMemberMap.addClassMember(cc.Name, iface)
		if err := recordConfigNames(configNames, "interface", iface.conflictName(), cc.Name, cc.ConfigTemplates); err != nil {
			return err
		}

		// check virtual
		// NOTE: a virtual connectionclass no longer marks the interface virtual.
		// "virtual" applies to the object it is attached to: a connection that is
		// not a real link can still have real interfaces at its ends.

		ccTier := iface.Connection.ClassTier(cc.Name)

		// check ippolicy flags
		for _, p := range cc.IPPolicy {
			isPolicy, other, ok := recordPolicy(connPolicies, cfg, p, ccTier)
			if !isPolicy {
				return fmt.Errorf("invalid policy name %s in connectionclass %s", p, cc.Name)
			}
			if !ok {
				return classConflictError("interface", iface.conflictName(), "policy", other, p)
			}
		}

		// check parameter flags
		for _, num := range cc.Parameters {
			isPolicy, other, ok := recordPolicy(connPolicies, cfg, num, ccTier)
			if !isPolicy {
				iface.setParamFlag(num)
				continue
			}
			if !ok {
				return classConflictError("interface", iface.conflictName(), "policy", other, num)
			}
		}

		// check MemberClasses
		for i := range cc.MemberClasses {
			iface.AddMemberClass(cc.MemberClasses[i])
		}

	}

	// Re-resolve: the loop above can add interface classes through the relational
	// class labels of the connection. resolveClasses reports undefined classes
	// instead of silently skipping them, which the previous inline rebuild did.
	if err := iface.resolveClasses(cfg); err != nil {
		return err
	}

	// check interfaceclass flags
	for _, cls := range iface.GetClasses() {
		ic := cls.(*InterfaceClass)
		// for _, cls := range iface.classLabels {
		// 	ic, ok := cfg.interfaceClassMap[cls]
		// 	if !ok {
		// 		return fmt.Errorf("invalid interfaceclass name %s", cls)
		// 	}
		// 	iface.ParsedLabels.Classes = append(iface.ParsedLabels.Classes, ic)
		nm.interfaceClassMemberMap.addClassMember(ic.Name, iface)
		if err := recordConfigNames(configNames, "interface", iface.conflictName(), ic.Name, ic.ConfigTemplates); err != nil {
			return err
		}

		// check virtual
		if ic.Virtual {
			iface.SetVirtual(true)
		}
		//iface.Virtual = iface.Virtual || ic.Virtual

		icTier := iface.ClassTier(ic.Name)

		// check ippolicy flags
		for _, p := range ic.IPPolicy {
			isPolicy, other, ok := recordPolicy(ifacePolicies, cfg, p, icTier)
			if !isPolicy {
				return fmt.Errorf("invalid policy name %s in interfaceclass %s", p, ic.Name)
			}
			if !ok {
				return classConflictError("interface", iface.conflictName(), "policy", other, p)
			}
		}

		// check parameter flags
		for _, num := range ic.Parameters {
			isPolicy, other, ok := recordPolicy(ifacePolicies, cfg, num, icTier)
			if !isPolicy {
				iface.setParamFlag(num)
				continue
			}
			if !ok {
				return classConflictError("interface", iface.conflictName(), "policy", other, num)
			}
		}

		// check neighbor classes
		for _, nc := range ic.NeighborClasses {
			iface.neighborClassMap[nc.Layer] = append(iface.neighborClassMap[nc.Layer], nc)
			// iface.hasNeighborClass[nc.Layer] = true
		}

		// check MemberClasses
		for i := range ic.MemberClasses {
			iface.AddMemberClass(ic.MemberClasses[i])
		}

		// check layers - add to connection if connection exists
		if iface.Connection != nil {
			for _, layer := range ic.Layers {
				iface.Connection.Layers.Add(layer)
			}
		}

		// Check for value conflicts
		tier := iface.ClassTier(ic.Name)
		for key, value := range ic.Values {
			if other, ok := values.set(key, value, tier); !ok {
				return classConflictError("interface", iface.conflictName(),
					"values for '"+key+"'", other, value)
			}
		}

		// Check for prefix conflicts (only if both are non-empty and different)
		if ic.Prefix != "" {
			if other, ok := single.set("prefix", ic.Prefix, tier); !ok {
				return classConflictError("interface", iface.conflictName(),
					"prefix", other, ic.Prefix)
			}
		}
	}

	// Connection classes first, so that the interface's own classes take the
	// layer when both name a policy for it.
	applyPolicies(connPolicies, cfg, iface.setPolicy)
	applyPolicies(ifacePolicies, cfg, iface.setPolicy)

	// Apply the winning single-valued attributes (see Node.SetClasses).
	if prefix, ok := single.get("prefix"); ok {
		iface.NamePrefix = prefix
	}

	return nil
}

func (iface *Interface) String() string {
	return fmt.Sprintf("%s.%s", iface.Node.String(), iface.Name)
}

func (iface *Interface) StringForMessage() string {
	return fmt.Sprintf("interface:%s", iface.String())
}

func (iface *Interface) ChildClasses() ([]string, error) {
	classes := []string{}
	for layer := range iface.Neighbors {
		classes = append(classes, ClassTypeNeighbor(layer))
	}
	for _, mc := range iface.GetMemberClasses() {
		classType, classNames, err := mc.GetSpecifiedClasses()
		if err != nil {
			return nil, err
		}
		for _, cn := range classNames {
			classes = append(classes, ClassTypeMember(classType, cn))
		}
	}
	return classes, nil
}

func (iface *Interface) Childs(c string) ([]NameSpacer, error) {
	objs := []NameSpacer{}
	tmp := strings.SplitN(c, "_", 3) // Maximum 3 splits for Member
	switch tmp[0] {
	case ClassTypeNeighborHeader:
		if len(tmp) < 2 {
			return nil, fmt.Errorf("invalid neighbor reference %q (expected <header>_<layer>)", c)
		}
		layer := tmp[1]
		for _, iface := range iface.Neighbors[layer] {
			objs = append(objs, iface)
		}
		return objs, nil
	case ClassTypeMemberHeader:
		if len(tmp) < 3 {
			return nil, fmt.Errorf("invalid member reference %q (expected <header>_<classType>_<className>)", c)
		}
		classType := tmp[1]
		className := tmp[2]
		for _, m := range iface.GetMembers() {
			if m.ClassType == classType && m.ClassName == className {
				objs = append(objs, m)
			}
		}
		if len(objs) == 0 {
			return nil, fmt.Errorf("no child objects that match %s", c)
		}
		return objs, nil
	default:
		return nil, fmt.Errorf("invalid class type %s for interface.Childs()", c)
	}
}

func (iface *Interface) DependClasses() ([]string, error) {
	return iface.ChildClasses()
}

func (iface *Interface) Depends(c string) ([]NameSpacer, error) {
	return iface.Childs(c)
}

func (iface *Interface) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, cls := range iface.Connection.GetClasses() {
		cc := cls.(*ConnectionClass)
		configTemplates = append(configTemplates, cc.ConfigTemplates...)
	}
	for _, cls := range iface.GetClasses() {
		ic := cls.(*InterfaceClass)
		configTemplates = append(configTemplates, ic.ConfigTemplates...)
	}
	return configTemplates
}

func (iface *Interface) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	cts := []*ConfigTemplate{}
	for _, ic := range cfg.InterfaceClasses {
		cts = append(cts, ic.ConfigTemplates...)
	}
	return cts
}

func (iface *Interface) GivenIPAddress(layer Layerer) (string, bool) {
	for k, v := range iface.valueLabels {
		if k == layer.IPAddressReplacer() {
			return v, true
		}
	}
	return "", false
}

// func (iface *Interface) setAwareLayers(aware []string, defaults []string, ignoreNode bool, ignoreDefaults bool) {
// 	var givenset mapset.Set[string]
// 	var defaultset mapset.Set[string]
// 	if ignoreDefaults {
// 		defaultset = mapset.NewSet[string]()
// 	} else {
// 		defaultset = mapset.NewSet(defaults...)
// 	}
// 	givenset = mapset.NewSet(aware...)
//
// 	if ignoreNode {
// 		iface.layerAwareObject.awareLayers = defaultset.Union(givenset)
// 	} else {
// 		appendum := defaultset.Union(givenset)
// 		iface.layerAwareObject.awareLayers = appendum.Union(iface.Node.layerAwareObject.awareLayers)
// 	}
// }

func (iface *Interface) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	ic, ok := cfg.interfaceClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid InterfaceClass name %s", cls)
	}
	return ic, nil
}

func (iface *Interface) setInterfaceBaseRelativeNameSpace(
	ns NameSpacer, globalParams map[string]map[string]string, header string) error {
	// self
	for k, val := range iface.GetParams() {
		key := header + k
		ns.SetRelativeParam(key, val)
	}

	// node params
	for k, val := range iface.Node.GetParams() {
		key := header + NumberPrefixNode + k
		ns.SetRelativeParam(key, val)
	}

	// node group params
	for _, group := range iface.Node.Groups {
		group.SetGroupRelativeParams(ns, header)
	}

	// meta value labels
	return setMetaValueLabelNameSpace(ns, iface, globalParams, header)
}

func (iface *Interface) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// global params (place lanels)
	setGlobalParams(iface, globalParams)

	// base params
	if err := iface.setInterfaceBaseRelativeNameSpace(iface, globalParams, ""); err != nil {
		return err
	}

	// opposite interface params
	if iface.Connection != nil {
		if err := iface.Opposite.setInterfaceBaseRelativeNameSpace(iface, globalParams, NumberPrefixOppositeInterface); err != nil {
			return err
		}
	}

	return nil
}

// add Neighbor object only when the Interface has NeighborClasses of corresponding layer
func (iface *Interface) AddNeighbor(neighbor *Interface, layer string) {
	if classes, ok := iface.neighborClassMap[layer]; ok {
		n := &Neighbor{
			Self:            iface,
			Neighbor:        neighbor,
			Layer:           layer,
			NeighborClasses: classes,
			NameSpace:       newNameSpace(),
		}
		iface.Neighbors[layer] = append(iface.Neighbors[layer], n)
	}
}

type Connection struct {
	Name       string
	NamePrefix string
	Src        *Interface
	Dst        *Interface
	Layers     mapset.Set[string]

	*ParsedLabels
	*NameSpace
	*memberReference
	*valueReference
	layerAwareObject
}

// Interfaces implemented by Connection. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var (
	_ NameSpacer     = (*Connection)(nil)
	_ LabelOwner     = (*Connection)(nil)
	_ MemberReferrer = (*Connection)(nil)
	_ ValueOwner     = (*Connection)(nil)
)

func newConnection(src *Interface, dst *Interface) *Connection {
	conn := &Connection{
		Src:              src,
		Dst:              dst,
		Layers:           mapset.NewSet[string](),
		ParsedLabels:     newParsedLabels(),
		NameSpace:        newNameSpace(),
		memberReference:  newMemberReference(),
		valueReference:   newValueReference(),
		layerAwareObject: newLayerAwareObject(),
	}
	return conn
}

func (conn *Connection) SortKey() string {
	return conn.Name
}

func (conn *Connection) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	conn.ParsedLabels = cfg.GetValidConnectionClasses(labels)
	conn.ParsedLabels.classLabels = append(conn.ParsedLabels.classLabels, moduleLabels...)
	// Module-provided classes are the weakest tier (see ClassTier* above):
	// they supply defaults and anything the user writes overrides them.
	for _, name := range moduleLabels {
		conn.ParsedLabels.setClassTier(name, ClassTierModule)
	}
	if err := conn.resolveClasses(cfg); err != nil {
		return err
	}
	return nil
}

// resolveClasses turns the class labels into class definitions. It runs both from
// SetLabels and at the start of SetClasses, because labels can still be added in
// between: AddClassLabels for relational classes, and modules through the
// ObjectClassifier hook. Resolving only in SetLabels would silently drop those.
func (conn *Connection) resolveClasses(cfg *Config) error {
	if err := expandUsedClasses(conn.ParsedLabels, ClassTypeConnection, func(name string) (ComposableClass, bool) {
		def, ok := cfg.ConnectionClassByName(name)
		return def, ok
	}, cfg.GlobalSettings.IgnoreUndefinedClass); err != nil {
		return err
	}
	return conn.resolveClassDefinitions(cfg)
}

func (conn *Connection) resolveClassDefinitions(cfg *Config) error {
	conn.ParsedLabels.Classes = []ObjectClass{}
	for _, cls := range conn.ClassLabels() {
		def, ok := cfg.ConnectionClassByName(cls)
		if !ok {
			if cfg.GlobalSettings.IgnoreUndefinedClass {
				continue
			}
			return fmt.Errorf("invalid connectionclass name %s", cls)
		}
		conn.ParsedLabels.Classes = append(conn.ParsedLabels.Classes, def)
	}
	return nil
}
func (conn *Connection) SetClasses(cfg *Config, nm *NetworkModel) error {
	if err := conn.resolveClasses(cfg); err != nil {
		return err
	}
	// Resolve attributes contributed by several classes (see tieredValues).
	values := newTieredValues()
	single := newTieredValues()
	policies := newTieredValues()

	conn.NamePrefix = DefaultConnectionPrefix

	defaultConnectionLayer := cfg.DefaultConnectionLayer()
	for _, layer := range defaultConnectionLayer {
		conn.Layers.Add(layer)
	}

	// check connectionclass flags to connections and their interfaces
	configNames := map[string]configNameOrigin{}
	for _, cls := range conn.GetClasses() {
		cc := cls.(*ConnectionClass)
		if err := recordConfigNames(configNames, "connection", conn.Name, cc.Name, cc.ConfigTemplates); err != nil {
			return err
		}

		// register connection to connectionClassMemberMap (same pattern as Node/Interface)
		nm.connectionClassMemberMap.addClassMember(cc.Name, conn)

		// check virtual (same pattern as Node/Interface)
		if cc.Virtual {
			conn.SetVirtual(true)
		}

		ccTier := conn.ClassTier(cc.Name)

		// check ippolicy flags (same pattern as Node/Interface)
		for _, p := range cc.IPPolicy {
			isPolicy, other, ok := recordPolicy(policies, cfg, p, ccTier)
			if !isPolicy {
				return fmt.Errorf("invalid policy name %s in connectionclass %s", p, cc.Name)
			}
			if !ok {
				return classConflictError("connection", conn.Name, "policy", other, p)
			}
		}

		// check parameter flags (same pattern as Node/Interface)
		for _, num := range cc.Parameters {
			isPolicy, other, ok := recordPolicy(policies, cfg, num, ccTier)
			if !isPolicy {
				conn.setParamFlag(num)
				continue
			}
			if !ok {
				return classConflictError("connection", conn.Name, "policy", other, num)
			}
		}

		// connected layer
		for _, layer := range cc.Layers {
			conn.Layers.Add(layer)
		}

		// check MemberClasses
		for i := range cc.MemberClasses {
			conn.AddMemberClass(cc.MemberClasses[i])
		}

		// Check for value conflicts
		tier := conn.ClassTier(cc.Name)
		for key, value := range cc.Values {
			if other, ok := values.set(key, value, tier); !ok {
				return classConflictError("connection", conn.Name, "values for '"+key+"'", other, value)
			}
		}

		// Check for prefix conflicts (only if both are non-empty and different)
		if cc.Prefix != "" {
			if other, ok := single.set("prefix", cc.Prefix, tier); !ok {
				return classConflictError("connection", conn.Name, "prefix", other, cc.Prefix)
			}
		}
	}

	applyPolicies(policies, cfg, conn.setPolicy)

	// Apply the winning prefix, as the other object types do. assignConnectionNames
	// used to redo this by taking the first non-empty prefix out of GetClasses(),
	// which is not the tier order once use: has appended classes to the end.
	if prefix, ok := single.get("prefix"); ok {
		conn.NamePrefix = prefix
	}

	return nil
}

func (conn *Connection) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {
	// global params (place labels)
	setGlobalParams(conn, globalParams)

	// self params
	for key, val := range conn.GetParams() {
		conn.SetRelativeParam(key, val)
	}

	return nil
}

func (conn *Connection) String() string {
	return fmt.Sprintf("%s--%s", conn.Src.String(), conn.Dst.String())
}

func (conn *Connection) StringForMessage() string {
	return fmt.Sprintf("connection:%s", conn.String())
}

func (conn *Connection) ChildClasses() ([]string, error) {
	classes := []string{}
	for _, mc := range conn.GetMemberClasses() {
		classType, classNames, err := mc.GetSpecifiedClasses()
		if err != nil {
			return nil, err
		}
		for _, cn := range classNames {
			classes = append(classes, ClassTypeMember(classType, cn))
		}
	}
	return classes, nil
}

func (conn *Connection) Childs(c string) ([]NameSpacer, error) {
	objs := []NameSpacer{}
	tmp := strings.SplitN(c, "_", 3) // Maximum 3 splits for Member

	switch tmp[0] {
	case ClassTypeMemberHeader:
		if len(tmp) < 3 {
			return nil, fmt.Errorf("invalid member reference %q (expected <header>_<classType>_<className>)", c)
		}
		classType := tmp[1]
		className := tmp[2]
		for _, m := range conn.GetMembers() {
			if m.ClassType == classType && m.ClassName == className {
				objs = append(objs, m)
			}
		}
		if len(objs) == 0 {
			return nil, fmt.Errorf("no child objects that match %s", c)
		}
		return objs, nil
	default:
		return nil, fmt.Errorf("invalid class type %s for connection.Childs()", c)
	}
}

func (conn *Connection) DependClasses() ([]string, error) {
	classes, err := conn.ChildClasses()
	if err != nil {
		return nil, err
	}
	// Add dependency on source and destination interfaces
	classes = append(classes, ClassTypeInterface)
	return classes, nil
}

func (conn *Connection) Depends(c string) ([]NameSpacer, error) {
	switch c {
	case ClassTypeInterface:
		return []NameSpacer{conn.Src, conn.Dst}, nil
	default:
		return conn.Childs(c)
	}
}

func (conn *Connection) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	cc, ok := cfg.connectionClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid ConnectionClass name %s", cls)
	}
	return cc, nil
}

func (conn *Connection) GivenIPNetwork(layer Layerer) (string, bool) {
	for k, v := range conn.valueLabels {
		if k == layer.IPNetworkReplacer() {
			return v, true
		}
	}
	return "", false
}

func (conn *Connection) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, cls := range conn.GetClasses() {
		cc := cls.(*ConnectionClass)
		configTemplates = append(configTemplates, cc.ConfigTemplates...)
	}
	return configTemplates
}

func (conn *Connection) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	cts := []*ConfigTemplate{}
	for _, cc := range cfg.ConnectionClasses {
		cts = append(cts, cc.ConfigTemplates...)
	}
	return cts
}

// type NetworkSegments struct {
// 	Layer    *Layer
// 	Segments []*SegmentMembers
// }

type NetworkSegment struct {
	Name        string // Auto-assigned name
	Layer       string
	Interfaces  []*Interface
	Connections []*Connection
	NamePrefix  string // Prefix for auto-naming

	*NameSpace
	*ParsedLabels
	*memberReference
	*valueReference
}

// Interfaces implemented by NetworkSegment. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var (
	_ NameSpacer     = (*NetworkSegment)(nil)
	_ LabelOwner     = (*NetworkSegment)(nil)
	_ MemberReferrer = (*NetworkSegment)(nil)
	_ ValueOwner     = (*NetworkSegment)(nil)
)

func NewNetworkSegment() *NetworkSegment {
	s := &NetworkSegment{
		Interfaces:      []*Interface{},
		Connections:     []*Connection{},
		ParsedLabels:    newParsedLabels(),
		NameSpace:       newNameSpace(),
		memberReference: newMemberReference(),
		valueReference:  newValueReference(),
	}
	return s
}

func (seg *NetworkSegment) SortKey() string {
	return seg.Name
}

func (seg *NetworkSegment) String() string {
	return seg.Name
}

func (seg *NetworkSegment) StringForMessage() string {
	return fmt.Sprintf("segment:%s:layer=%s(%d interfaces, %d connections)", seg.Name, seg.Layer, len(seg.Interfaces), len(seg.Connections))
}

func (seg *NetworkSegment) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// global params (place lanels)
	setGlobalParams(seg, globalParams)

	// self params
	for key, val := range seg.GetParams() {
		seg.SetRelativeParam(key, val)
	}

	return nil
}

func (seg *NetworkSegment) ChildClasses() ([]string, error) {
	classes := []string{}
	for _, mc := range seg.GetMemberClasses() {
		classType, classNames, err := mc.GetSpecifiedClasses()
		if err != nil {
			return nil, err
		}
		for _, cn := range classNames {
			classes = append(classes, ClassTypeMember(classType, cn))
		}
	}
	return classes, nil
}

func (seg *NetworkSegment) Childs(c string) ([]NameSpacer, error) {
	objs := []NameSpacer{}
	tmp := strings.SplitN(c, "_", 3) // Maximum 3 splits for Member

	switch tmp[0] {
	case ClassTypeMemberHeader:
		if len(tmp) < 3 {
			return nil, fmt.Errorf("invalid member reference %q (expected <header>_<classType>_<className>)", c)
		}
		classType := tmp[1]
		className := tmp[2]
		for _, m := range seg.GetMembers() {
			if m.ClassType == classType && m.ClassName == className {
				objs = append(objs, m)
			}
		}
		if len(objs) == 0 {
			return nil, fmt.Errorf("no child objects that match %s", c)
		}
		return objs, nil
	default:
		return nil, fmt.Errorf("invalid class type %s for segment.Childs()", c)
	}
}

func (seg *NetworkSegment) DependClasses() ([]string, error) {
	classes, err := seg.ChildClasses()
	if err != nil {
		return nil, err
	}
	// Segment depends on its interfaces and connections
	classes = append(classes, ClassTypeInterface, ClassTypeConnection)
	return classes, nil
}

func (seg *NetworkSegment) Depends(c string) ([]NameSpacer, error) {
	switch c {
	case ClassTypeInterface:
		var objs []NameSpacer
		for _, iface := range seg.Interfaces {
			objs = append(objs, iface)
		}
		return objs, nil
	case ClassTypeConnection:
		var objs []NameSpacer
		for _, conn := range seg.Connections {
			objs = append(objs, conn)
		}
		return objs, nil
	default:
		return seg.Childs(c)
	}
}

func (seg *NetworkSegment) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, cls := range seg.GetClasses() {
		sc := cls.(*SegmentClass)
		configTemplates = append(configTemplates, sc.ConfigTemplates...)
	}
	return configTemplates
}

func (seg *NetworkSegment) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	// Segment labels are set indirectly via SetSegmentLabelsFromRelationalLabels
	return fmt.Errorf("segment labels should be set via SetSegmentLabelsFromRelationalLabels, not SetLabels")
}

// SetSegmentLabelsFromRelationalLabels sets segment class labels by collecting relational class labels
// from the segment's connections and interfaces. Unlike SetLabels, segments receive labels indirectly.
func (seg *NetworkSegment) SetSegmentLabelsFromRelationalLabels(cfg *Config, layer *Layer) error {
	// fmt.Printf("DEBUG: SetSegmentLabelsFromRelationalLabels called for segment with %d connections, %d interfaces\n", len(seg.Connections), len(seg.Interfaces))
	scNames := mapset.NewSet[string]()

	// Check connections for relational class labels
	for _, conn := range seg.Connections {
		// fmt.Printf("DEBUG: Checking connection %s, relational labels: %v\n", conn.Name, conn.RelationalClassLabels())
		for _, rlabel := range conn.RelationalClassLabels() {
			// fmt.Printf("DEBUG: Found relational label: %+v\n", rlabel)
			if rlabel.ClassType == ClassTypeSegment {
				// fmt.Printf("DEBUG: Processing segment relational label: %s\n", rlabel.Name)
				sc, ok := cfg.SegmentClassByName(rlabel.Name)
				// fmt.Printf("DEBUG: SegmentClassByName(%s) returned ok=%v\n", rlabel.Name, ok)
				if !ok {
					return fmt.Errorf("unknown segment class (%v)", rlabel.Name)
				}
				// fmt.Printf("DEBUG: SegmentClass layer: %s, current layer: %s\n", sc.Layer, layer.Name)
				if sc.Layer == layer.Name {
					// fmt.Printf("DEBUG: Layer match! Adding %s to scNames\n", rlabel.Name)
					if !scNames.Contains(rlabel.Name) {
						scNames.Add(rlabel.Name)
					}
				} else {
					// fmt.Printf("DEBUG: Layer mismatch - SegmentClass not added\n")
				}
			}
		}
	}

	// Check interfaces for relational class labels
	for _, iface := range seg.Interfaces {
		for _, rlabel := range iface.RelationalClassLabels() {
			if rlabel.ClassType == ClassTypeSegment {
				sc, ok := cfg.SegmentClassByName(rlabel.Name)
				if !ok {
					return fmt.Errorf("unknown segment class (%v)", rlabel.Name)
				}
				if sc.Layer == layer.Name {
					if !scNames.Contains(rlabel.Name) {
						scNames.Add(rlabel.Name)
					}
				}
			}
		}
	}

	// fmt.Printf("DEBUG: Found segment class names: %v\n", scNames.ToSlice())
	for _, name := range scNames.ToSlice() {
		seg.ParsedLabels.AddClassLabels(name)
	}
	// Segment classes arrive through relational labels rather than SetLabels, so
	// the use: expansion has to happen on this path too.
	if err := expandUsedClasses(seg.ParsedLabels, ClassTypeSegment, func(name string) (ComposableClass, bool) {
		def, ok := cfg.SegmentClassByName(name)
		return def, ok
	}, cfg.GlobalSettings.IgnoreUndefinedClass); err != nil {
		return err
	}
	seg.ParsedLabels.Classes = []ObjectClass{}
	for _, name := range seg.ClassLabels() {
		if sc, ok := cfg.SegmentClassByName(name); ok {
			seg.ParsedLabels.Classes = append(seg.ParsedLabels.Classes, sc)
		}
	}
	// fmt.Printf("DEBUG: Final segment classes after SetSegmentLabelsFromRelationalLabels: %v\n", seg.ClassLabels())
	return nil
}

func (seg *NetworkSegment) SetClasses(cfg *Config, nm *NetworkModel) error {
	// set defaults for segments without class
	seg.NamePrefix = DefaultSegmentPrefix
	configNames := map[string]configNameOrigin{}

	// Resolve attributes contributed by several classes (see tieredValues).
	single := newTieredValues()
	values := newTieredValues()

	for _, cls := range seg.GetClasses() {
		sc := cls.(*SegmentClass)
		if err := recordConfigNames(configNames, "segment", seg.Name, sc.Name, sc.ConfigTemplates); err != nil {
			return err
		}
		tier := seg.ClassTier(sc.Name)

		// Check for value conflicts
		for key, value := range sc.Values {
			if other, ok := values.set(key, value, tier); !ok {
				return classConflictError("segment", seg.Name, "values for '"+key+"'", other, value)
			}
		}

		// Check for prefix conflicts (only if both are non-empty and different)
		if sc.Prefix != "" {
			if other, ok := single.set("prefix", sc.Prefix, tier); !ok {
				return classConflictError("segment", seg.Name, "prefix", other, sc.Prefix)
			}
		}

		// check parameter flags
		for _, num := range sc.Parameters {
			seg.setParamFlag(num)
		}
	}

	// Apply the winning single-valued attributes (see Node.SetClasses).
	if prefix, ok := single.get("prefix"); ok {
		seg.NamePrefix = prefix
	}
	// Unlike the other class types, the given values are applied here rather than
	// in setGivenParameters: segments do not exist yet when that runs (they are
	// built later, during IP assignment). Computed parameters are assigned after
	// this point, so the same "do not overwrite" guard applies.
	for key, value := range values.values {
		if !seg.HasParam(key) {
			seg.AddParam(key, value)
		}
	}
	return nil
}

func (seg *NetworkSegment) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	sc, ok := cfg.segmentClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid SegmentClass name %s", cls)
	}
	return sc, nil
}

func (seg *NetworkSegment) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	cts := []*ConfigTemplate{}
	for _, sc := range cfg.SegmentClasses {
		cts = append(cts, sc.ConfigTemplates...)
	}
	return cts
}

type Neighbor struct {
	Self            *Interface
	Neighbor        *Interface
	Layer           string
	NeighborClasses []*NeighborClass

	*NameSpace
}

// Interfaces implemented by Neighbor. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var _ NameSpacer = (*Neighbor)(nil)

func (n *Neighbor) StringForMessage() string {
	return fmt.Sprintf("neighbor:%s(%s)", n.Neighbor.String(), n.Self.String())
}

func (n *Neighbor) ChildClasses() ([]string, error) {
	return []string{}, nil
}

func (n *Neighbor) Childs(c string) ([]NameSpacer, error) {
	return nil, nil
}

func (n *Neighbor) DependClasses() ([]string, error) {
	return n.ChildClasses()
}

func (n *Neighbor) Depends(c string) ([]NameSpacer, error) {
	return n.Childs(c)
}

func (n *Neighbor) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, cls := range n.NeighborClasses {
		configTemplates = append(configTemplates, cls.ConfigTemplates...)
	}
	return configTemplates
}

func (n *Neighbor) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	return n.GetConfigTemplates(cfg)
}

func (n *Neighbor) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// global params (place lanels)
	setGlobalParams(n, globalParams)

	// base params (n.self)
	if err := n.Self.setInterfaceBaseRelativeNameSpace(n, globalParams, ""); err != nil {
		return err
	}

	// base opposite params
	if n.Self.Connection != nil {
		if err := n.Self.Opposite.setInterfaceBaseRelativeNameSpace(n.Self, globalParams, NumberPrefixOppositeInterface); err != nil {
			return err
		}
	}

	// neighbor params
	if err := n.Neighbor.setInterfaceBaseRelativeNameSpace(n, globalParams, NumberPrefixNeighbor); err != nil {
		return err
	}

	// neighbor opposite params
	if n.Neighbor.Connection != nil {
		if err := n.Neighbor.Opposite.setInterfaceBaseRelativeNameSpace(n.Neighbor, globalParams, NumberPrefixNeighbor+NumberPrefixOppositeInterface); err != nil {
			return err
		}
	}

	return nil
}

type Member struct {
	ClassName string
	ClassType string
	Referrer  MemberReferrer
	Member    NameSpacer

	*NameSpace
}

// Interfaces implemented by Member. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var _ NameSpacer = (*Member)(nil)

func NewMember(cls string, classtype string, memberObject NameSpacer, referrer MemberReferrer) *Member {
	m := Member{
		ClassName: cls,
		ClassType: classtype,
		Referrer:  referrer,
		Member:    memberObject,
		NameSpace: newNameSpace(),
	}
	return &m
}

func (m *Member) StringForMessage() string {
	return fmt.Sprintf("member:target=(%s),referrer=(%s)", m.Member.StringForMessage(), m.Referrer.StringForMessage())
}

func (m *Member) ChildClasses() ([]string, error) {
	return []string{}, nil
}

func (m *Member) Childs(c string) ([]NameSpacer, error) {
	return nil, nil
}

func (m *Member) DependClasses() ([]string, error) {
	return m.ChildClasses()
}

func (m *Member) Depends(c string) ([]NameSpacer, error) {
	return m.Childs(c)
}

func (m *Member) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, mc := range m.Referrer.GetMemberClasses() {
		configTemplates = append(configTemplates, mc.ConfigTemplates...)
	}
	return configTemplates
}

func (m *Member) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	return m.GetConfigTemplates(cfg)
}

func (m *Member) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// placelabels
	setGlobalParams(m, globalParams)

	mr := m.Referrer
	mm := m.Member

	//fmt.Printf("#MEMBER %s\n", m.StringForMessage())
	//fmt.Printf("#referer %+v\n", mr.GetParams())
	//fmt.Printf("#member %+v\n", mm.GetParams())

	//switch m.ClassType {
	//case: ClassTypeNode:
	//	mr.(*Node).set
	//}

	// params of member referrer itself
	for key, val := range mr.GetParams() {
		m.SetRelativeParam(key, val)
	}

	// node params for interfaces
	if m.ClassType == ClassTypeInterface {
		for nodekey, val := range m.Referrer.(*Interface).Node.GetParams() {
			key := NumberPrefixNode + nodekey
			m.SetRelativeParam(key, val)
		}
	}

	// member parameters
	for mkey, val := range mm.GetParams() {
		key := NumberPrefixMember + mkey
		m.SetRelativeParam(key, val)
	}

	//fmt.Printf("#result %+v\n", m.relativeParams)

	return nil
}

type Group struct {
	Name  string
	Nodes []*Node

	*NameSpace
	*ParsedLabels
	*valueReference

	//numbered mapset.Set[string]
}

// Interfaces implemented by Group. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var (
	_ NameSpacer    = (*Group)(nil)
	_ LabelOwner    = (*Group)(nil)
	_ ValueOwner    = (*Group)(nil)
	_ FileGenerator = (*Group)(nil)
)

func newGroup(name string) *Group {
	group := &Group{
		Name:           name,
		Nodes:          []*Node{},
		NameSpace:      newNameSpace(),
		valueReference: newValueReference(),
	}
	return group
}

func (g *Group) SortKey() string {
	return g.Name
}

func (g *Group) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	g.ParsedLabels = cfg.GetValidGroupClasses(labels)
	g.ParsedLabels.classLabels = append(g.ParsedLabels.classLabels, moduleLabels...)
	// Module-provided classes are the weakest tier (see ClassTier* above):
	// they supply defaults and anything the user writes overrides them.
	for _, name := range moduleLabels {
		g.ParsedLabels.setClassTier(name, ClassTierModule)
	}
	if err := g.resolveClasses(cfg); err != nil {
		return err
	}
	return nil
}

// resolveClasses turns the class labels into class definitions. It runs both from
// SetLabels and at the start of SetClasses, because labels can still be added in
// between: AddClassLabels for relational classes, and modules through the
// ObjectClassifier hook. Resolving only in SetLabels would silently drop those.
func (g *Group) resolveClasses(cfg *Config) error {
	if err := expandUsedClasses(g.ParsedLabels, ClassTypeGroup, func(name string) (ComposableClass, bool) {
		def, ok := cfg.GroupClassByName(name)
		return def, ok
	}, cfg.GlobalSettings.IgnoreUndefinedClass); err != nil {
		return err
	}
	return g.resolveClassDefinitions(cfg)
}

func (g *Group) resolveClassDefinitions(cfg *Config) error {
	g.ParsedLabels.Classes = []ObjectClass{}
	for _, cls := range g.ClassLabels() {
		def, ok := cfg.GroupClassByName(cls)
		if !ok {
			if cfg.GlobalSettings.IgnoreUndefinedClass {
				continue
			}
			return fmt.Errorf("invalid groupclass name %s", cls)
		}
		g.ParsedLabels.Classes = append(g.ParsedLabels.Classes, def)
	}
	return nil
}
func (g *Group) SetClasses(cfg *Config, nm *NetworkModel) error {
	if err := g.resolveClasses(cfg); err != nil {
		return err
	}
	// Resolve attributes contributed by several classes (see tieredValues).
	values := newTieredValues()

	configNames := map[string]configNameOrigin{}
	for _, cls := range g.GetClasses() {
		gc := cls.(*GroupClass)
		if err := recordConfigNames(configNames, "group", g.Name, gc.Name, gc.ConfigTemplates); err != nil {
			return err
		}
		//	for _, cls := range g.classLabels {
		//		gc, ok := cfg.groupClassMap[cls]
		//		if !ok {
		//			return fmt.Errorf("invalid GroupClass name %s", cls)
		//		}
		//		g.ParsedLabels.Classes = append(g.ParsedLabels.Classes, gc)

		// set virtual
		if gc.Virtual {
			g.SetVirtual(true)
		}

		// check numbered
		for _, num := range gc.Parameters {
			g.setParamFlag(num)
		}

		// Check for value conflicts
		tier := g.ClassTier(gc.Name)
		for key, value := range gc.Values {
			if other, ok := values.set(key, value, tier); !ok {
				return classConflictError("group", g.Name, "values for '"+key+"'", other, value)
			}
		}
	}
	return nil
}

func (g *Group) StringForMessage() string {
	return fmt.Sprintf("group:%s", g.Name)
}

// ChildClasses lets a group aggregate the config blocks of the objects it
// contains, the same way the network model does. This is what makes a
// group-scope file such as a per-host topo.yaml expressible: the template
// refers to {{ .nodes_... }} and {{ .connections_... }} and gets only the
// members of that group.
//
// Note that a node reached through a group is also a child of the network
// model, so the same object is registered under two parents on purpose.
func (g *Group) ChildClasses() ([]string, error) {
	return []string{ClassTypeNode, ClassTypeConnection}, nil
}

func (g *Group) Childs(c string) ([]NameSpacer, error) {
	switch c {
	case ClassTypeNode:
		var nodes []NameSpacer
		for _, n := range g.Nodes {
			nodes = append(nodes, n)
		}
		return nodes, nil
	case ClassTypeConnection:
		return g.internalConnections(), nil
	default:
		return nil, fmt.Errorf("invalid class type %s for group.Childs()", c)
	}
}

// internalConnections returns the connections with both endpoints inside the
// group. A connection that leaves the group is deliberately excluded: it
// belongs to no single group, and rendering it into a per-group file would
// name an endpoint the file cannot reach.
func (g *Group) internalConnections() []NameSpacer {
	members := make(map[*Node]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		members[n] = true
	}

	var connections []NameSpacer
	seen := map[*Connection]bool{}
	for _, n := range g.Nodes {
		for _, iface := range n.Interfaces {
			conn := iface.Connection
			if conn == nil || seen[conn] {
				continue
			}
			if conn.Src == nil || conn.Dst == nil {
				continue
			}
			if !members[conn.Src.Node] || !members[conn.Dst.Node] {
				continue
			}
			seen[conn] = true
			connections = append(connections, conn)
		}
	}
	return connections
}

func (g *Group) DependClasses() ([]string, error) {
	return g.ChildClasses()
}

func (g *Group) Depends(c string) ([]NameSpacer, error) {
	return g.Childs(c)
}

func (g *Group) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	configTemplates := []*ConfigTemplate{}
	for _, cls := range g.GetClasses() {
		gc := cls.(*GroupClass)
		configTemplates = append(configTemplates, gc.ConfigTemplates...)
	}
	return configTemplates
}

func (g *Group) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	cts := []*ConfigTemplate{}
	for _, gc := range cfg.GroupClasses {
		cts = append(cts, gc.ConfigTemplates...)
	}
	return cts
}

// OutputDir returns the directory that this node's files are written into,
// relative to the output root: the directory of the group that carries
// GlobalSettings.OutputGroupClass, or empty when the setting is unused or the
// node belongs to no such group.
//
// Belonging to two such groups is an error rather than a choice: the class is
// meant to name a placement unit (a host), and a node sits on exactly one.
func (n *Node) OutputDir(cfg *Config) (string, error) {
	className := cfg.GlobalSettings.OutputGroupClass
	if className == "" {
		return "", nil
	}
	dir := ""
	for _, group := range n.Groups {
		if !group.HasClass(className) {
			continue
		}
		if dir != "" {
			return "", fmt.Errorf(
				"node %s belongs to more than one %s group (%s and %s), so its output directory is ambiguous",
				n.Name, className, dir, group.Name,
			)
		}
		dir = group.Name
	}
	return dir, nil
}

// FilesToGenerate returns a list of file names that the group will generate based on its classes.
// It examines GroupClass ConfigTemplates.
func (g *Group) FilesToGenerate(cfg *Config) []string {
	fileSet := make(map[string]bool)
	for _, cls := range g.GetClasses() {
		gc := cls.(*GroupClass)
		for _, ct := range gc.ConfigTemplates {
			if ct.File != "" {
				fileSet[ct.File] = true
			}
		}
	}

	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func (g *Group) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	gc, ok := cfg.groupClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid GroupClass name %s", cls)
	}
	return gc, nil
}

// Set relative parameters of the group to the group member namespacers
func (g *Group) SetGroupRelativeParams(ns NameSpacer, header string) error {
	// opposite: include opposite prefix in the keys

	for k, val := range g.GetParams() {
		// prioritize numbers by node-num > smaller-group-num > large-group-num
		num := header + NumberPrefixGroup + k
		if !ns.HasRelativeParam(num) {
			ns.SetRelativeParam(num, val)
		}

		// alias for group classes (for multi-layer groups)
		for _, label := range g.ClassLabels() {
			cnum := header + label + NumberSeparator + k
			if !ns.HasRelativeParam(cnum) {
				ns.SetRelativeParam(cnum, val)
			}
		}
	}
	return nil
}

func (g *Group) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// global params (place labels)
	setGlobalParams(g, globalParams)

	// base params - copy self params to relativeParams
	for k, val := range g.GetParams() {
		g.SetRelativeParam(k, val)
	}

	// group params with prefix (for group_ prefixed access)
	g.SetGroupRelativeParams(g, "")

	return nil
}

// Value represents a virtual object that holds a set of related parameters.
// Unlike Node, Interface, etc., Value has no corresponding object in the DOT file.
// Values are dynamically generated by param_rule with mode: attach.
type Value struct {
	// ParamRuleName is the name of the param_rule that generated this Value
	ParamRuleName string
	// Owner is the parent object that owns this Value
	Owner ValueOwner
	// Index is the position in the Value list (0, 1, 2, ...)
	Index int

	*NameSpace
}

// Interfaces implemented by Value. ObjectInstance is omitted: it is embedded in
// NameSpacer, so asserting NameSpacer already covers it.
var _ NameSpacer = (*Value)(nil)

// NewValue creates a new Value with the given parameters
func NewValue(paramRuleName string, owner ValueOwner, index int) *Value {
	return &Value{
		ParamRuleName: paramRuleName,
		Owner:         owner,
		Index:         index,
		NameSpace:     newNameSpace(),
	}
}

// StringForMessage returns a string representation for debug messages
func (v *Value) StringForMessage() string {
	return fmt.Sprintf("value:%s[%d]@%s", v.ParamRuleName, v.Index, v.Owner.StringForMessage())
}

// ChildClasses returns an empty list (Value is a leaf object)
func (v *Value) ChildClasses() ([]string, error) {
	return []string{}, nil
}

// Childs returns nil (Value has no children)
func (v *Value) Childs(c string) ([]NameSpacer, error) {
	return nil, nil
}

// DependClasses returns an empty list (Value has no dependencies)
func (v *Value) DependClasses() ([]string, error) {
	return []string{}, nil
}

// Depends returns nil (Value has no dependencies)
func (v *Value) Depends(c string) ([]NameSpacer, error) {
	return nil, nil
}

// GetConfigTemplates returns config templates from the param_rule that generated this Value
func (v *Value) GetConfigTemplates(cfg *Config) []*ConfigTemplate {
	rule, ok := cfg.ParameterRuleByName(v.ParamRuleName)
	if !ok {
		return []*ConfigTemplate{}
	}
	return rule.ConfigTemplates
}

// GetPossibleConfigTemplates returns the same as GetConfigTemplates for Value
func (v *Value) GetPossibleConfigTemplates(cfg *Config) []*ConfigTemplate {
	return v.GetConfigTemplates(cfg)
}

// BuildRelativeNameSpace builds the relative namespace for template rendering
func (v *Value) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {
	// global params (place labels)
	setGlobalParams(v, globalParams)

	// self params
	for key, val := range v.GetParams() {
		v.SetRelativeParam(key, val)
	}

	// owner params with owner_ prefix
	for key, val := range v.Owner.GetParams() {
		v.SetRelativeParam("owner_"+key, val)
	}

	return nil
}

// CollectTarget is one file to copy out of a node before the lab is destroyed:
// where it is inside the container, and where it lands in the output.
type CollectTarget struct {
	Node          string
	ContainerPath string
	OutputPath    string
}

// CollectDirName is where collected files land, mirroring the container's own
// paths under a directory per node: collected/r1/var/log/frr.log. It stands
// beside the generated tree rather than in it - what was generated and what
// came back from a run are not the same kind of thing.
const CollectDirName = "collected"

// CollectTargets lists what the lab is asked to bring back, in node order so
// that two runs produce the same script. The paths are templates, rendered with
// the node's own parameters.
func CollectTargets(cfg *Config, nm *NetworkModel) ([]CollectTarget, error) {
	var targets []CollectTarget
	for _, node := range nm.Nodes {
		// Nothing to bring back from a node that was never put in place, or from
		// one the platform provides itself: neither has a filesystem of ours.
		if !node.IsMaterialised() || cfg.IsSwitchNode(node) {
			continue
		}
		for _, cls := range node.GetClasses() {
			nc, ok := cls.(*NodeClass)
			if !ok {
				continue
			}
			for _, raw := range nc.Collect {
				target, err := renderWithNodeParams(raw, node)
				if err != nil {
					return nil, fmt.Errorf("collect path %q of class %s: %w", raw, nc.Name, err)
				}
				targets = append(targets, CollectTarget{
					Node:          node.Name,
					ContainerPath: target,
					OutputPath:    path.Join(CollectDirName, node.Name, strings.TrimPrefix(target, "/")),
				})
			}
		}
	}
	return targets, nil
}

// renderWithNodeParams fills a collect path in with the node's parameters, the
// same way a config template is filled in.
func renderWithNodeParams(raw string, node *Node) (string, error) {
	tpl, err := template.New("collect").Option("missingkey=error").Parse(raw)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	// The node's own parameters, which is where a class's values land. The
	// relative ones (self_*, and the rest) are filled in later, when config
	// blocks are rendered, and a path does not need them.
	if err := tpl.Execute(&sb, node.GetParams()); err != nil {
		return "", err
	}
	return sb.String(), nil
}
