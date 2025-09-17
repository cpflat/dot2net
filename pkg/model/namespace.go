package model

import (
	"github.com/cpflat/dot2net/pkg/types"
)

const NumberSeparator string = "_"
const NumberPrefixNode string = "node" + NumberSeparator
const NumberPrefixConnection string = "conn" + NumberSeparator
const NumberPrefixGroup string = "group" + NumberSeparator
const NumberPrefixOppositeHeader string = "opp" + NumberSeparator
const NumberPrefixOppositeInterface string = "opp" + NumberSeparator
const NumberPrefixNeighbor string = "n" + NumberSeparator
const NumberPrefixMember string = "m" + NumberSeparator

const SelfConfigHeader string = "self" + NumberSeparator

const ChildNodesConfigHeader string = "nodes" + NumberSeparator
const ChildInterfacesConfigHeader string = "interfaces" + NumberSeparator
const ChildConnectionsConfigHeader string = "connections" + NumberSeparator
const ChildSegmentsConfigHeader string = "segments" + NumberSeparator
const ChildGroupsConfigHeader string = "groups" + NumberSeparator
const ChildNeighborsConfigHeader string = "neighbors" + NumberSeparator
const ChildMembersConfigHeader string = "members" + NumberSeparator

//const NumberPrefixOppositeNode string = "oppnode_"
//const NumberPrefixOppositeGroup string = "oppgroup_"

// func mergeMaps(m ...map[string]interface{}) map[string]interface{} {
// 	ans := make(map[string]interface{}, 0)
//
// 	for _, c := range m {
// 		for k, v := range c {
// 			ans[k] = v
// 		}
// 	}
// 	return ans
// }

// func checkPlaceLabelOwner(ns types.NameSpacer, o types.LabelOwner, globalParams map[string]map[string]string) error {
// 	for _, plabel := range o.PlaceLabels() {
// 		if _, exists := globalParams[plabel]; exists {
// 			return fmt.Errorf("duplicated PlaceLabels %+v", plabel)
// 		}
// 		globalParams[plabel] = map[string]string{}
//
// 		for k, v := range ns.GetParams() {
// 			globalParams[plabel][k] = v
// 		}
// 	}
// 	return nil
// }

// func initPlaceLabelNameSpace(nm *types.NetworkModel) (map[string]map[string]string, error) {
// 	// Search placelabels for global namespace
// 	globalParams := map[string]map[string]string{} // globalNumbers[PlaceLabel][NumberKey] = NumberValue
// 	for _, node := range nm.Nodes {
// 		checkPlaceLabelOwner(node, node, globalParams)
//
// 		for _, iface := range node.Interfaces {
// 			checkPlaceLabelOwner(iface, iface, globalParams)
// 		}
// 	}
// 	for _, group := range nm.Groups {
// 		checkPlaceLabelOwner(group, group, globalParams)
// 	}
//
// 	// Set numbers for place labels from global namespace
// 	for _, node := range nm.Nodes {
// 		for plabel, nums := range globalParams {
// 			for k, v := range nums {
// 				name := plabel + NumberSeparator + k
// 				node.SetRelativeParam(name, v)
// 			}
// 		}
// 		for _, iface := range node.Interfaces {
// 			for plabel, nums := range globalParams {
// 				for k, v := range nums {
// 					name := plabel + NumberSeparator + k
// 					iface.SetRelativeParam(name, v)
// 				}
// 			}
// 		}
// 	}
//
// 	return globalParams, nil
// }
//
// func setPlaceLabelNameSpace(ns types.NameSpacer, globalNumbers map[string]map[string]string) {
// 	// namespace of PlaceLabels is referrable from anywhere
// 	for plabel, nums := range globalNumbers {
// 		for k, v := range nums {
// 			name := plabel + NumberSeparator + k
// 			ns.SetRelativeParam(name, v)
// 		}
// 	}
// }
//
// func setMetaValueLabelNameSpace(ns types.NameSpacer, o types.LabelOwner, globaladdressOwner map[string]map[string]string) error {
// 	// if MetaValueLabel is given, a PlaceLabel namespace can be referrable with the MetaValueLabel
// 	for mvlabel, target := range o.MetaValueLabels() {
// 		nums, ok := globaladdressOwner[target]
// 		if !ok {
// 			return fmt.Errorf("unknown PlaceLabel %v (specified for MetaValueLabel %v)", target, mvlabel)
// 		}
// 		for k, v := range nums {
// 			name := mvlabel + NumberSeparator + k
// 			ns.SetRelativeParam(name, v)
// 		}
// 	}
// 	return nil
// }

