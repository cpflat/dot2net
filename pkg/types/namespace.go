package types

import (
	"fmt"
)

// namespace related constants
const NumberSeparator string = "_"
const NumberPrefixNode string = "node" + NumberSeparator
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

func checkPlaceLabelOwner(ns NameSpacer, o LabelOwner,
	globalParams map[string]map[string]string) (map[string]map[string]string, error) {
	for _, plabel := range o.PlaceLabels() {
		if _, exists := globalParams[plabel]; exists {
			return nil, fmt.Errorf("duplicated PlaceLabels %+v", plabel)
		}
		globalParams[plabel] = map[string]string{}

		for k, v := range ns.GetParams() {
			globalParams[plabel][k] = v
		}
	}
	return globalParams, nil
}

func InitGloballNameSpace(nm *NetworkModel) (map[string]map[string]string, error) {
	// Search placelabels for global namespace
	globalParams := map[string]map[string]string{} // globalNumbers[PlaceLabel][NumberKey] = NumberValue
	for _, node := range nm.Nodes {
		globalParams, err := checkPlaceLabelOwner(node, node, globalParams)
		if err != nil {
			return nil, err
		}

		for _, iface := range node.Interfaces {
			checkPlaceLabelOwner(iface, iface, globalParams)
		}
	}
	for _, group := range nm.Groups {
		checkPlaceLabelOwner(group, group, globalParams)
	}

	// Set numbers for place labels from global namespace
	for _, node := range nm.Nodes {
		for plabel, nums := range globalParams {
			for k, v := range nums {
				name := plabel + NumberSeparator + k
				node.SetRelativeParam(name, v)
			}
		}
		for _, iface := range node.Interfaces {
			for plabel, nums := range globalParams {
				for k, v := range nums {
					name := plabel + NumberSeparator + k
					iface.SetRelativeParam(name, v)
				}
			}
		}
	}

	return globalParams, nil
}

func setGlobalParams(ns NameSpacer, globalParams map[string]map[string]string) {
	for header, nums := range globalParams {
		for k, v := range nums {
			name := header + NumberSeparator + k
			ns.SetRelativeParam(name, v)
		}
	}
}

// set meta value label parameters for labelowners
func setMetaValueLabelNameSpace(ns NameSpacer, o LabelOwner,
	globaladdressOwner map[string]map[string]string, header string) error {

	// if MetaValueLabel is given, a PlaceLabel namespace can be referrable with the MetaValueLabel
	for mvlabel, target := range o.MetaValueLabels() {
		nums, ok := globaladdressOwner[target]
		if !ok {
			return fmt.Errorf("unknown PlaceLabel %v (specified for MetaValueLabel %v)", target, mvlabel)
		}
		for k, v := range nums {
			name := header + mvlabel + NumberSeparator + k
			ns.SetRelativeParam(name, v)
		}
	}
	return nil
}
