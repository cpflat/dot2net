package model

import (
	"fmt"
	"sort"

	"github.com/cpflat/dot2net/pkg/types"
)

// aggregateCrossingLinks cuts the number of links that leave a machine, by
// replacing a shared segment that reaches across machines with one bridge per
// machine and a link between them.
//
// A shared segment is drawn as a node the platform provides rather than deploys
// - one bridge, one collision domain, one switch, depending on who realizes it.
// Such a node stands on one machine. Left alone, every member on another
// machine needs a link that leaves its own, so a segment with n members over
// there costs n crossings. Give each machine its own bridge and join the
// bridges, and the same segment costs one crossing per pair of machines
// whatever n is. That is the point: a crossing costs a VLAN from a finite pool
// (doc/active/STARBED_REQUIREMENTS.md REQ-7).
//
// # Meant to move out of core
//
// This is a model transformation, not something the rest of the build depends
// on: nothing downstream asks whether it ran. It reads only vocabulary the core
// owns and every platform shares - which groups are machines (worker) and which
// nodes the platform provides (deploy) - and it changes the model only through
// the same public API a module has (NewNode, AdoptInterface, NewConnection,
// CloneLabels). Its signature is the one an object-classifier hook already uses,
// so moving it to a module means moving the file and registering the hook.
//
// Keep it that way. Reaching into anything unexported here, or letting a later
// stage read a flag this pass leaves behind, would tie it to core for good.
func aggregateCrossingLinks(cfg *types.Config, nm *types.NetworkModel) error {
	if !cfg.GlobalSettings.AggregateCrossingLinks {
		return nil
	}
	// With no placement units declared there is one machine, and nothing can
	// cross out of it.
	if _, ok := cfg.GroupClassByName(types.WorkerGroupClassName); !ok {
		return nil
	}

	// Snapshot: the loop adds nodes, and a bridge it adds is never itself a
	// candidate.
	candidates := make([]*types.Node, 0, len(nm.Nodes))
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			candidates = append(candidates, node)
		}
	}

	for _, node := range candidates {
		if err := aggregateAtSharedSegment(cfg, nm, node); err != nil {
			return err
		}
	}
	return nil
}

// aggregateAtSharedSegment does the replacement for one shared segment.
func aggregateAtSharedSegment(cfg *types.Config, nm *types.NetworkModel, seg *types.Node) error {
	// Which machine each member sits on. A segment whose members share one
	// machine crosses nothing and is left alone - that is every single-machine
	// topology, and every segment inside one machine of a multi-machine one.
	facing := map[string][]*types.Interface{}
	for _, iface := range seg.Interfaces {
		if iface.Opposite == nil {
			continue
		}
		// A link to another shared segment is not a member: it is the link
		// between two sides of a segment that is already split, whether this
		// pass made it or the author wrote it out by hand. Counting it as a
		// member would make each side look like it reached the other's machine,
		// and every hand-written pair would be split again.
		if cfg.IsSwitchNode(iface.Opposite.Node) {
			continue
		}
		machine := machineOf(cfg, iface.Opposite.Node)
		if machine == nil {
			return fmt.Errorf(
				"node %s is connected to %s but belongs to no %s group, so there is no machine "+
					"to put its side of the segment on",
				iface.Opposite.Node.Name, seg.Name, types.WorkerGroupClassName)
		}
		facing[machine.Name] = append(facing[machine.Name], iface)
	}
	if len(facing) < 2 {
		return nil
	}

	machines := make([]string, 0, len(facing))
	for name := range facing {
		machines = append(machines, name)
	}
	sort.Strings(machines)

	// Where the node the author drew stays. Honouring the group they put it in
	// keeps the output recognisable; with none written, the first machine in
	// name order keeps it, so the result does not depend on map iteration.
	home := machines[0]
	if machine := machineOf(cfg, seg); machine != nil {
		home = machine.Name
	} else {
		homeGroup, ok := nm.GroupByName(home)
		if !ok {
			return fmt.Errorf("aggregateCrossingLinks panic: no group named %s", home)
		}
		placeOn(seg, homeGroup)
	}

	// One bridge per other machine, carrying the classes of the node the author
	// drew: it is the same segment, reached from somewhere else.
	bridges := map[string]*types.Node{home: seg}
	for _, machine := range machines {
		if machine == home {
			continue
		}
		group, ok := nm.GroupByName(machine)
		if !ok {
			return fmt.Errorf("aggregateCrossingLinks panic: no group named %s", machine)
		}
		name := seg.Name + "_" + machine
		if _, taken := nm.NodeByName(name); taken {
			return fmt.Errorf(
				"cannot name the %s side of segment %s: %s is already a node",
				machine, seg.Name, name)
		}
		bridge := nm.NewNode(name)
		// On its own machine it is the segment, and nothing else there is. The
		// suffix is for the model, which holds every machine at once.
		bridge.LocalName = seg.Name
		bridge.ParsedLabels = seg.ParsedLabels.CloneLabels()
		placeOn(bridge, group)
		bridges[machine] = bridge

		// Each member keeps facing the segment; it now meets it on its own
		// machine, so that link no longer crosses.
		for _, iface := range facing[machine] {
			bridge.AdoptInterface(iface)
		}
	}

	// Join the bridges in a chain, in name order: k-1 links for k machines, and
	// no loop. A star costs the same and would also do; the shape between them
	// is not something the topology states, so the simplest deterministic one
	// is used until a reason to choose appears.
	for i := 1; i < len(machines); i++ {
		if err := linkBridges(cfg, nm, bridges[machines[i-1]], bridges[machines[i]]); err != nil {
			return err
		}
	}
	return nil
}

// linkBridges wires two bridges of one segment together. The link is given
// whatever the topology gives a link it does not label, because that is what it
// is: the link the author would otherwise have drawn themselves.
func linkBridges(cfg *types.Config, nm *types.NetworkModel, a, b *types.Node) error {
	aIface := a.NewInterface("")
	if err := aIface.SetLabels(cfg, nil, getModuleInterfaceClassLabels(cfg)); err != nil {
		return err
	}
	bIface := b.NewInterface("")
	if err := bIface.SetLabels(cfg, nil, getModuleInterfaceClassLabels(cfg)); err != nil {
		return err
	}
	aIface.Opposite = bIface
	bIface.Opposite = aIface

	conn := nm.NewConnection(aIface, bIface)
	return conn.SetLabels(cfg, nil, getModuleConnectionClassLabels(cfg))
}

// machineOf returns the machine a node stands on, or nil. A node stands on at
// most one: checkWorkerGroupsDisjoint has already refused anything else.
func machineOf(cfg *types.Config, node *types.Node) *types.Group {
	if node == nil {
		return nil
	}
	for _, group := range node.Groups {
		if cfg.IsWorkerGroup(group) {
			return group
		}
	}
	return nil
}

func placeOn(node *types.Node, group *types.Group) {
	node.Groups = append(node.Groups, group)
	group.Nodes = append(group.Nodes, node)
}
