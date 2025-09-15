package types

import (
	"fmt"
	"os"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

const DefaultNodePrefix string = "node"
const DefaultInterfacePrefix string = "net"
const DefaultConnectionPrefix string = "conn"

// abstracted module

type Module interface {
	UpdateConfig(cfg *Config) error
	AddModuleNodeClassLabel(label string)
	GetModuleNodeClassLabels() []string
	AddModuleInterfaceClassLabel(label string)
	GetModuleInterfaceClassLabels() []string
	AddModuleConnectionClassLabel(label string)
	GetModuleConnectionClassLabels() []string
	//SetClasses(cfg *Config, nm *NetworkModel) error
	GenerateParameters(cfg *Config, nm *NetworkModel) error
	CheckModuleRequirements(cfg *Config, nm *NetworkModel) error
}

type StandardModule struct {
	NodeClassLabels       []string
	InterfaceClassLabels  []string
	ConnectionClassLabels []string
}

func NewStandardModule() *StandardModule {
	return &StandardModule{
		NodeClassLabels:       []string{},
		InterfaceClassLabels:  []string{},
		ConnectionClassLabels: []string{},
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

// abstracted structures

type ObjectInstance interface {
	StringForMessage() string // just for debug messages
}

// NameSpacer is an element of top-down network model
// A namespacer owns parameter namespace and generates configuration blocks
// Candidates: Network, Node, Interface, Segment, Neighbor, Member, Group
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

// LabelOwner includes Node, Interface, Connection, Group
type LabelOwner interface {
	ClassLabels() []string
	RelationalClassLabels() []RelationalClassLabel
	PlaceLabels() []string
	ValueLabels() map[string]string
	MetaValueLabels() map[string]string
	SetLabels(cfg *Config, labels []string, moduleLabels []string) error
	AddClassLabels(labels ...string)

	HasClass(string) bool
	GetClasses() []ObjectClass

	SetVirtual(bool)
	IsVirtual() bool

	ClassDefinition(cfg *Config, cls string) (interface{}, error)

	ObjectInstance
}

type RelationalClassLabel struct {
	ClassType string
	Name      string
}

type ParsedLabels struct {
	classLabels     []string
	rClassLabels    []RelationalClassLabel
	placeLabels     []string
	valueLabels     map[string]string
	metaValueLabels map[string]string
	Classes         []ObjectClass
	virtual         bool // virtual object flag
}

func newParsedLabels() *ParsedLabels {
	return &ParsedLabels{
		classLabels:     []string{},
		rClassLabels:    []RelationalClassLabel{},
		placeLabels:     []string{},
		valueLabels:     map[string]string{},
		metaValueLabels: map[string]string{},
	}
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

func (l *ParsedLabels) AddClassLabels(labels ...string) {
	l.classLabels = append(l.classLabels, labels...)
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

// addressOwner includes Node, Interface
// commented out because it currently does not have abstracted usage (explicitly addressed)
// type addressOwner interface {
// 	setAware(string)
// 	IsAware(string) bool
// }

type addressedObject struct {
	layerPolicy map[string]*IPPolicy
	layers      mapset.Set[string]
}

func newAddressedObject() addressedObject {
	return addressedObject{
		layerPolicy: map[string]*IPPolicy{},
		layers:      mapset.NewSet[string](),
	}
}

func (a addressedObject) AwareLayer(layer string) bool {
	return a.layers.Contains(layer)
}

func (a addressedObject) GetLayerPolicy(layer string) *IPPolicy {
	val, ok := a.layerPolicy[layer]
	if ok {
		return val
	} else {
		return nil
	}
}

func (a addressedObject) setPolicy(layer *Layer, policy *IPPolicy) {
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
	// configFileGenerator

	NetworkSegments map[string][]*NetworkSegment
	//Files           *ConfigFiles

	nodeMap                  map[string]*Node
	groupMap                 map[string]*Group
	nodeClassMemberMap       classMemberMap
	interfaceClassMemberMap  classMemberMap
	connectionClassMemberMap classMemberMap
	segmentClassMemberMap    classMemberMap
}

func NewNetworkModel() *NetworkModel {
	nm := &NetworkModel{
		NetworkSegments: map[string][]*NetworkSegment{},
		//Files:                    newConfigFiles(),
		NameSpace:                newNameSpace(),
		nodeMap:                  map[string]*Node{},
		groupMap:                 map[string]*Group{},
		nodeClassMemberMap:       classMemberMap{mapper: map[string][]NameSpacer{}},
		interfaceClassMemberMap:  classMemberMap{mapper: map[string][]NameSpacer{}},
		connectionClassMemberMap: classMemberMap{mapper: map[string][]NameSpacer{}},
		segmentClassMemberMap:    classMemberMap{mapper: map[string][]NameSpacer{}},
	}
	return nm
}

func (nm *NetworkModel) NewNode(name string) *Node {
	node := newNode(name)
	// node := &Node{
	// 	Name:            name,
	// 	NameSpace:       newNameSpace(),
	// 	addressedObject: newAddressedObject(),
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

func (nm *NetworkModel) SegmentClassMembers(cls string) []NameSpacer {
	return nm.segmentClassMemberMap.getClassMembers(cls)
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
	TinetAttr *map[string]interface{}
	ClabAttr  *map[string]interface{}

	*NameSpace
	*ParsedLabels
	*memberReference
	addressedObject
	// configFileGenerator

	NamePrefix string

	mgmtInterface      *Interface
	mgmtInterfaceClass *InterfaceClass
	interfaceMap       map[string]*Interface
}

func newNode(name string) *Node {
	node := &Node{
		Name:            name,
		NameSpace:       newNameSpace(),
		addressedObject: newAddressedObject(),
		interfaceMap:    map[string]*Interface{},
		memberReference: newMemberReference(),
	}
	return node
}

func (n *Node) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	n.ParsedLabels = cfg.GetValidNodeClasses(labels)
	n.ParsedLabels.classLabels = append(n.ParsedLabels.classLabels, moduleLabels...)
	for _, cls := range n.ClassLabels() {
		nc, ok := cfg.NodeClassByName(cls)
		if !ok {
			return fmt.Errorf("invalid nodeclass name %s", cls)
		}
		n.ParsedLabels.Classes = append(n.ParsedLabels.Classes, nc)
	}
	return nil
}

func (n *Node) SetClasses(cfg *Config, nm *NetworkModel) error {
	primaryNC := ""

	// set defaults for nodes without primary class
	n.NamePrefix = DefaultNodePrefix

	for _, cls := range n.GetClasses() {
		nc := cls.(*NodeClass)
		// nc, ok := cfg.NodeClassByName(cls)
		// if !ok {
		// 	return fmt.Errorf("invalid nodeclass name %s", cls)
		// }
		// n.ParsedLabels.Classes = append(n.ParsedLabels.Classes, nc)
		nm.nodeClassMemberMap.addClassMember(nc.Name, n)

		// check virtual
		if nc.Virtual {
			n.SetVirtual(true)
		}

		// check ippolicy flags
		for _, p := range nc.IPPolicy {
			policy, ok := cfg.policyMap[p]
			if ok {
				n.setPolicy(policy.layer, policy)
			} else {
				return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
			}
		}

		// check interface_policy flags
		for _, p := range nc.InterfaceIPPolicy {
			policy, ok := cfg.policyMap[p]
			if ok {
				for _, iface := range n.Interfaces {
					iface.setPolicy(policy.layer, policy)
				}
			} else {
				return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
			}
		}

		// check parameter flags
		for _, num := range nc.Parameters {
			policy, ok := cfg.policyMap[num]
			if ok {
				// ip policy
				n.setPolicy(policy.layer, policy)
			} else {
				n.setParamFlag(num)
			}
		}

		// check MemberClasses
		for i := range nc.MemberClasses {
			n.AddMemberClass(nc.MemberClasses[i])
		}

		// check primary node class consistency
		if nc.Primary {
			if primaryNC == "" {
				primaryNC = nc.Name
			} else {
				return fmt.Errorf("multiple primary node classes on one node (%s, %s)", primaryNC, nc.Name)
			}
			// add parameters of primary node class
			if n.NamePrefix != "" {
				n.NamePrefix = nc.Prefix
			}
			if nc.MgmtInterface != "" {
				if mgmtnc, ok := cfg.InterfaceClassByName(nc.MgmtInterface); ok {
					n.mgmtInterfaceClass = mgmtnc
				} else {
					return fmt.Errorf("invalid mgmt interface class name %s", nc.MgmtInterface)
				}
			}
			n.TinetAttr = &nc.TinetAttr
			n.ClabAttr = &nc.ClabAttr
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

	if primaryNC == "" && !n.IsVirtual() {
		fmt.Fprintf(os.Stderr, "warning: no primary node class on node %s\n", n.Name)
	}
	return nil
}

func (n *Node) String() string {
	return n.Name
}

func (n *Node) StringForMessage() string {
	return fmt.Sprintf("node:%s", n.Name)
}

func (n *Node) NewInterface(name string) *Interface {
	iface := newInterface(n, name)
	// iface := &Interface{
	// 	Name:             name,
	// 	Node:             n,
	// 	Neighbors:        map[string][]*Neighbor{},
	// 	NameSpace:        newNameSpace(),
	// 	addressedObject:  newAddressedObject(),
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
		iface.SetLabels(cfg, []string{ic.Name}, []string{})
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
// 	n.addressedObject.AwareLayers = defaultset.Union(givenset)
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
	setMetaValueLabelNameSpace(ns, n, globalParams, header)

	return nil
}

func (n *Node) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {
	// global params (place lanels)
	setGlobalParams(n, globalParams)

	// base params
	n.setNodeBaseRelativeNameSpace(n, globalParams, "")

	return nil
}

type Interface struct {
	Name       string
	Node       *Node
	Virtual    bool
	Connection *Connection
	Opposite   *Interface
	Neighbors  map[string][]*Neighbor
	TinetAttr  *map[string]interface{}
	// ClabAttr        *map[string]interface{}
	NamePrefix string

	*NameSpace
	*ParsedLabels
	*memberReference
	addressedObject

	//hasNeighborClass map[string]bool // key: layer
	neighborClassMap map[string][]*NeighborClass
}

func newInterface(node *Node, name string) *Interface {
	iface := &Interface{
		Name:            name,
		Node:            node,
		Neighbors:       map[string][]*Neighbor{},
		NameSpace:       newNameSpace(),
		addressedObject: newAddressedObject(),
		memberReference: newMemberReference(),
		// hasNeighborClass: map[string]bool{},
		neighborClassMap: map[string][]*NeighborClass{},
	}
	return iface
}

func (iface *Interface) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	iface.ParsedLabels = cfg.GetValidInterfaceClasses(labels)
	iface.ParsedLabels.classLabels = append(iface.ParsedLabels.classLabels, moduleLabels...)
	for _, cls := range iface.ClassLabels() {
		ic, ok := cfg.InterfaceClassByName(cls)
		if !ok {
			return fmt.Errorf("invalid interfaceclass name %s", cls)
		}
		iface.ParsedLabels.Classes = append(iface.ParsedLabels.Classes, ic)
	}
	return nil
}

func (iface *Interface) SetClasses(cfg *Config, nm *NetworkModel) error {
	primaryIC := ""

	// set virtual flag to interfaces of virtual nodes as default
	iface.SetVirtual(iface.Node.IsVirtual())
	//iface.Virtual = iface.Node.Virtual

	// set defaults for interfaces without primary class
	iface.NamePrefix = DefaultInterfacePrefix

	// check connectionclass flags
	for _, cls := range iface.Connection.ClassLabels() {
		cc, ok := cfg.ConnectionClassByName(cls)
		if !ok {
			return fmt.Errorf("invalid connectionclass name %s", cls)
		}
		nm.connectionClassMemberMap.addClassMember(cc.Name, iface)

		// check virtual
		//iface.Virtual = iface.Virtual || cc.Virtual
		if cc.Virtual {
			iface.SetVirtual(true)
			iface.Connection.SetVirtual(true)
		}

		// check ippolicy flags
		for _, p := range cc.IPPolicy {
			policy, ok := cfg.policyMap[p]
			if ok {
				iface.setPolicy(policy.layer, policy)
			} else {
				return fmt.Errorf("invalid policy name %s in connectionclass %s", p, cc.Name)
			}
		}

		// check parameter flags
		for _, num := range cc.Parameters {
			policy, ok := cfg.policyMap[num]
			if ok {
				// ip policy
				iface.setPolicy(policy.layer, policy)
			} else {
				iface.setParamFlag(num)
			}
		}


		// check MemberClasses
		for i := range cc.MemberClasses {
			iface.AddMemberClass(cc.MemberClasses[i])
		}


	}

	// rebuild ParsedLabels.Classes based on current classLabels to include classes added by AddClassLabels
	iface.ParsedLabels.Classes = []ObjectClass{}
	for _, clsName := range iface.ClassLabels() {
		if ic, ok := cfg.InterfaceClassByName(clsName); ok {
			iface.ParsedLabels.Classes = append(iface.ParsedLabels.Classes, ic)
		}
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

		// check virtual
		if ic.Virtual {
			iface.SetVirtual(true)
			iface.Connection.SetVirtual(true)
		}
		//iface.Virtual = iface.Virtual || ic.Virtual

		// check ippolicy flags
		for _, p := range ic.IPPolicy {
			policy, ok := cfg.policyMap[p]
			if ok {
				iface.setPolicy(policy.layer, policy)
			} else {
				return fmt.Errorf("invalid policy name %s in interfaceclass %s", p, ic.Name)
			}
		}

		// check parameter flags
		for _, num := range ic.Parameters {
			policy, ok := cfg.policyMap[num]
			if ok {
				// ip policy
				iface.setPolicy(policy.layer, policy)
			} else {
				iface.setParamFlag(num)
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

		// check primary interface class consistency
		if ic.Primary {
			if primaryIC == "" {
				primaryIC = ic.Name
			} else {
				return fmt.Errorf("multiple primary interface classes on one node (%s, %s)", primaryIC, ic.Name)
			}
			if ic.Prefix != "" {
				iface.NamePrefix = ic.Prefix
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
		layer := tmp[1]
		for _, iface := range iface.Neighbors[layer] {
			objs = append(objs, iface)
		}
		return objs, nil
	case ClassTypeMemberHeader:
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
// 		iface.addressedObject.awareLayers = defaultset.Union(givenset)
// 	} else {
// 		appendum := defaultset.Union(givenset)
// 		iface.addressedObject.awareLayers = appendum.Union(iface.Node.addressedObject.awareLayers)
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
	setMetaValueLabelNameSpace(ns, iface, globalParams, header)

	return nil
}

func (iface *Interface) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// global params (place lanels)
	setGlobalParams(iface, globalParams)

	// base params
	iface.setInterfaceBaseRelativeNameSpace(iface, globalParams, "")

	// opposite interface params
	if iface.Connection != nil {
		iface.Opposite.setInterfaceBaseRelativeNameSpace(iface, globalParams, NumberPrefixOppositeInterface)
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
	Name   string
	Src    *Interface
	Dst    *Interface
	Layers mapset.Set[string]

	*ParsedLabels
	*NameSpace
	*memberReference
	addressedObject
}

func newConnection(src *Interface, dst *Interface) *Connection {
	conn := &Connection{
		Src:             src,
		Dst:             dst,
		Layers:          mapset.NewSet[string](),
		ParsedLabels:    newParsedLabels(),
		NameSpace:       newNameSpace(),
		memberReference: newMemberReference(),
		addressedObject: newAddressedObject(),
	}
	return conn
}

func (conn *Connection) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	conn.ParsedLabels = cfg.GetValidConnectionClasses(labels)
	conn.ParsedLabels.classLabels = append(conn.ParsedLabels.classLabels, moduleLabels...)
	for _, cls := range conn.ClassLabels() {
		cc, ok := cfg.ConnectionClassByName(cls)
		if !ok {
			return fmt.Errorf("invalid interfaceclass name %s", cls)
		}
		conn.ParsedLabels.Classes = append(conn.ParsedLabels.Classes, cc)
	}
	return nil
}

func (conn *Connection) SetClasses(cfg *Config, nm *NetworkModel) error {
	defaultConnectionLayer := cfg.DefaultConnectionLayer()
	for _, layer := range defaultConnectionLayer {
		conn.Layers.Add(layer)
	}

	// check connectionclass flags to connections and their interfaces
	for _, cls := range conn.GetClasses() {
		cc := cls.(*ConnectionClass)
		
		// register connection to connectionClassMemberMap (same pattern as Node/Interface)
		nm.connectionClassMemberMap.addClassMember(cc.Name, conn)
		
		// check virtual (same pattern as Node/Interface)
		if cc.Virtual {
			conn.SetVirtual(true)
		}
		
		// check ippolicy flags (same pattern as Node/Interface)
		for _, p := range cc.IPPolicy {
			policy, ok := cfg.policyMap[p]
			if ok {
				conn.setPolicy(policy.layer, policy)
			} else {
				return fmt.Errorf("invalid policy name %s in connectionclass %s", p, cc.Name)
			}
		}
		
		// check parameter flags (same pattern as Node/Interface)
		for _, num := range cc.Parameters {
			policy, ok := cfg.policyMap[num]
			if ok {
				// ip policy
				conn.setPolicy(policy.layer, policy)
			} else {
				conn.setParamFlag(num)
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
	Layer       string
	Interfaces  []*Interface
	Connections []*Connection

	*NameSpace
	*ParsedLabels
	*memberReference
}

func NewNetworkSegment() *NetworkSegment {
	s := &NetworkSegment{
		Interfaces:      []*Interface{},
		Connections:     []*Connection{},
		ParsedLabels:    newParsedLabels(),
		NameSpace:       newNameSpace(),
		memberReference: newMemberReference(),
	}
	return s
}

func (seg *NetworkSegment) StringForMessage() string {
	return fmt.Sprintf("segment:layer=%s(%d interfaces, %d connections)", seg.Layer, len(seg.Interfaces), len(seg.Connections))
}

func (seg *NetworkSegment) BuildRelativeNameSpace(globalParams map[string]map[string]string) error {

	// global params (place lanels)
	setGlobalParams(seg, globalParams)

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
	scNames := mapset.NewSet[string]()
	
	// Check connections for relational class labels
	for _, conn := range seg.Connections {
		for _, rlabel := range conn.RelationalClassLabels() {
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
	
	for _, name := range scNames.ToSlice() {
		seg.AddClassLabels(name)
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
	n.Self.setInterfaceBaseRelativeNameSpace(n, globalParams, "")

	// base opposite params
	if n.Self.Connection != nil {
		n.Self.Opposite.setInterfaceBaseRelativeNameSpace(n.Self, globalParams, NumberPrefixOppositeInterface)
	}

	// neighbor params
	n.Neighbor.setInterfaceBaseRelativeNameSpace(n, globalParams, NumberPrefixNeighbor)

	// neighbor opposite params
	if n.Neighbor.Connection != nil {
		n.Neighbor.Opposite.setInterfaceBaseRelativeNameSpace(n.Neighbor, globalParams, NumberPrefixNeighbor+NumberPrefixOppositeInterface)
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

	//numbered mapset.Set[string]
}

func newGroup(name string) *Group {
	group := &Group{
		Name:      name,
		Nodes:     []*Node{},
		NameSpace: newNameSpace(),
	}
	return group
}

func (g *Group) SetLabels(cfg *Config, labels []string, moduleLabels []string) error {
	g.ParsedLabels = cfg.GetValidGroupClasses(labels)
	g.ParsedLabels.classLabels = append(g.ParsedLabels.classLabels, moduleLabels...)
	for _, cls := range g.ClassLabels() {
		gc, ok := cfg.GroupClassByName(cls)
		if !ok {
			return fmt.Errorf("invalid interfaceclass name %s", cls)
		}
		g.ParsedLabels.Classes = append(g.ParsedLabels.Classes, gc)
	}
	return nil
}

func (g *Group) SetClasses(cfg *Config, nm *NetworkModel) error {
	for _, cls := range g.GetClasses() {
		gc := cls.(*GroupClass)
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
	}
	return nil
}

func (g *Group) StringForMessage() string {
	return fmt.Sprintf("group:%s", g.Name)
}

func (g *Group) ChildClasses() ([]string, error) {
	return []string{}, nil
}

func (g *Group) Childs(c string) ([]NameSpacer, error) {
	return nil, nil
}

func (g *Group) DependClasses() ([]string, error) {
	classes, err := g.ChildClasses()
	if err != nil {
		return nil, err
	}
	// Group depends on its nodes
	classes = append(classes, ClassTypeNode)
	return classes, nil
}

func (g *Group) Depends(c string) ([]NameSpacer, error) {
	switch c {
	case ClassTypeNode:
		var nodes []NameSpacer
		for _, n := range g.Nodes {
			nodes = append(nodes, n)
		}
		return nodes, nil
	default:
		return g.Childs(c)
	}
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

	// global params (place lanels)
	setGlobalParams(g, globalParams)

	// base params
	g.SetGroupRelativeParams(g, "")

	return nil
}
