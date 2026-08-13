package model

import (
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// buildForAggregate builds the skeleton and runs the pass on it, returning the
// model so a test can count what crosses a machine boundary.
func buildForAggregate(t *testing.T, yaml, dot string) *types.NetworkModel {
	t.Helper()
	cfgPath, dotPath := writeTempInput(t, yaml, dot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := buildSkeleton(cfg, d)
	if err != nil {
		t.Fatalf("buildSkeleton: %v", err)
	}
	if err := aggregateCrossingLinks(cfg, nm); err != nil {
		t.Fatalf("aggregateCrossingLinks: %v", err)
	}
	return nm
}

// crossings counts the links whose two ends stand on different machines. This is
// the number the feature exists to bring down: each one costs a VLAN.
func crossings(t *testing.T, cfg *types.Config, nm *types.NetworkModel) int {
	t.Helper()
	n := 0
	for _, conn := range nm.Connections {
		src := machineOf(cfg, conn.Src.Node)
		dst := machineOf(cfg, conn.Dst.Node)
		if src != dst {
			n++
		}
	}
	return n
}

const aggregateYAML = `
name: aggregate
nodeclass:
  - name: router
  - name: sw
    deploy: platform
groupclass:
  - name: worker
`

// One segment, four members, two machines. Every member on the far side would
// otherwise need a link of its own leaving its machine.
const spanningDOT = `graph {
  sw1 [xlabel="sw"];
  subgraph host1 { xlabel="worker"; r1; r2; }
  subgraph host2 { xlabel="worker"; r3; r4; }
  r1 -- sw1;
  r2 -- sw1;
  r3 -- sw1;
  r4 -- sw1;
}`

func TestAggregateLeavesOneCrossingForOneSegment(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, aggregateYAML, spanningDOT)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := buildSkeleton(cfg, d)
	if err != nil {
		t.Fatalf("buildSkeleton: %v", err)
	}

	// Without the pass the segment stands on no machine at all, so every member
	// reaches it from somewhere else: four members, four crossings.
	if got := crossings(t, cfg, nm); got != 4 {
		t.Fatalf("before aggregating, crossings = %d, want 4", got)
	}

	if err := aggregateCrossingLinks(cfg, nm); err != nil {
		t.Fatalf("aggregateCrossingLinks: %v", err)
	}
	if got := crossings(t, cfg, nm); got != 1 {
		t.Errorf("after aggregating, crossings = %d, want 1", got)
	}

	// Each machine holds one side of the segment, and every member meets the
	// side standing on its own machine.
	for _, name := range []string{"sw1", "sw1_host2"} {
		node, ok := nm.NodeByName(name)
		if !ok {
			t.Fatalf("expected a node named %s", name)
		}
		if machineOf(cfg, node) == nil {
			t.Errorf("%s stands on no machine", name)
		}
	}
}

// TestAggregateScalesWithMembersNotCrossings is the claim the feature rests on:
// the cost of a segment is set by how many machines it reaches, not by how many
// members sit on them.
func TestAggregateScalesWithMembersNotCrossings(t *testing.T) {
	nm := buildForAggregate(t, aggregateYAML, `graph {
  sw1 [xlabel="sw"];
  subgraph host1 { xlabel="worker"; r1; }
  subgraph host2 { xlabel="worker"; r2; r3; r4; r5; r6; }
  r1 -- sw1;
  r2 -- sw1;
  r3 -- sw1;
  r4 -- sw1;
  r5 -- sw1;
  r6 -- sw1;
}`)
	cfgPath, _ := writeTempInput(t, aggregateYAML, "graph {}")
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := crossings(t, cfg, nm); got != 1 {
		t.Errorf("five members on the far machine still cost %d crossings, want 1", got)
	}
}

// TestAggregateLeavesHandSplitSegmentsAlone guards the case that broke the
// examples: a topology that already writes one bridge per machine and joins
// them must come out unchanged. The link between the two sides is not a member,
// and reading it as one would split each side again.
func TestAggregateLeavesHandSplitSegmentsAlone(t *testing.T) {
	nm := buildForAggregate(t, aggregateYAML, `graph {
  subgraph host1 { xlabel="worker"; r1; br1 [xlabel="sw"]; }
  subgraph host2 { xlabel="worker"; r2; br2 [xlabel="sw"]; }
  r1 -- br1;
  r2 -- br2;
  br1 -- br2;
}`)
	if len(nm.Nodes) != 4 {
		names := []string{}
		for _, n := range nm.Nodes {
			names = append(names, n.Name)
		}
		t.Errorf("a hand-split segment was split again: nodes are %v", names)
	}
}

// TestAggregateCanBeTurnedOff pins the setting: the topology keeps the graph
// it drew, crossings and all.
func TestAggregateCanBeTurnedOff(t *testing.T) {
	yaml := "global:\n  aggregate_crossing_links: false\n" + aggregateYAML
	cfgPath, dotPath := writeTempInput(t, yaml, spanningDOT)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.GlobalSettings.AggregateCrossingLinks {
		t.Fatal("the setting was written as false but did not take")
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := buildSkeleton(cfg, d)
	if err != nil {
		t.Fatalf("buildSkeleton: %v", err)
	}
	if err := aggregateCrossingLinks(cfg, nm); err != nil {
		t.Fatalf("aggregateCrossingLinks: %v", err)
	}
	// The topology is left exactly as drawn: one segment belonging to no
	// machine, reached by four links that each leave one.
	if got := crossings(t, cfg, nm); got != 4 {
		t.Errorf("with the setting off, crossings = %d, want the 4 the topology drew", got)
	}
}

// TestAggregateDefaultsToOn pins that writing nothing leaves it on - the case
// Go's zero value would otherwise get wrong.
func TestAggregateDefaultsToOn(t *testing.T) {
	cfgPath, _ := writeTempInput(t, aggregateYAML, "graph {}")
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.GlobalSettings.AggregateCrossingLinks {
		t.Error("a topology that says nothing must get aggregation, but it was off")
	}
}
