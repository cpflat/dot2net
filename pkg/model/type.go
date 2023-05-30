package model

import (
	"fmt"

	mapset "github.com/deckarep/golang-set/v2"
)

// abstracted structures

// labelOwner includes Node, Interface, Connection
type labelOwner interface {
	ClassLabels() []string
	PlaceLabels() []string
	ValueLabels() map[string]string
	MetaValueLabels() map[string]string
	HasClass(string) bool
	GetClasses() []ObjectClass
}

type parsedLabels struct {
	classLabels     []string
	placeLabels     []string
	valueLabels     map[string]string
	metaValueLabels map[string]string
	classes         []ObjectClass
}

func newParsedLabels() *parsedLabels {
	return &parsedLabels{
		classLabels:     []string{},
		placeLabels:     []string{},
		valueLabels:     map[string]string{},
		metaValueLabels: map[string]string{},
	}
}

func (l *parsedLabels) ClassLabels() []string {
	return l.classLabels
}

func (l *parsedLabels) PlaceLabels() []string {
	return l.placeLabels
}

func (l *parsedLabels) ValueLabels() map[string]string {
	return l.valueLabels
}

func (l *parsedLabels) MetaValueLabels() map[string]string {
	return l.metaValueLabels
}

func (l *parsedLabels) HasClass(name string) bool {
	for _, cls := range l.classLabels {
		if cls == name {
			return true
		}
	}
	return false
}

func (l *parsedLabels) GetClasses() []ObjectClass {
	return l.classes
}

// classMemberReferer includes Node, Interface
// commented out because it currently does not have abstracted usage (explicitly addressed)
type memberReferer interface {
	labelOwner
	NameSpacer

	addMemberClass(*MemberClass)
	getMemberClasses() []*MemberClass
	addMember(*Member)
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

func (mr *memberReference) addMemberClass(mc *MemberClass) {
	mr.memberClasses = append(mr.memberClasses, mc)
}

func (mr *memberReference) getMemberClasses() []*MemberClass {
	return mr.memberClasses
}

func (mr *memberReference) addMember(m *Member) {
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
	awareLayers mapset.Set[string]
}

func newAddressedObject() addressedObject {
	return addressedObject{
		awareLayers: mapset.NewSet[string](),
	}
}

func (a addressedObject) setAware(layer string) {
	a.awareLayers.Add(layer)
}

func (a addressedObject) IsAware(layer string) bool {
	return a.awareLayers.Contains(layer)
}

// NameSpacer includes Node, Interface, Neighbor, Group
type NameSpacer interface {
	setNumbered(k string)
	isNumbered(k string) bool
	iterNumbered() <-chan string
	addNumber(k, v string)
	hasNumber(k string) bool
	setNumbers(map[string]string)
	setRelativeNumber(k, v string)
	hasRelativeNumber(k string) bool
	setRelativeNumbers(map[string]string)
	GetNumbers() map[string]string
	GetRelativeNumbers() map[string]string
	GetValue(string) (string, error)
}

type NameSpace struct {
	numbered        mapset.Set[string]
	numbers         map[string]string
	relativeNumbers map[string]string
}

func newNameSpace() *NameSpace {
	return &NameSpace{
		numbered:        mapset.NewSet[string](),
		numbers:         map[string]string{},
		relativeNumbers: map[string]string{},
	}
}

func (ns *NameSpace) setNumbered(k string) {
	ns.numbered.Add(k)
}

func (ns *NameSpace) isNumbered(k string) bool {
	return ns.numbered.Contains(k)
}

func (ns *NameSpace) iterNumbered() <-chan string {
	return ns.numbered.Iter()
}

func (ns *NameSpace) addNumber(k, v string) {
	ns.numbers[k] = v
}

func (ns *NameSpace) hasNumber(k string) bool {
	_, ok := ns.numbers[k]
	return ok
}

func (ns *NameSpace) setNumbers(given map[string]string) {
	if len(ns.numbers) == 0 {
		ns.numbers = given
	} else {
		for k, v := range given {
			ns.numbers[k] = v
		}
	}
}

func (ns *NameSpace) GetNumbers() map[string]string {
	return ns.numbers
}

func (ns *NameSpace) setRelativeNumber(k, v string) {
	ns.relativeNumbers[k] = v
}

func (ns *NameSpace) hasRelativeNumber(k string) bool {
	_, ok := ns.relativeNumbers[k]
	return ok
}

func (ns *NameSpace) setRelativeNumbers(given map[string]string) {
	if len(ns.relativeNumbers) == 0 {
		ns.relativeNumbers = given
	} else {
		for k, v := range given {
			ns.relativeNumbers[k] = v
		}
	}
}

func (ns *NameSpace) GetRelativeNumbers() map[string]string {
	return ns.relativeNumbers
}

func (ns *NameSpace) GetValue(key string) (string, error) {
	val, ok := ns.relativeNumbers[key]
	if ok {
		return val, nil
	} else {
		fmt.Printf("%+v, %+v\n", key, ns)
		return val, fmt.Errorf("unknown key %v", key)
	}
}

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
	Nodes       []*Node
	Connections []*Connection
	Groups      []*Group

	nodeMap                  map[string]*Node
	groupMap                 map[string]*Group
	nodeClassMemberMap       classMemberMap
	interfaceClassMemberMap  classMemberMap
	connectionClassMemberMap classMemberMap
}