// func setGroupNameSpace(ns types.NameSpacer, groups []*types.Group, opposite bool) {
// 	for _, group := range groups {
// 		// groups: smaller group is forward, larger group is backward
// 		for k, val := range group.GetParams() {
// 			// prioritize numbers by node-num > smaller-group-num > large-group-num
// 			var num string
// 			if opposite {
// 				num = NumberPrefixOppositeHeader + NumberPrefixGroup + k
// 			} else {
// 				num = NumberPrefixGroup + k
// 			}
// 			if !ns.HasRelativeParam(num) {
// 				ns.SetRelativeParam(num, val)
// 			}
//
// 			// alias for group classes (for multi-layer groups)
// 			for _, label := range group.ClassLabels() {
// 				var cnum string
// 				if opposite {
// 					cnum = NumberPrefixOppositeInterface + label + NumberSeparator + k
// 				} else {
// 					cnum = label + NumberSeparator + k
// 				}
// 				if !ns.HasRelativeParam(cnum) {
// 					ns.SetRelativeParam(cnum, val)
// 				}
// 			}
// 		}
// 	}
// }

// func setL2OppositeNameSpace(iface *types.Interface) {
// 	// opposite interface
// 	if iface.Connection != nil {
// 		oppIf := iface.Opposite
// 		for oppnum, val := range oppIf.GetParams() {
// 			num := NumberPrefixOppositeInterface + oppnum
// 			iface.SetRelativeParam(num, val)
// 		}
//
// 		// node of opposite interface
// 		oppNode := oppIf.Node
// 		for oppnnum, val := range oppNode.GetParams() {
// 			num := NumberPrefixOppositeHeader + NumberPrefixNode + oppnnum
// 			iface.SetRelativeParam(num, val)
// 		}
//
// 		for _, group := range oppNode.Groups {
// 			group.SetGroupRelativeParams(iface, NumberPrefixOppositeHeader)
// 		}
// 	}
// }
//
// func setNeighborNameSpace(iface *types.Interface) {
// 	for _, neighbors := range iface.Neighbors {
// 		for _, n := range neighbors {
// 			// base namespace same as original interface (n.self)
// 			for k, v := range n.Self.GetParams() {
// 				n.SetRelativeParam(k, v)
// 			}
//
// 			// base node namespace
// 			nodeNumbers := getNodeNameSpace(n.Self)
// 			for k, v := range nodeNumbers {
// 				n.SetRelativeParam(k, v)
// 			}
//
// 			// relative namespace (neighbor interface)
// 			for num, val := range n.Neighbor.GetParams() {
// 				name := NumberPrefixNeighbor + num
// 				n.SetRelativeParam(name, val)
// 			}
//
// 			// relative namespace (neighbor host)
// 			nodeNumbers = getNodeNameSpace(n.Neighbor)
// 			for num, val := range nodeNumbers {
// 				name := NumberPrefixNeighbor + num
// 				n.SetRelativeParam(name, val)
// 			}
// 		}
// 	}
// }
//
// func setMemberClassNameSpace(nm *types.NetworkModel, mr types.MemberReferrer) error {
// 	var nodeNameSpace map[string]string
// 	switch t := mr.(type) {
// 	case *types.Node:
// 		// pass
// 	case *types.Interface:
// 		nodeNameSpace = getNodeNameSpace(t)
// 	default:
// 		return fmt.Errorf("unknown memberReferer type: %v", t)
// 	}
//
// 	for _, mc := range mr.GetMemberClasses() {
// 		classtype, classes, err := mc.GetSpecifiedClasses()
// 		if err != nil {
// 			return err
// 		}
//
// 		for _, cls := range classes {
// 			var members []types.NameSpacer
// 			switch classtype {
// 			case types.ClassTypeNode:
// 				members = nm.NodeClassMembers(cls)
// 			case types.ClassTypeInterface:
// 				members = nm.InterfaceClassMembers(cls)
// 			case types.ClassTypeConnection:
// 				members = nm.ConnectionClassMembers(cls)
// 			}
// 			if len(members) == 0 {
// 				fmt.Fprintf(os.Stderr, "warning: class %s has no members\n", cls)
// 				// return fmt.Errorf("class %v has no members", cls)
// 			}
//
// 			for _, memberObject := range members {
// 				if !mc.IncludeSelf && memberObject == mr.(types.NameSpacer) {
// 					continue
// 				}
// 				member := types.NewMember(cls, classtype, memberObject, mr)
// 				// base namespace
// 				for k, v := range mr.GetParams() {
// 					member.SetRelativeParam(k, v)
// 				}
// 				// node namespace
// 				for k, v := range nodeNameSpace {
// 					member.SetRelativeParam(k, v)
// 				}
// 				// member namespace
// 				for k, v := range memberObject.GetParams() {
// 					key := NumberPrefixMember + k
// 					member.SetRelativeParam(key, v)
// 				}
// 				mr.AddMember(member)
// 			}
// 		}
// 	}
// 	return nil
// }
//
// func getNodeNameSpace(iface *types.Interface) map[string]string {
// 	nodeNumbers := map[string]string{}
// 	for nodenum, val := range iface.Node.GetParams() {
// 		num := NumberPrefixNode + nodenum
// 		nodeNumbers[num] = val
// 	}
// 	return nodeNumbers
// }

