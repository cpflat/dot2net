package model

import "fmt"

const NumberSeparator string = "_"
const NumberPrefixNode string = "node" + NumberSeparator
const NumberPrefixGroup string = "group" + NumberSeparator
const NumberPrefixOppositeHeader string = "opp" + NumberSeparator
const NumberPrefixOppositeInterface string = "opp" + NumberSeparator
const NumberPrefixNeighbor string = "n" + NumberSeparator

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

func checkPlaceLabelOwner(ns NameSpacer, o labelOwner, globalNumbers map[string]map[string]string) error {
	for _, plabel := range o.PlaceLabels() {
		if _, exists := globalNumbers[plabel]; exists {
			return fmt.Errorf("duplicated PlaceLabels %+v", plabel)
		}
		globalNumbers[plabel] = map[string]string{}

		for k, v := range ns.(*NameSpace).numbers {
			globalNumbers[plabel][k] = v
		}
	}
	return nil
}

func initPlaceLabelNameSpace(nm *NetworkModel) (map[string]map[string]string, error) {
	// Search placelabels for global namespace
	globalNumbers := map[string]map[string]string{} // globalNumbers[PlaceLabel][NumberKey] = NumberValue
	for _, node := range nm.Nodes {
		checkPlaceLabelOwner(node, node, globalNumbers)

		for _, iface := range node.Interfaces {
			checkPlaceLabelOwner(iface, iface, globalNumbers)
		}
	}
	for _, group := range nm.Groups {
		checkPlaceLabelOwner(group, group, globalNumbers)
	}

	// Set numbers for place labels from global namespace
	for _, node := range nm.Nodes {
		for plabel, nums := range globalNumbers {
			for k, v := range nums {
				name := plabel + NumberSeparator + k
				node.setRelativeNumber(name, v)
			}
		}
		for _, iface := range node.Interfaces {
			for plabel, nums := range globalNumbers {
				for k, v := range nums {
					name := plabel + NumberSeparator + k
					iface.setRelativeNumber(name, v)
				}
			}
		}
	}

	return globalNumbers, nil
}

func setPlaceLabelNameSpace(ns NameSpacer, globalNumbers map[string]map[string]string) {
	// namespace of PlaceLabels is referrable from anywhere
	for plabel, nums := range globalNumbers {
		for k, v := range nums {
			name := plabel + NumberSeparator + k
			ns.setRelativeNumber(name, v)
		}
	}
}

func setMetaValueLabelNameSpace(ns NameSpacer, o labelOwner, globaladdressOwner map[string]map[string]string) error {
	// if MetaValueLabel is given, a PlaceLabel namespace can be referrable with the MetaValueLabel
	for mvlabel, target := range o.MetaValueLabels() {
		nums, ok := globaladdressOwner[target]
		if !ok {
			return fmt.Errorf("unknown PlaceLabel %v (specified for MetaValueLabel %v)", target, mvlabel)
		}
		for k, v := range nums {
			name := mvlabel + NumberSeparator + k
			ns.setRelativeNumber(name, v)
		}
	}
	return nil
}

func setGroupNameSpace(ns NameSpacer, groups []*Group, opposite bool) {
	for _, group := range groups {
		// groups: smaller group is forward, larger group is backward
		for k, val := range group.NameSpace.numbers {
			// prioritize numbers by node-num > smaller-group-num > large-group-num
			var num string
			if opposite {
				num = NumberPrefixOppositeHeader + NumberPrefixNode + k
			} else {
				num = NumberPrefixGroup + k
			}
			if ns.hasRelativeNumber(num) {
				ns.setRelativeNumber(num, val)
			}

			// alias for group classes (for multi-layer groups)
			for _, label := range group.classLabels {
				var cnum string
				if opposite {
					cnum = NumberPrefixOppositeInterface + label + NumberSeparator + k
				} else {
					cnum = label + NumberSeparator + k
				}
				if ns.hasRelativeNumber(cnum) {
					ns.setRelativeNumber(cnum, val)
				}
			}
		}
	}
}

func setL2OppositeNameSpace(iface *Interface) {
	// opposite interface
	if iface.Connection != nil {
		oppIf := iface.Opposite
		for oppnum, val := range oppIf.NameSpace.numbers {
			num := NumberPrefixOppositeInterface + oppnum
			iface.setRelativeNumber(num, val)
		}

		// node of opposite interface
		oppNode := oppIf.Node
		for oppnnum, val := range oppNode.NameSpace.numbers {
			num := NumberPrefixOppositeHeader + NumberPrefixNode + oppnnum
			iface.setRelativeNumber(num, val)
		}

		setGroupNameSpace(iface, oppNode.Groups, true)
	}
}

func setNeighborNameSpace(iface *Interface) {
	for _, neighbors := range iface.Neighbors {
		for _, n := range neighbors {
			// base namespace same as original interface (n.self)
			for k, v := range n.Self.NameSpace.numbers {
				n.setRelativeNumber(k, v)
			}

			// base node namespace
			nodeNumbers := getNodeNameSpace(n.Self)
			for k, v := range nodeNumbers {
				n.setRelativeNumber(k, v)
			}

			// relative namespace (neighbor interface)
			for num, val := range n.Neighbor.NameSpace.numbers {
				name := NumberPrefixNeighbor + num
				n.setRelativeNumber(name, val)
			}

			// relative namespace (neighbor host)
			nodeNumbers = getNodeNameSpace(n.Neighbor)
			for num, val := range nodeNumbers {
				name := NumberPrefixNeighbor + num
				n.setRelativeNumber(name, val)
			}
		}
	}
}

func getNodeNameSpace(iface *Interface) map[string]string {
	nodeNumbers := map[string]string{}
	for nodenum, val := range iface.Node.NameSpace.numbers {
		num := NumberPrefixNode + nodenum
		nodeNumbers[num] = val
	}
	return nodeNumbers
}

func makeRelativeNamespace(nm *NetworkModel) error {
	// Search placelabels for global namespace
	globalNumbers, err := initPlaceLabelNameSpace(nm)
	if err != nil {
		return err
	}

	// generate relative numbers
	for _, node := range nm.Nodes {

		// node self
		for num, val := range node.NameSpace.numbers {
			node.setRelativeNumber(num, val)
		}

		// node groups
		setGroupNameSpace(node, node.Groups, false)

		// PlaceLabels
		setPlaceLabelNameSpace(node, globalNumbers)

		// MetaValueLabels
		err = setMetaValueLabelNameSpace(node, node, globalNumbers)
		if err != nil {
			return err
		}

		for _, iface := range node.Interfaces {

			// interface self
			for num, val := range iface.NameSpace.numbers {
				iface.setRelativeNumber(num, val)
			}

			// parent node of the interface
			nodeNumbers := getNodeNameSpace(iface)
			iface.setRelativeNumbers(nodeNumbers)

			// node group of the interface
			setGroupNameSpace(iface, node.Groups, false)

			// L2 opposite interface
			setL2OppositeNameSpace(iface)

			// L3 neighbor interfaces
			setNeighborNameSpace(iface)

			// PlaceLabels
			setPlaceLabelNameSpace(iface, globalNumbers)

			// MetaValueLabels
			err = setMetaValueLabelNameSpace(iface, iface, globalNumbers)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