func (nm *NetworkModel) newNode(name string) *Node {
	node := &Node{
		Name:            name,
		NameSpace:       newNameSpace(),
		addressedObject: newAddressedObject(),
		interfaceMap:    map[string]*Interface{},
		memberReference: newMemberReference(),
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
		Name:      name,
		Nodes:     []*Node{},
		NameSpace: newNameSpace(),
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
	Name       string
	Interfaces []*Interface
	Groups     []*Group
	Virtual    bool
	Files      *ConfigFiles
	TinetAttr  *map[string]interface{}
	ClabAttr   *map[string]interface{}

	*NameSpace
	*parsedLabels
	*memberReference
	addressedObject

	namePrefix         string
	mgmtInterface      *Interface
	mgmtInterfaceClass *InterfaceClass
	interfaceMap       map[string]*Interface
}

func (n *Node) String() string {
	return n.Name
}

func (n *Node) newInterface(name string) *Interface {
	iface := &Interface{
		Name:             name,
		Node:             n,
		Neighbors:        map[string][]*Neighbor{},
		NameSpace:        newNameSpace(),
		addressedObject:  newAddressedObject(),
		memberReference:  newMemberReference(),
		hasNeighborClass: map[string]bool{},
	}
	n.Interfaces = append(n.Interfaces, iface)
	if name != "" {
		n.interfaceMap[iface.Name] = iface
	}
	return iface
}

func (n *Node) GetManagementInterface() *Interface {
	return n.mgmtInterface
}

func (n *Node) setAwareLayers(aware []string, defaults []string, ignoreDefaults bool) {
	var givenset mapset.Set[string]
	var defaultset mapset.Set[string]
	if ignoreDefaults {
		defaultset = mapset.NewSet[string]()
	} else {
		defaultset = mapset.NewSet(defaults...)
	}
	givenset = mapset.NewSet(aware...)

	n.addressedObject.awareLayers = defaultset.Union(givenset)
}

func (n *Node) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	nc, ok := cfg.nodeClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid NodeClass name %s", cls)
	}
	return nc, nil
}

func (n *Node) GivenIPLoopback(ipspace *IPSpaceDefinition) (string, bool) {
	for k, v := range n.valueLabels {
		if k == ipspace.IPLoopbackReplacer() {
			return v, true
		}
	}
	return "", false
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

	*NameSpace
	*parsedLabels
	*memberReference
	addressedObject

	hasNeighborClass map[string]bool // key: ipspace
	namePrefix       string
}

func (iface *Interface) String() string {
	return fmt.Sprintf("%s@%s", iface.Name, iface.Node.String())
}

func (iface *Interface) GivenIPAddress(ipspace *IPSpaceDefinition) (string, bool) {
	for k, v := range iface.valueLabels {
		if k == ipspace.IPAddressReplacer() {
			return v, true
		}
	}
	return "", false
}

func (iface *Interface) setAwareLayers(aware []string, defaults []string, ignoreNode bool, ignoreDefaults bool) {
	var givenset mapset.Set[string]
	var defaultset mapset.Set[string]
	if ignoreDefaults {
		defaultset = mapset.NewSet[string]()
	} else {
		defaultset = mapset.NewSet(defaults...)
	}
	givenset = mapset.NewSet(aware...)

	if ignoreNode {
		iface.addressedObject.awareLayers = defaultset.Union(givenset)
	} else {
		appendum := defaultset.Union(givenset)
		iface.addressedObject.awareLayers = appendum.Union(iface.Node.addressedObject.awareLayers)
	}
}

func (iface *Interface) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	ic, ok := cfg.interfaceClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid InterfaceClass name %s", cls)
	}
	return ic, nil
}

func (iface *Interface) addNeighbor(neighbor *Interface, ipspace string) {
	if _, ok := iface.hasNeighborClass[ipspace]; ok {
		n := &Neighbor{
			Self:      iface,
			Neighbor:  neighbor,
			NameSpace: newNameSpace(),
		}
		iface.Neighbors[ipspace] = append(iface.Neighbors[ipspace], n)
	}
}

type Connection struct {
	Src      *Interface
	Dst      *Interface
	IPSpaces mapset.Set[string]

	*parsedLabels
}

func (conn *Connection) String() string {
	return fmt.Sprintf("%s--%s", conn.Src.String(), conn.Dst.String())
}

func (conn *Connection) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	cc, ok := cfg.connectionClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid ConnectionClass name %s", cls)
	}
	return cc, nil
}

func (conn *Connection) GivenIPNetwork(ipspace *IPSpaceDefinition) (string, bool) {
	for k, v := range conn.valueLabels {
		if k == ipspace.IPNetworkReplacer() {
			return v, true
		}
	}
	return "", false
}

type Neighbor struct {
	Self     *Interface
	Neighbor *Interface

	*NameSpace
}

type Member struct {
	ClassName string
	ClassType string
	Referer   memberReferer
	Member    NameSpacer

	*NameSpace
}

type Group struct {
	Name  string
	Nodes []*Node

	*NameSpace
	*parsedLabels

	//numbered mapset.Set[string]
}

func (g *Group) ClassDefinition(cfg *Config, cls string) (interface{}, error) {
	gc, ok := cfg.groupClassMap[cls]
	if !ok {
		return nil, fmt.Errorf("invalid GroupClass name %s", cls)
	}
	return gc, nil
}
