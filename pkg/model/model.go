package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

// const IPPolicyTypeDefault string = "ip"
// const IPPolicyTypeLoopback string = "loopback"

// const DefaultNodePrefix string = "node"
// const DefaultInterfacePrefix string = "net"

const ManagementInterfaceName string = "mgmt"

const NumberReplacerName string = "name"
const ManagementLayerReplacer string = "mgmt"

const NumberAS string = "as"
const NumberNumber string = "number"

// const DummyIPSpace string = "none"

func BuildNetworkModel(cfg *types.Config, d *Diagram, verbose bool) (nm *types.NetworkModel, err error) {

	err = LoadModules(cfg)
	if err != nil {
		return nil, err
	}

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
		err = assignNodeNames(nm)
		if err != nil {
			return nil, err
		}
	}
	err = assignInterfaceNames(nm)
	if err != nil {
		return nil, err
	}
	err = assignConnectionNames(nm)
	if err != nil {
		return nil, err
	}

	// assign numbers, interface names and addresses
	err = setGivenParameters(nm)
	if err != nil {
		return nil, err
	}

	err = generateModuleParameters(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = assignIPParameters(cfg, nm, verbose)
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

	// // build config commands from config templates
	// cfg, err = loadTemplates(cfg)
	// if err != nil {
	// 	return nil, err
	// }

	// err = generateConfigFiles(cfg, nm, output)
	// if err != nil {
	// 	return nil, err
	// }

	return nm, err
}

func BuildConfigFiles(cfg *types.Config, nm *types.NetworkModel, verbose bool) error {
	// build config commands from config templates

	err := checkModuleRequirements(cfg, nm)
	if err != nil {
		return err
	}

	cfg, err = types.LoadTemplates(cfg)
	if err != nil {
		return err
	}

	err = generateConfigFiles(cfg, nm, verbose)
	if err != nil {
		return err
	}

	return nil
}

func buildSkeleton(cfg *types.Config, d *Diagram) (*types.NetworkModel, error) {
	nm := types.NewNetworkModel()
	nm.Name = cfg.Name
	nm.Classes = cfg.NetworkClasses
	// nm := &NetworkModel{
	// 	NetworkSegments:          map[string][]*SegmentMembers{},
	// 	nodeMap:                  map[string]*Node{},
	// 	groupMap:                 map[string]*Group{},
	// 	nodeClassMemberMap:       classMemberMap{mapper: map[string][]NameSpacer{}},
	// 	interfaceClassMemberMap:  classMemberMap{mapper: map[string][]NameSpacer{}},
	// 	connectionClassMemberMap: classMemberMap{mapper: map[string][]NameSpacer{}},
	// }

	ifaceCounter := map[string]int{}
	for _, e := range d.graph.Edges.Edges {
		ifaceCounter[e.Src]++
		ifaceCounter[e.Dst]++
	}

	nm.Groups = make([]*types.Group, 0, len(d.graph.SubGraphs.SubGraphs))
	for _, s := range d.graph.SubGraphs.SubGraphs {
		group := nm.NewGroup(s.Name)
		group.SetLabels(cfg, getSubGraphLabels(s), []string{})
	}

	nm.Nodes = make([]*types.Node, 0, len(d.graph.Nodes.Nodes))
	for _, n := range d.SortedNodes() {
		node := nm.NewNode(n.Name)
		// Note: node.Name can be overwritten later if nodeautoname = true
		// but the name must be DOTID in this function to keep consistency with other graph objects
		err := node.SetLabels(cfg, getNodeLabels(n), getModuleNodeClassLabels(cfg))
		if err != nil {
			return nil, err
		}
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

	nm.Connections = make([]*types.Connection, 0, len(d.graph.Edges.Edges))
	for _, e := range d.SortedLinks() {
		labels, srcLabels, dstLabels := getEdgeLabels(e)

		srcNode, ok := nm.NodeByName(e.Src)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := srcNode.InterfaceByName(e.SrcPort); ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.SrcPort)
		}
		// new interface
		// interface name can be blank (automatically named later)
		srcIf := srcNode.NewInterface(strings.TrimLeft(e.SrcPort, ":"))
		err := srcIf.SetLabels(cfg, srcLabels, getModuleInterfaceClassLabels(cfg))
		if err != nil {
			return nil, err
		}

		dstNode, ok := nm.NodeByName(e.Dst)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := dstNode.InterfaceByName(e.DstPort); ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.DstPort)
		}
		dstIf := dstNode.NewInterface(strings.TrimLeft(e.DstPort, ":"))
		err = dstIf.SetLabels(cfg, dstLabels, getModuleInterfaceClassLabels(cfg))
		if err != nil {
			return nil, err
		}

		srcIf.Opposite = dstIf
		dstIf.Opposite = srcIf

		conn := nm.NewConnection(srcIf, dstIf)
		err = conn.SetLabels(cfg, labels, getModuleConnectionClassLabels(cfg))
		if err != nil {
			return nil, err
		}
		// relational class label for interfaces
		for _, rlabel := range conn.RelationalClassLabels() {
			if rlabel.ClassType == types.ClassTypeInterface {
				srcIf.AddClassLabels(rlabel.Name)
				dstIf.AddClassLabels(rlabel.Name)
			}
		}

		if len(conn.PlaceLabels()) > 0 {
			return nil, fmt.Errorf("connection cannot have placeLabels")
		}
		//if (len(srcIf.Labels.ClassLabels) == 0 || len(dstIf.Labels.ClassLabels) == 0) && len(conn.Labels.ClassLabels) == 0 {
		//	return nil, fmt.Errorf("set default interfaceclass or connectionclass to leave links unlabeled")
		//}
	}

	return nm, nil
}

