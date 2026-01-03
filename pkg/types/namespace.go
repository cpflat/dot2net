package types

import (
	"fmt"
)

// namespace related constants
const NumberSeparator string = "_"
const NumberPrefixNode string = "node" + NumberSeparator
const NumberPrefixConnection string = "conn" + NumberSeparator
const NumberPrefixGroup string = "group" + NumberSeparator
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

// Value reference prefix for template parameters
const ValueReferencePrefix string = "values" + NumberSeparator

// Reserved parameter name for object name
const ReservedParamName string = "name"

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

// ReservedPrefixes returns the list of reserved parameter name prefixes.
// Parameters starting with these prefixes are reserved for internal use.
func ReservedPrefixes() []string {
	return []string{
		NumberPrefixNode,
		NumberPrefixConnection,
		NumberPrefixGroup,
		NumberPrefixOppositeInterface,
		NumberPrefixNeighbor,
		NumberPrefixMember,
		SelfConfigHeader,
		ChildNodesConfigHeader,
		ChildInterfacesConfigHeader,
		ChildConnectionsConfigHeader,
		ChildSegmentsConfigHeader,
		ChildGroupsConfigHeader,
		ChildNeighborsConfigHeader,
		ChildMembersConfigHeader,
		ValueReferencePrefix,
	}
}

// ReservedNames returns the list of reserved parameter names.
// These exact names are reserved for internal use.
func ReservedNames() []string {
	return []string{
		ReservedParamName, // "name"
	}
}

// CheckReservedParamName checks if a parameter name conflicts with reserved names or prefixes.
// Returns an error message if the name is reserved, or empty string if it's safe to use.
func CheckReservedParamName(paramName string) string {
	// Check exact reserved names
	for _, reserved := range ReservedNames() {
		if paramName == reserved {
			return fmt.Sprintf(
				"'%s' is a reserved name (used internally for object names in templates); "+
					"please choose a different name",
				paramName)
		}
	}

	// Check reserved prefixes
	for _, prefix := range ReservedPrefixes() {
		if len(paramName) >= len(prefix) && paramName[:len(prefix)] == prefix {
			purpose := describeReservedPrefix(prefix)
			return fmt.Sprintf(
				"'%s' uses reserved prefix '%s' (%s); "+
					"please rename to avoid this prefix (e.g., '%s' instead)",
				paramName, prefix, purpose, suggestAlternativeName(paramName, prefix))
		}
	}

	return ""
}

// describeReservedPrefix returns a human-readable description of what a reserved prefix is used for.
func describeReservedPrefix(prefix string) string {
	switch prefix {
	case NumberPrefixNode:
		return "for cross-object node references"
	case NumberPrefixConnection:
		return "for cross-object connection references"
	case NumberPrefixGroup:
		return "for cross-object group references"
	case NumberPrefixOppositeInterface:
		return "for opposite interface references"
	case NumberPrefixNeighbor:
		return "for neighbor references"
	case NumberPrefixMember:
		return "for member references"
	case SelfConfigHeader:
		return "for interface config block references"
	case ChildNodesConfigHeader:
		return "for child nodes config references"
	case ChildInterfacesConfigHeader:
		return "for child interfaces config references"
	case ChildConnectionsConfigHeader:
		return "for child connections config references"
	case ChildSegmentsConfigHeader:
		return "for child segments config references"
	case ChildGroupsConfigHeader:
		return "for child groups config references"
	case ChildNeighborsConfigHeader:
		return "for child neighbors config references"
	case ChildMembersConfigHeader:
		return "for child members config references"
	case ValueReferencePrefix:
		return "for Value class references"
	default:
		return "reserved for internal use"
	}
}

// suggestAlternativeName suggests an alternative parameter name that avoids the reserved prefix.
func suggestAlternativeName(paramName, prefix string) string {
	// Remove the prefix and suggest a full word instead
	suffix := paramName[len(prefix):]
	if suffix == "" {
		suffix = "param"
	}

	// Map common abbreviations to full words
	switch prefix {
	case NumberPrefixNode:
		return "node_" + suffix // node_ -> full word "node_"
	case NumberPrefixConnection:
		return "connection_" + suffix // conn_ -> connection_
	case NumberPrefixGroup:
		return "group_" + suffix // group_ -> keep as is, but this shouldn't conflict
	default:
		// For other prefixes, suggest prefixing with "my_" or "custom_"
		return "my_" + paramName
	}
}
