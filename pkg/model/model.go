package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const IPPolicyTypeDefault string = "ip"
const IPPolicyTypeLoopback string = "loopback"

const DefaultNodePrefix string = "node"
const DefaultInterfacePrefix string = "net"
const ManagementInterfaceName string = "mgmt"

const NumberReplacerName string = "name"
const ManagementLayerReplacer string = "mgmt"

const NumberAS string = "as"
const NumberNumber string = "number"

// const DummyIPSpace string = "none"

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
	err = setGivenParameters(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = assignIPParameters(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = assignParameters(cfg, nm)
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
	nm := &NetworkModel{
		NetworkSegments:          map[string][]*SegmentMembers{},
		nodeMap:                  map[string]*Node{},
		groupMap:                 map[string]*Group{},
		nodeClassMemberMap:       classMemberMap{mapper: map[string][]NameSpacer{}},
		interfaceClassMemberMap:  classMemberMap{mapper: map[string][]NameSpacer{}},
		connectionClassMemberMap: classMemberMap{mapper: map[string][]NameSpacer{}},
	}

	ifaceCounter := map[string]int{}
	for _, e := range d.graph.Edges.Edges {
		ifaceCounter[e.Src]++
		ifaceCounter[e.Dst]++
	}

	nm.Groups = make([]*Group, 0, len(d.graph.SubGraphs.SubGraphs))
	for _, s := range d.graph.SubGraphs.SubGraphs {
		group := nm.newGroup(s.Name)
		group.parsedLabels = cfg.getValidGroupClasses(getSubGraphLabels(s))
	}

	nm.Nodes = make([]*Node, 0, len(d.graph.Nodes.Nodes))
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

	defaultConnectionLayer := cfg.DefaultConnectionLayer()

	var primaryNC string
	primaryICMap := map[*Interface]string{}

	// check nodes
	for _, node := range nm.Nodes {
		primaryNC = ""

		// set defaults for nodes without primary class
		node.namePrefix = DefaultNodePrefix

		// check nodeclass flags
		for _, cls := range node.classLabels {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid nodeclass name %s", cls)
			}
			node.parsedLabels.classes = append(node.parsedLabels.classes, nc)
			nm.nodeClassMemberMap.addClassMember(nc.Name, node)

			// check virtual
			if nc.Virtual {
				node.Virtual = true
			}

			// check ippolicy flags
			for _, p := range nc.IPPolicy {
				policy, ok := cfg.policyMap[p]
				if ok {
					node.setPolicy(policy.layer, policy)
				} else {
					return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
				}
			}

			// check interface_policy flags
			for _, p := range nc.InterfaceIPPolicy {
				policy, ok := cfg.policyMap[p]
				if ok {
					for _, iface := range node.Interfaces {
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
					node.setPolicy(policy.layer, policy)
				} else {
					node.setNumbered(num)
				}
			}

			// check MemberClasses
			for i := range nc.MemberClasses {
				node.addMemberClass(nc.MemberClasses[i])
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

		if primaryNC == "" && !node.Virtual {
			fmt.Fprintf(os.Stderr, "warning: no primary node class on node %s\n", node.Name)
		}
	}

	// check connections
	for _, conn := range nm.Connections {

		for _, layer := range defaultConnectionLayer {
			conn.Layers.Add(layer)
		}

		// check connectionclass flags to connections and their interfaces
		for _, cls := range conn.classLabels {
			cc, ok := cfg.connectionClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid connectionclass name %s", cls)
			}
			conn.parsedLabels.classes = append(conn.parsedLabels.classes, cc)

			// connected layer
			for _, layer := range cc.Layers {
				conn.Layers.Add(layer)
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
			// set virtual flag to interfaces of virtual nodes as default
			iface.Virtual = node.Virtual

			// set defaults for interfaces without primary class
			iface.namePrefix = DefaultInterfacePrefix

			// check connectionclass flags
			for _, cls := range iface.Connection.classLabels {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return fmt.Errorf("invalid connectionclass name %s", cls)
				}
				nm.connectionClassMemberMap.addClassMember(cc.Name, iface)

				// check virtual
				iface.Virtual = iface.Virtual || cc.Virtual

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
						iface.setNumbered(num)
					}
				}

				// check neighbor classes
				for _, nc := range cc.NeighborClasses {
					iface.hasNeighborClass[nc.Layer] = true
				}

				// check MemberClasses
				for i := range cc.MemberClasses {
					iface.addMemberClass(cc.MemberClasses[i])
				}

			}

			// check interfaceclass flags
			for _, cls := range iface.classLabels {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return fmt.Errorf("invalid interfaceclass name %s", cls)
				}
				iface.parsedLabels.classes = append(iface.parsedLabels.classes, ic)
				nm.interfaceClassMemberMap.addClassMember(ic.Name, iface)

				// check virtual
				iface.Virtual = iface.Virtual || ic.Virtual

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
						iface.setNumbered(num)
					}
				}

				// check neighbor classes
				for _, nc := range ic.NeighborClasses {
					iface.hasNeighborClass[nc.Layer] = true
				}

				// check MemberClasses
				for i := range ic.MemberClasses {
					iface.addMemberClass(ic.MemberClasses[i])
				}

				// check primary interface class consistency
				if ic.Primary {
					if picname, exists := primaryICMap[iface]; !exists {
						primaryICMap[iface] = ic.Name
					} else {
						return fmt.Errorf("multiple primary interface classes on one node (%s, %s)", picname, ic.Name)
					}
					if ic.Prefix != "" {
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

	for _, group := range nm.Groups {
		// check groupclass flags to groups
		for _, cls := range group.classLabels {
			gc, ok := cfg.groupClassMap[cls]
			if !ok {
				return fmt.Errorf("invalid GroupClass name %s", cls)
			}

			// check numbered
			for _, num := range gc.Parameters {
				group.setNumbered(num)
			}
		}
	}

	return nil
}

func addSpecialInterfaces(cfg *Config, nm *NetworkModel) error {
	if cfg.HasManagementLayer() {
		// set mgmt interfaces on nodes
		name := cfg.ManagementLayer.InterfaceName
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
				iface.parsedLabels = newParsedLabels()
				iface.parsedLabels.classLabels = append(iface.parsedLabels.classLabels, ic.Name)
				node.mgmtInterface = iface
			}
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

func setGivenParameters(cfg *Config, nm *NetworkModel) error {
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
	return nil
}

func assignIPParameters(cfg *Config, nm *NetworkModel) error {
	if cfg.HasManagementLayer() {
		err := assignManagementIPAddresses(cfg, nm)
		if err != nil {
			return err
		}
	}

	for _, layer := range cfg.Layers {
		// loopback
		err := assignIPLoopbacks(cfg, nm, layer)
		if err != nil {
			return err
		}

		// determine network segment
		segs, err := searchSegments(nm, layer, false)
		// segs, err := searchSegments(nm, layer, true)
		if err != nil {
			return err
		}
		if len(segs) > 0 {
			nm.NetworkSegments[layer.Name] = segs
		}
		setNeighbors(segs, layer)

		// assign ip addresses
		err = assignIPAddresses(cfg, nm, layer)
		if err != nil {
			return err
		}
	}
	return nil
}

func assignParameters(cfg *Config, nm *NetworkModel) error {
	nodesForParams := map[string][]*Node{}
	interfacesForParams := map[string][]*Interface{}
	groupsForParams := map[string][]*Group{}

	// add object names as parameters, and list up objects for assignment
	for _, node := range nm.Nodes {
		node.addNumber(NumberReplacerName, node.Name)
		for num := range node.iterNumbered() {
			nodesForParams[num] = append(nodesForParams[num], node)
		}
		for _, iface := range node.Interfaces {
			iface.addNumber(NumberReplacerName, iface.Name)
			for num := range iface.iterNumbered() {
				interfacesForParams[num] = append(interfacesForParams[num], iface)
			}
		}
	}
	for _, group := range nm.Groups {
		group.addNumber(NumberReplacerName, group.Name)
		for num := range group.iterNumbered() {
			groupsForParams[num] = append(groupsForParams[num], group)
		}
	}

	// add node parameters
	for key, nodes := range nodesForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		params, err := getParameterCandidates(cfg, rule, len(nodes))
		if err != nil {
			return err
		}
		for i, obj := range nodes {
			obj.addNumber(key, params[i])
		}
	}

	// add interface parameters
	for key, ifaces := range interfacesForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		params, err := getParameterCandidates(cfg, rule, len(ifaces))
		if err != nil {
			return err
		}
		for i, obj := range ifaces {
			obj.addNumber(key, params[i])
		}
	}

	// add group parameters
	for key, groups := range groupsForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		params, err := getParameterCandidates(cfg, rule, len(groups))
		if err != nil {
			return err
		}
		for i, obj := range groups {
			obj.addNumber(key, params[i])
		}
	}

	return nil
}
