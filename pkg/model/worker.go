package model

import (
	"fmt"
	"sort"

	"github.com/cpflat/dot2net/pkg/types"
)

// checkWorkerGroupsDisjoint rejects a node that sits in more than one worker
// group. A worker group stands for one placement unit, and a node is deployed
// to exactly one of them.
//
// Nested or overlapping worker subgraphs are easy to write, and until this
// check they produced contradictory output without a word: the node was listed
// in the topology file of both machines, and classifyBoundaryConnections marked
// the links inside one machine as leaving it, because the two endpoints then
// belong to different sets of worker groups. Node.OutputDir carries the same
// invariant, but it only speaks when the output is split by group and the node
// writes a file of its own, so the mistake escaped whenever either was absent.
//
// It runs on the skeleton, where the group class labels are already resolved -
// class_policy defaults included, since GetValidGroupClasses applies them while
// the labels are set.
func checkWorkerGroupsDisjoint(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		var names []string
		for _, group := range node.Groups {
			if cfg.IsWorkerGroup(group) {
				names = append(names, group.Name)
			}
		}
		if len(names) > 1 {
			sort.Strings(names)
			return fmt.Errorf(
				"node %s belongs to more than one %s group (%s and %s), but a node is deployed to one placement unit",
				node.Name, types.WorkerGroupClassName, names[0], names[1],
			)
		}
	}
	return nil
}