func checkClasses(cfg *types.Config, nm *types.NetworkModel) error {
	/*
		- check primary class consistency
		- store primary class attributes on objects
		- check flags (IPAware, Numbered and IPSpaces)
	*/

	// defaultConnectionLayer := cfg.DefaultConnectionLayer()

	// var primaryNC string
	// primaryICMap := map[*types.Interface]string{}

	// check nodes
	for _, node := range nm.Nodes {
		node.SetClasses(cfg, nm)
		// 		primaryNC = ""
		//
		// 		// set defaults for nodes without primary class
		// 		node.NamePrefix = DefaultNodePrefix
		//
		// 		// check nodeclass flags
		// 		for _, cls := range node.ClassLabels() {
		// 			nc, ok := cfg.NodeClassByName(cls)
		// 			if !ok {
		// 				return fmt.Errorf("invalid nodeclass name %s", cls)
		// 			}
		// 			node.ParsedLabels.Classes = append(node.ParsedLabels.Classes, nc)
		// 			nm.nodeClassMemberMap.addClassMember(nc.Name, node)
		//
		// 			// check virtual
		// 			if nc.Virtual {
		// 				node.Virtual = true
		// 			}
		//
		// 			// check ippolicy flags
		// 			for _, p := range nc.IPPolicy {
		// 				policy, ok := cfg.policyMap[p]
		// 				if ok {
		// 					node.setPolicy(policy.layer, policy)
		// 				} else {
		// 					return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
		// 				}
		// 			}
		//
		// 			// check interface_policy flags
		// 			for _, p := range nc.InterfaceIPPolicy {
		// 				policy, ok := cfg.policyMap[p]
		// 				if ok {
		// 					for _, iface := range node.Interfaces {
		// 						iface.setPolicy(policy.layer, policy)
		// 					}
		// 				} else {
		// 					return fmt.Errorf("invalid policy name %s in nodeclass %s", p, nc.Name)
		// 				}
		// 			}
		//
		// 			// check parameter flags
		// 			for _, num := range nc.Parameters {
		// 				policy, ok := cfg.policyMap[num]
		// 				if ok {
		// 					// ip policy
		// 					node.setPolicy(policy.layer, policy)
		// 				} else {
		// 					node.setParamFlag(num)
		// 				}
		// 			}
		//
		// 			// check MemberClasses
		// 			for i := range nc.MemberClasses {
		// 				node.addMemberClass(nc.MemberClasses[i])
		// 			}
		//
		// 			// check primary node class consistency
		// 			if nc.Primary {
		// 				if primaryNC == "" {
		// 					primaryNC = nc.Name
		// 				} else {
		// 					return fmt.Errorf("multiple primary node classes on one node (%s, %s)", primaryNC, nc.Name)
		// 				}
		// 				// add parameters of primary node class
		// 				if node.NamePrefix != "" {
		// 					node.NamePrefix = nc.Prefix
		// 				}
		// 				if nc.MgmtInterface != "" {
		// 					if mgmtnc, ok := cfg.InterfaceClassByName(nc.MgmtInterface); ok {
		// 						node.MgmtInterfaceClass = mgmtnc
		// 					} else {
		// 						return fmt.Errorf("invalid mgmt interface class name %s", nc.MgmtInterface)
		// 					}
		// 				}
		// 				node.TinetAttr = &nc.TinetAttr
		// 				node.ClabAttr = &nc.ClabAttr
		// 			} else {
		// 				if nc.Prefix != "" {
		// 					return fmt.Errorf("prefix can be specified only in primary class")
		// 				}
		// 				if nc.MgmtInterface != "" {
		// 					return fmt.Errorf("mgmt inteface class can be specified only in primary class")
		// 				}
		// 				if len(nc.TinetAttr) > 0 || len(nc.ClabAttr) > 0 {
		// 					return fmt.Errorf("output-specific attributes can be specified only in primary class")
		// 				}
		// 			}
		// 		}
		//
		// 		if primaryNC == "" && !node.Virtual {
		// 			fmt.Fprintf(os.Stderr, "warning: no primary node class on node %s\n", node.Name)
		// 		}
	}

	// check connections
	for _, conn := range nm.Connections {

		conn.SetClasses(cfg, nm)

		// 		for _, layer := range defaultConnectionLayer {
		// 			conn.Layers.Add(layer)
		// 		}
		//
		// 		// check connectionclass flags to connections and their interfaces
		// 		for _, cls := range conn.classLabels {
		// 			cc, ok := cfg.connectionClassMap[cls]
		// 			if !ok {
		// 				return fmt.Errorf("invalid connectionclass name %s", cls)
		// 			}
		// 			conn.parsedLabels.classes = append(conn.parsedLabels.classes, cc)
		//
		// 			// connected layer
		// 			for _, layer := range cc.Layers {
		// 				conn.Layers.Add(layer)
		// 			}
		//
		// 			// check primary interface class consistency
		// 			if cc.Primary {
		// 				if name, exists := primaryICMap[conn.Src]; !exists {
		// 					primaryICMap[conn.Src] = cc.Name
		// 				} else {
		// 					return fmt.Errorf("multiple primary interface/connection classes on one node (%s, %s)", name, cc.Name)
		// 				}
		// 				conn.Src.TinetAttr = &cc.TinetAttr
		// 				// conn.Src.ClabAttr = &cc.ClabAttr
		// 				if name, exists := primaryICMap[conn.Dst]; !exists {
		// 					primaryICMap[conn.Dst] = cc.Name
		// 				} else {
		// 					return fmt.Errorf("multiple primary interface/connection classes on one node (%s, %s)", name, cc.Name)
		// 				}
		// 				conn.Dst.TinetAttr = &cc.TinetAttr
		// 				// conn.Dst.ClabAttr = &cc.ClabAttr
		// 				if cc.Prefix != "" {
		// 					conn.Src.namePrefix = cc.Prefix
		// 					conn.Dst.namePrefix = cc.Prefix
		// 				}
		// 			} else {
		// 				if cc.Prefix != "" {
		// 					return fmt.Errorf("prefix can be specified only in primary class")
		// 				}
		// 				if len(cc.TinetAttr) > 0 || len(cc.ClabAttr) > 0 {
		// 					return fmt.Errorf("output-specific attributes can be specified only in primary class")
		// 				}
		// 			}
		//		}
	}

	// check interfaces
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			iface.SetClasses(cfg, nm)
			// 			// set virtual flag to interfaces of virtual nodes as default
			// 			iface.Virtual = node.Virtual
			//
			// 			// set defaults for interfaces without primary class
			// 			iface.namePrefix = DefaultInterfacePrefix
			//
			// 			// check connectionclass flags
			// 			for _, cls := range iface.Connection.classLabels {
			// 				cc, ok := cfg.connectionClassMap[cls]
			// 				if !ok {
			// 					return fmt.Errorf("invalid connectionclass name %s", cls)
			// 				}
			// 				nm.connectionClassMemberMap.addClassMember(cc.Name, iface)
			//
			// 				// check virtual
			// 				iface.Virtual = iface.Virtual || cc.Virtual
			//
			// 				// check ippolicy flags
			// 				for _, p := range cc.IPPolicy {
			// 					policy, ok := cfg.policyMap[p]
			// 					if ok {
			// 						iface.setPolicy(policy.layer, policy)
			// 					} else {
			// 						return fmt.Errorf("invalid policy name %s in connectionclass %s", p, cc.Name)
			// 					}
			// 				}
			//
			// 				// check parameter flags
			// 				for _, num := range cc.Parameters {
			// 					policy, ok := cfg.policyMap[num]
			// 					if ok {
			// 						// ip policy
			// 						iface.setPolicy(policy.layer, policy)
			// 					} else {
			// 						iface.setParamFlag(num)
			// 					}
			// 				}
			//
			// 				// add neighbor classes
			// 				for _, nc := range cc.NeighborClasses {
			// 					iface.neighborClassMap[nc.Layer] = append(iface.neighborClassMap[nc.Layer], nc)
			// 					// iface.hasNeighborClass[nc.Layer] = true
			// 				}
			//
			// 				// check MemberClasses
			// 				for i := range cc.MemberClasses {
			// 					iface.addMemberClass(cc.MemberClasses[i])
			// 				}
			//
			// 			}
			//
			// 			// check interfaceclass flags
			// 			for _, cls := range iface.classLabels {
			// 				ic, ok := cfg.interfaceClassMap[cls]
			// 				if !ok {
			// 					return fmt.Errorf("invalid interfaceclass name %s", cls)
			// 				}
			// 				iface.parsedLabels.classes = append(iface.parsedLabels.classes, ic)
			// 				nm.interfaceClassMemberMap.addClassMember(ic.Name, iface)
			//
			// 				// check virtual
			// 				iface.Virtual = iface.Virtual || ic.Virtual
			//
			// 				// check ippolicy flags
			// 				for _, p := range ic.IPPolicy {
			// 					policy, ok := cfg.policyMap[p]
			// 					if ok {
			// 						iface.setPolicy(policy.layer, policy)
			// 					} else {
			// 						return fmt.Errorf("invalid policy name %s in interfaceclass %s", p, ic.Name)
			// 					}
			// 				}
			//
			// 				// check parameter flags
			// 				for _, num := range ic.Parameters {
			// 					policy, ok := cfg.policyMap[num]
			// 					if ok {
			// 						// ip policy
			// 						iface.setPolicy(policy.layer, policy)
			// 					} else {
			// 						iface.setParamFlag(num)
			// 					}
			// 				}
			//
			// 				// check neighbor classes
			// 				for _, nc := range ic.NeighborClasses {
			// 					iface.neighborClassMap[nc.Layer] = append(iface.neighborClassMap[nc.Layer], nc)
			// 					// iface.hasNeighborClass[nc.Layer] = true
			// 				}
			//
			// 				// check MemberClasses
			// 				for i := range ic.MemberClasses {
			// 					iface.addMemberClass(ic.MemberClasses[i])
			// 				}
			//
			// 				// check primary interface class consistency
			// 				if ic.Primary {
			// 					if picname, exists := primaryICMap[iface]; !exists {
			// 						primaryICMap[iface] = ic.Name
			// 					} else {
			// 						return fmt.Errorf("multiple primary interface classes on one node (%s, %s)", picname, ic.Name)
			// 					}
			// 					if ic.Prefix != "" {
			// 						iface.namePrefix = ic.Prefix
			// 					}
			// 					iface.TinetAttr = &ic.TinetAttr
			// 					// iface.ClabAttr = &ic.ClabAttr
			// 				} else {
			// 					if ic.Prefix != "" {
			// 						return fmt.Errorf("prefix can be specified only in primary class")
			// 					}
			// 					if len(ic.TinetAttr) > 0 || len(ic.ClabAttr) > 0 {
			// 						return fmt.Errorf("output-specific attributes can be specified only in primary class")
			// 					}
			// 				}
			// 			}
		}
	}

	for _, group := range nm.Groups {
		group.SetClasses(cfg, nm)
		// 		// check groupclass flags to groups
		// 		for _, cls := range group.classLabels {
		// 			gc, ok := cfg.groupClassMap[cls]
		// 			if !ok {
		// 				return fmt.Errorf("invalid GroupClass name %s", cls)
		// 			}
		//
		// 			// check numbered
		// 			for _, num := range gc.Parameters {
		// 				group.setParamFlag(num)
		// 			}
		// 		}
	}

	// add class members to member referrers
	for _, mr := range nm.MemberReferrers() {
		for _, mc := range mr.GetMemberClasses() {
			classtype, classes, err := mc.GetSpecifiedClasses()
			if err != nil {
				return err
			}

			for _, cls := range classes {
				var members []types.NameSpacer
				switch classtype {
				case types.ClassTypeNode:
					members = nm.NodeClassMembers(cls)
				case types.ClassTypeInterface:
					members = nm.InterfaceClassMembers(cls)
				case types.ClassTypeConnection:
					members = nm.ConnectionClassMembers(cls)
				}
				if len(members) == 0 {
					fmt.Fprintf(os.Stderr, "warning: class %s has no members\n", cls)
					// return fmt.Errorf("class %v has no members", cls)
				}
				for _, memberObject := range members {
					member := types.NewMember(cls, classtype, memberObject, mr)
					mr.AddMember(member)
				}
			}
		}
	}

	return nil
}

