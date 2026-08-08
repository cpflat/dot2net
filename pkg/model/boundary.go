package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

// classifyBoundaryConnections attaches a class to the connections that leave a
// group, for every group class that names one in boundary_class.
//
// Which links cross a boundary follows from the topology: a node's groups are
// already known once the skeleton is built. Making the author mark the edges
// instead would record the same fact twice, and the two records can disagree -
// which is what example/vlan_multihost used to do, with hand-written
// normal_conn / vlan_conn labels beside the subgraphs that said the same thing.
//
// It runs before the classes are resolved, because a label added afterwards
// would never become a class, and the point of attaching one is to let the
// scenario hang a connection class off it - a VLAN id policy for the links
// between machines, an eBGP template for the ones between autonomous systems.
func classifyBoundaryConnections(cfg *types.Config, nm *types.NetworkModel) error {
	for _, gc := range cfg.GroupClasses {
		if gc.BoundaryClass == "" {
			continue
		}
		if _, ok := cfg.ConnectionClassByName(gc.BoundaryClass); !ok {
			return fmt.Errorf("groupclass %s: boundary_class %q is not a defined connectionclass",
				gc.Name, gc.BoundaryClass)
		}
		for _, conn := range nm.Connections {
			if conn.Src == nil || conn.Dst == nil {
				continue
			}
			if groupKey(conn.Src.Node, gc.Name) == groupKey(conn.Dst.Node, gc.Name) {
				continue
			}
			// Module tier: the class is derived, so anything the scenario wrote
			// on the edge itself outranks it instead of clashing with it.
			conn.AddModuleClassLabels(gc.BoundaryClass)
		}
	}
	return nil
}

// groupKey identifies which groups of the named class a node belongs to. Two
// endpoints with the same key are on the same side of the boundary; a node in
// no such group has the empty key, so a link from inside a group to a node
// outside every group of that class counts as leaving it.
func groupKey(n *types.Node, className string) string {
	if n == nil {
		return ""
	}
	var names []string
	for _, g := range n.Groups {
		if g.HasClass(className) {
			names = append(names, g.Name)
		}
	}
	sort.Strings(names)
	return strings.Join(names, "\x00")
}
