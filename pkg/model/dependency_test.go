package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// testDepNode is a minimal DependencyNode[string] used to exercise the generic
// DependencyGraph directly, independently of the ConfigTemplate/NameSpacer
// adapters (which are only covered indirectly via format_test.go).
type testDepNode struct {
	id   string
	deps []string
	err  error // if non-nil, GetDependencies returns it
}

func (n testDepNode) GetID() string { return n.id }
func (n testDepNode) GetDependencies() ([]string, error) {
	if n.err != nil {
		return nil, n.err
	}
	return n.deps, nil
}
func (n testDepNode) GetItem() string  { return n.id }
func (n testDepNode) GetLabel() string { return n.id }

func buildGraph(nodes ...testDepNode) *DependencyGraph[string] {
	dg := NewDependencyGraph[string]()
	for _, n := range nodes {
		dg.AddNode(n)
	}
	return dg
}

func indexOf(sorted []string, id string) int {
	for i, s := range sorted {
		if s == id {
			return i
		}
	}
	return -1
}

func TestDependencyGraph_LinearChain(t *testing.T) {
	// a depends on b, b depends on c -> dependencies must precede dependents.
	dg := buildGraph(
		testDepNode{id: "a", deps: []string{"b"}},
		testDepNode{id: "b", deps: []string{"c"}},
		testDepNode{id: "c"},
	)
	sorted, err := dg.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("sorted length = %d, want 3 (%v)", len(sorted), sorted)
	}
	if !(indexOf(sorted, "c") < indexOf(sorted, "b") && indexOf(sorted, "b") < indexOf(sorted, "a")) {
		t.Errorf("expected order c < b < a, got %v", sorted)
	}
}

func TestDependencyGraph_Diamond(t *testing.T) {
	// a -> {b, c} -> d
	dg := buildGraph(
		testDepNode{id: "a", deps: []string{"b", "c"}},
		testDepNode{id: "b", deps: []string{"d"}},
		testDepNode{id: "c", deps: []string{"d"}},
		testDepNode{id: "d"},
	)
	sorted, err := dg.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	da, db, dc, dd := indexOf(sorted, "a"), indexOf(sorted, "b"), indexOf(sorted, "c"), indexOf(sorted, "d")
	if !(dd < db && dd < dc && db < da && dc < da) {
		t.Errorf("expected d before b,c before a, got %v", sorted)
	}
}

func TestDependencyGraph_IndependentStableOrder(t *testing.T) {
	// Nodes with no dependencies are emitted in sorted-ID order (the graph
	// sorts node IDs to make map iteration deterministic).
	dg := buildGraph(
		testDepNode{id: "c"},
		testDepNode{id: "a"},
		testDepNode{id: "b"},
	)
	sorted, err := dg.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a", "b", "c"}
	for i := range want {
		if sorted[i] != want[i] {
			t.Fatalf("independent nodes = %v, want deterministic %v", sorted, want)
		}
	}
}

func TestDependencyGraph_Empty(t *testing.T) {
	dg := NewDependencyGraph[string]()
	sorted, err := dg.TopologicalSort()
	if err != nil {
		t.Fatalf("unexpected error on empty graph: %v", err)
	}
	if len(sorted) != 0 {
		t.Errorf("empty graph sorted = %v, want empty", sorted)
	}
}

func TestDependencyGraph_Cycles(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []testDepNode
		wantCycle string // expected substring of the reported cycle path
	}{
		{
			name:      "self-loop",
			nodes:     []testDepNode{{id: "a", deps: []string{"a"}}},
			wantCycle: "[a a]",
		},
		{
			name: "two-node cycle",
			nodes: []testDepNode{
				{id: "a", deps: []string{"b"}},
				{id: "b", deps: []string{"a"}},
			},
			wantCycle: "[a b a]",
		},
		{
			name: "three-node cycle",
			nodes: []testDepNode{
				{id: "a", deps: []string{"b"}},
				{id: "b", deps: []string{"c"}},
				{id: "c", deps: []string{"a"}},
			},
			wantCycle: "[a b c a]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg := buildGraph(tt.nodes...)
			_, err := dg.TopologicalSort()
			if err == nil {
				t.Fatalf("expected cyclic dependency error, got nil")
			}
			if !strings.Contains(err.Error(), "cyclic dependency detected") {
				t.Errorf("error %q lacks 'cyclic dependency detected'", err.Error())
			}
			if !strings.Contains(err.Error(), tt.wantCycle) {
				t.Errorf("error %q does not contain cycle path %q", err.Error(), tt.wantCycle)
			}
		})
	}
}

func TestDependencyGraph_MissingDependency(t *testing.T) {
	dg := buildGraph(
		testDepNode{id: "a", deps: []string{"ghost"}},
	)
	_, err := dg.TopologicalSort()
	if err == nil {
		t.Fatalf("expected missing-dependency error, got nil")
	}
	if !strings.Contains(err.Error(), "dependency ghost not found for node a") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDependencyGraph_GetDependenciesError(t *testing.T) {
	sentinel := errors.New("boom")
	dg := buildGraph(
		testDepNode{id: "a", err: sentinel},
	)
	_, err := dg.TopologicalSort()
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected GetDependencies error to propagate, got %v", err)
	}
}

// TestReorderConfigTemplates_CycleUsesReadableLabels verifies that a cyclic
// dependency between config templates is reported with human-readable names
// (CR-037) rather than the internal synthetic node IDs (template_0, ...).
func TestReorderConfigTemplates_CycleUsesReadableLabels(t *testing.T) {
	cts := []*types.ConfigTemplate{
		{Name: "alpha", Depends: []string{"beta"}},
		{Name: "beta", Depends: []string{"alpha"}},
	}
	_, err := reorderConfigTemplates(cts)
	if err == nil {
		t.Fatalf("expected cyclic dependency error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "template:alpha") || !strings.Contains(msg, "template:beta") {
		t.Errorf("cycle message %q should name templates alpha/beta", msg)
	}
	if strings.Contains(msg, "template_0") || strings.Contains(msg, "template_1") {
		t.Errorf("cycle message %q leaks internal synthetic IDs", msg)
	}
}

// TestReorderNameSpacers_OrdersDependenciesFirst exercises the NameSpacer
// adapter and, in particular, the pointer-equality match (depNS == originalNS)
// that maps dependency objects back to graph node IDs. A Group depends on its
// member Nodes, so both nodes must precede the group in the result.
func TestReorderNameSpacers_OrdersDependenciesFirst(t *testing.T) {
	nm := types.NewNetworkModel()
	n1 := nm.NewNode("n1")
	n2 := nm.NewNode("n2")
	g := nm.NewGroup("g1")
	g.Nodes = []*types.Node{n1, n2}

	namespacers := []types.NameSpacer{g, n1, n2}
	sorted, err := reorderNameSpacers(namespacers)
	if err != nil {
		t.Fatalf("reorderNameSpacers: %v", err)
	}
	if len(sorted) != 3 {
		t.Fatalf("sorted length = %d, want 3", len(sorted))
	}

	pos := map[types.NameSpacer]int{}
	for i, ns := range sorted {
		pos[ns] = i
	}
	if !(pos[n1] < pos[g] && pos[n2] < pos[g]) {
		t.Errorf("expected member nodes before group; positions n1=%d n2=%d g=%d",
			pos[n1], pos[n2], pos[g])
	}
}
