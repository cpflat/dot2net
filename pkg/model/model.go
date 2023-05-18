package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

const DefaultNodePrefix string = "node"
const DefaultInterfacePrefix string = "net"
const ManagementInterfaceName string = "mgmt"

const NumberReplacerName string = "name"

const NumberAS string = "as"

// const DummyIPSpace string = "none"

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
		NameSpace:       newNameSpace(),
		addressedObject: newAddressedObject(),
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
	parsedLabels
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
		hasNeighborClass: map[string]bool{},
	}
	n.Interfaces = append(n.Interfaces, iface)
	if name != "" {
		n.interfaceMap[iface.Name] = iface
	}
	return iface
}

func (n *Node) HasClass(name string) bool {
	for _, cls := range n.classLabels {
		if cls == name {
			return true
		}
	}
	return false
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
	parsedLabels
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

	parsedLabels
}

func (conn *Connection) String() string {
	return fmt.Sprintf("%s--%s", conn.Src.String(), conn.Dst.String())
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

type Group struct {
	Name  string
	Nodes []*Node

	*NameSpace
	parsedLabels

	//numbered mapset.Set[string]
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

	err = makeRelativeNamespace(nm)
	if err != nil {
		return nil, err
	}

	// build config commands from config templates
	cfg, err = loadTemplates(cfg)
	if err != nil {
		return nil, err
	}

	err = generateConfigFiles(cfg, nm, output)
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
		group.parsedLabels = cfg.getValidGroupClasses(getSubGraphLabels(s))
	}

	nm.Nodes = make([]*Node, 0, len(d.graph.Nodes.Nodes))
	nm.nodeMap = map[string]*Node{}
	for _, n := range d.SortedNodes() {
		node := nm.newNode(n.Name)
		// Note: node.Name can be overwritten later if nodeautoname = true
		// but the name must be DOTID in this function to keep consistency with other graph objects
		node.parsedLabels = cfg.getValidNodeClasses(getNodeLabels(n))
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
		srcIf.parsedLabels = cfg.getValidInterfaceClasses(srcLabels)

		dstNode, ok := nm.NodeByName(e.Dst)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := dstNode.interfaceMap[e.DstPort]; ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.DstPort)
		}
		dstIf := dstNode.newInterface(strings.TrimLeft(e.DstPort, ":"))
		dstIf.parsedLabels = cfg.getValidInterfaceClasses(dstLabels)

		srcIf.Opposite = dstIf
		dstIf.Opposite = srcIf

		conn := nm.newConnection(srcIf, dstIf)
		conn.parsedLabels = cfg.getValidConnectionClasses(labels)
		if len(conn.placeLabels) > 0 {
			return nil, fmt.Errorf("connection cannot have placeLabels")
		}
		//if (len(srcIf.Labels.ClassLabels) == 0 || len(dstIf.Labels.ClassLabels) == 0) && len(conn.Labels.ClassLabels) == 0 {
		//	return nil, fmt.Errorf("set default interfaceclass or connectionclass to leave links unlabeled")
		//}
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

	// check nodes
	for _, node := range nm.Nodes {
		primaryNC = ""
		nodeIPAware := []string{}
		nodeIPAwareIgnoreDefaults := false

		// set defaults for nodes without primary class
		node.namePrefix = DefaultNodePrefix

		// check nodeclass flags
		for _, cls := range node.classLabels {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid NodeClass name %s", cls)
			}

			// check virtual
			if nc.Virtual {
				node.Virtual = true
			}

			// check IP aware
			nodeIPAware = append(nodeIPAware, nc.IPAware...)
			if nc.IPAwareIgnoreDefaults {
				nodeIPAwareIgnoreDefaults = true
			}

			// check numbered
			for _, num := range nc.Numbered {
				node.setNumbered(num)
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
		node.setAwareLayers(nodeIPAware, defaultIPAware, nodeIPAwareIgnoreDefaults)

		if primaryNC == "" && !node.Virtual {
			fmt.Fprintf(os.Stderr, "warning: no primary node class on node %s\n", node.Name)
		}
	}

	// check connections
	for _, conn := range nm.Connections {

		for _, space := range defaultIPConnect {
			conn.IPSpaces.Add(space)
		}

		// check connectionclass flags to connections and their interfaces
		for _, cls := range conn.classLabels {
			cc, ok := cfg.connectionClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid ConnectionClass name %s", cls)
			}

			// connected ip spaces
			for _, space := range cc.IPSpaces {
				conn.IPSpaces.Add(space)
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

	// check interfaces
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			ifaceIPAware := []string{}
			ifaceIPAwareIgnoreNode := false
			ifaceIPAwareIgnoreDefaults := false

			// set virtual flag to interfaces of virtual nodes as default
			iface.Virtual = node.Virtual

			// set defaults for interfaces without primary class
			iface.namePrefix = DefaultInterfacePrefix

			// check connectionclass flags
			for _, cls := range iface.Connection.classLabels {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return fmt.Errorf("invalid ConnectionClass name %s", cls)
				}

				// check virtual
				iface.Virtual = iface.Virtual || cc.Virtual

				// aware ip spaces
				ifaceIPAware = append(ifaceIPAware, cc.IPAware...)
				ifaceIPAwareIgnoreNode = ifaceIPAwareIgnoreNode || cc.IPAwareIgnoreNode
				ifaceIPAwareIgnoreDefaults = ifaceIPAwareIgnoreDefaults || cc.IPAwareIgnoreDefaults

				// check numbered
				for _, num := range cc.Numbered {
					iface.setNumbered(num)
				}

				// check neighbor classes
				for _, nc := range cc.NeighborClasses {
					iface.hasNeighborClass[nc.IPSpace] = true
				}
			}

			// check interfaceclass flags
			for _, cls := range iface.classLabels {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return fmt.Errorf("invalid InterfaceClass name %s", cls)
				}

				// check virtual
				iface.Virtual = iface.Virtual || ic.Virtual

				// aware ip spaces
				ifaceIPAware = append(ifaceIPAware, ic.IPAware...)
				ifaceIPAwareIgnoreNode = ifaceIPAwareIgnoreNode || ic.IPAwareIgnoreNode
				ifaceIPAwareIgnoreDefaults = ifaceIPAwareIgnoreDefaults || ic.IPAwareIgnoreDefaults

				// check numbered
				for _, num := range ic.Numbered {
					iface.setNumbered(num)
				}

				// check neighbor classes
				for _, nc := range ic.NeighborClasses {
					iface.hasNeighborClass[nc.IPSpace] = true
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
			iface.setAwareLayers(ifaceIPAware, defaultIPAware, ifaceIPAwareIgnoreNode, ifaceIPAwareIgnoreDefaults)
		}
	}

	for _, group := range nm.Groups {
		// check groupclass flags to groups
		for _, cls := range group.classLabels {
			gc, ok := cfg.groupClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid GroupClass name %s", cls)
			}

			// check numbered
			for _, num := range gc.Numbered {
				group.setNumbered(num)
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
				for _, cls := range iface.classLabels {
					if cls == ic.Name {
						return fmt.Errorf("mgmt InterfaceClass should not be specified in topology graph (automatically added)")
					}
				}
			}

			// add management interface
			iface := node.newInterface(name)
			iface.parsedLabels = parsedLabels{
				classLabels: []string{ic.Name},
			}
			iface.setAware(cfg.mgmtIPSpace.Name)
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
		for k, v := range node.valueLabels {
			// check existance (ip numbers may already added)
			if !node.hasNumber(k) {
				node.addNumber(k, v)
			}
		}
		for _, iface := range node.Interfaces {
			for k, v := range iface.valueLabels {
				// check existance (ip numbers may already added)
				if !iface.hasNumber(k) {
					iface.addNumber(k, v)
				}
			}
		}
	}
	for _, conn := range nm.Connections {
		for k, v := range conn.valueLabels {
			if !conn.Src.hasNumber(k) {
				conn.Src.addNumber(k, v)
			}
			if !conn.Dst.hasNumber(k) {
				conn.Dst.addNumber(k, v)
			}
		}
	}
	for _, group := range nm.Groups {
		for k, v := range group.valueLabels {
			if !group.hasNumber(k) {
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
		for num := range node.iterNumbered() {
			nodesForNumbers[num] = append(nodesForNumbers[num], node)
		}
		for _, iface := range node.Interfaces {
			iface.addNumber(NumberReplacerName, iface.Name)
			for num := range iface.iterNumbered() {
				interfacesForNumbers[num] = append(interfacesForNumbers[num], iface)
			}
		}
	}
	for _, group := range nm.Groups {
		group.addNumber(NumberReplacerName, group.Name)
		for num := range group.iterNumbered() {
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
				if group.hasNumber(num) {
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