func makeRelativeNamespace(nm *types.NetworkModel) error {

	globals, err := types.InitGloballNameSpace(nm)
	if err != nil {
		return err
	}

	for _, ns := range nm.NameSpacers() {
		ns.BuildRelativeNameSpace(globals)
	}

	return nil

	// // Search placelabels for global namespace
	// globalNumbers, err := initPlaceLabelNameSpace(nm)
	// if err != nil {
	// 	return err
	// }

	// // network
	// err = nm.BuildRelativeNameSpace(globalNumbers)
	// if err != nil {
	// 	return err
	// }

	// // generate relative numbers
	// for _, node := range nm.Nodes {

	// 	// node self
	// 	for num, val := range node.GetParams() {
	// 		node.SetRelativeParam(num, val)
	// 	}

	// 	// node groups
	// 	for _, group := range node.Groups {
	// 		group.SetGroupRelativeParams(node, "")
	// 	}
	// 	// setGroupNameSpace(node, node.Groups, false)

	// 	// PlaceLabels
	// 	setPlaceLabelNameSpace(node, globalNumbers)

	// 	// MetaValueLabels
	// 	err = setMetaValueLabelNameSpace(node, node, globalNumbers)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	// member classes
	// 	err = setMemberClassNameSpace(nm, node)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	for _, iface := range node.Interfaces {

	// 		// interface self
	// 		for num, val := range iface.GetParams() {
	// 			iface.SetRelativeParam(num, val)
	// 		}

	// 		// parent node of the interface
	// 		nodeNumbers := getNodeNameSpace(iface)
	// 		iface.SetRelativeParams(nodeNumbers)

	// 		// node group of the interface
	// 		for _, group := range node.Groups {
	// 			group.SetGroupRelativeParams(iface, "")
	// 		}
	// 		// setGroupNameSpace(iface, node.Groups, false)

	// 		// L2 opposite interface
	// 		setL2OppositeNameSpace(iface)

	// 		// L3 neighbor interfaces
	// 		setNeighborNameSpace(iface)

	// 		// member classes
	// 		err = setMemberClassNameSpace(nm, iface)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		// PlaceLabels
	// 		setPlaceLabelNameSpace(iface, globalNumbers)

	// 		// MetaValueLabels
	// 		err = setMetaValueLabelNameSpace(iface, iface, globalNumbers)
	// 		if err != nil {
	// 			return err
	// 		}
	// 	}

	// 	for _, group := range node.Groups {
	// 		// group self
	// 		for num, val := range group.GetParams() {
	// 			group.SetRelativeParam(num, val)
	// 		}
	// 	}
	// }

	// return nil
}