func addSpecialInterfaces(cfg *types.Config, nm *types.NetworkModel) error {
	if cfg.HasManagementLayer() {
		// set mgmt interfaces on nodes
		name := cfg.ManagementLayer.InterfaceName
		if name == "" {
			name = ManagementInterfaceName
		}
		for _, node := range nm.Nodes {
			_, err := node.CreateManagementInterface(cfg, name)
			if err != nil {
				return err
			}

			// 			iface.SetLabels(cfg, []string{ic.Name})
			// 			if ic := node.GetManagementInterfaceClass(); ic != nil {
			//
			// 				// check that mgmtInterfaceClass is not used in topology
			// 				for _, iface := range node.Interfaces {
			// 					for _, cls := range iface.ClassLabels() {
			// 						if cls == ic.Name {
			// 							return fmt.Errorf("mgmt InterfaceClass should not be specified in topology graph (automatically added)")
			// 						}
			// 					}
			// 				}
			//
			// 				// add management interface
			// 				iface := node.NewInterface(name)
			// 				iface.SetLabels(cfg, []string{ic.Name})
			// 				// iface.parsedLabels = newParsedLabels()
			// 				// iface.parsedLabels.classLabels = append(iface.parsedLabels.classLabels, ic.Name)
			// 				node.mgmtInterface = iface
			// 			}
		}
	}
	return nil
}

// assignNodeNames assign names for unnamed nodes with given name prefix automatically
func assignNodeNames(nm *types.NetworkModel) error {
	prefixMap := map[string][]*types.Node{}
	for _, node := range nm.Nodes {
		prefixMap[node.NamePrefix] = append(prefixMap[node.NamePrefix], node)
	}

	for prefix, nodes := range prefixMap {
		for i, node := range nodes {
			oldName := node.Name
			node.Name = prefix + strconv.Itoa(i+1) // starts with 1
			nm.RenameNode(node, oldName, node.Name)
		}
	}

	return nil
}

// assignInterfaceNames assign names for unnamed interfaces with given name prefix automatically
func assignInterfaceNames(nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		existingNames := map[string]struct{}{}
		prefixMap := map[string][]*types.Interface{} // Interfaces to be named automatically
		for _, iface := range node.Interfaces {
			if iface.Name == "" {
				prefixMap[iface.NamePrefix] = append(prefixMap[iface.NamePrefix], iface)
			} else {
				existingNames[iface.Name] = struct{}{}
			}
		}
		for prefix, interfaces := range prefixMap {
			i := 0
			for _, iface := range interfaces {
				oldName := iface.Name
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
				iface.Node.RenameInterface(iface, oldName, iface.Name)
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

func assignConnectionNames(nm *types.NetworkModel) error {
	existingNames := map[string]struct{}{}
	prefixMap := map[string][]*types.Connection{} // Connections to be named automatically
	
	for _, conn := range nm.Connections {
		if conn.Name == "" {
			// Get prefix from ConnectionClass
			prefix := types.DefaultConnectionPrefix
			for _, cls := range conn.GetClasses() {
				cc := cls.(*types.ConnectionClass)
				if cc.Prefix != "" {
					prefix = cc.Prefix
					break
				}
			}
			prefixMap[prefix] = append(prefixMap[prefix], conn)
		} else {
			existingNames[conn.Name] = struct{}{}
		}
	}
	
	for prefix, connections := range prefixMap {
		i := 0
		for _, conn := range connections {
			var name string
			for { // avoid existing names
				name = prefix + strconv.Itoa(i)
				_, exists := existingNames[name]
				if !exists {
					break
				}
				i++ // starts with 0, increment by loop
			}
			conn.Name = name
			existingNames[conn.Name] = struct{}{}
			i++
		}
	}
	
	// confirm all connections are named
	for _, conn := range nm.Connections {
		if conn.Name == "" {
			return fmt.Errorf("there still exists unnamed connections after assignConnectionNames")
		}
	}
	
	return nil
}

func setGivenParameters(nm *types.NetworkModel) error {
	// add parameters only when no same key in namespace
	addParam := func(lo types.LabelOwner, k string, v string) error {
		switch obj := lo.(type) {
		case types.NameSpacer: // *Node, *Interface, *Group
			if !obj.HasParam(k) {
				obj.AddParam(k, v)
			}
		case *types.Connection:
			if !obj.Src.HasParam(k) {
				obj.Src.AddParam(k, v)
			}
			if !obj.Dst.HasParam(k) {
				obj.Dst.AddParam(k, v)
			}
		default:
			return fmt.Errorf("unexpected type %T for setGivenParameters", lo)
		}
		return nil
	}

	for _, lo := range nm.LabelOwners() {
		// set values in ValueLabels
		for k, v := range lo.ValueLabels() {
			err := addParam(lo, k, v)
			if err != nil {
				return err
			}
		}

		// set values in config
		for _, cls := range lo.GetClasses() {
			if loClass, ok := cls.(types.LabelOwnerClass); ok {
				values := loClass.GetGivenValues()
				for k, v := range values {
					err := addParam(lo, k, v)
					if err != nil {
						return err
					}
				}
			} else {
				return fmt.Errorf("unexpected class type %T for setGivenParameters", cls)
			}
		}
	}

	return nil

	// // set values in ValueLabels
	// for _, node := range nm.Nodes {
	// 	for k, v := range node.valueLabels {
	// 		// check existance (ip numbers may already added)
	// 		if !node.hasParam(k) {
	// 			node.addParam(k, v)
	// 		}
	// 	}
	// 	for _, iface := range node.Interfaces {
	// 		for k, v := range iface.valueLabels {
	// 			// check existance (ip numbers may already added)
	// 			if !iface.hasParam(k) {
	// 				iface.addParam(k, v)
	// 			}
	// 		}
	// 	}
	// }
	// for _, conn := range nm.Connections {
	// 	for k, v := range conn.valueLabels {
	// 		if !conn.Src.hasParam(k) {
	// 			conn.Src.addParam(k, v)
	// 		}
	// 		if !conn.Dst.hasParam(k) {
	// 			conn.Dst.addParam(k, v)
	// 		}
	// 	}
	// }
	// for _, group := range nm.Groups {
	// 	for k, v := range group.valueLabels {
	// 		if !group.hasParam(k) {
	// 			group.addParam(k, v)
	// 		}
	// 	}
	// }
	// return nil
}

func assignIPParameters(cfg *types.Config, nm *types.NetworkModel, verbose bool) error {
	if cfg.HasManagementLayer() {
		err := assignManagementIPAddresses(cfg, nm)
		if err != nil {
			return err
		}
	}

	for _, layer := range cfg.Layers {
		// loopback
		err := assignIPLoopbacks(nm, layer)
		if err != nil {
			return err
		}

		// determine network segment
		segs, err := searchSegments(nm, layer, verbose)
		if err != nil {
			return err
		}
		if len(segs) > 0 {
			nm.NetworkSegments[layer.Name] = segs
		}
		for _, seg := range segs {
			err := seg.SetSegmentLabelsFromRelationalLabels(cfg, layer)
			if err != nil {
				return err
			}
		}
		setNeighbors(segs, layer)

		// assign ip addresses
		err = assignIPAddresses(nm, layer)
		if err != nil {
			return err
		}
	}
	return nil
}

func assignParameters(cfg *types.Config, nm *types.NetworkModel) error {

	err := assignNetworkParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignNodeParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignInterfaceParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignGroupParameters(cfg, nm)

	return err
}
