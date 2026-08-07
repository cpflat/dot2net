package model

import (
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// buildForTierOrder runs the model far enough that names and given values are
// assigned, which is where the two order-dependent resolutions used to sit.
func buildForTierOrder(t *testing.T, yaml, dot string) *types.NetworkModel {
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
	if err := checkClasses(cfg, nm); err != nil {
		t.Fatalf("checkClasses: %v", err)
	}
	if err := assignConnectionNames(nm); err != nil {
		t.Fatalf("assignConnectionNames: %v", err)
	}
	if err := setGivenParameters(nm); err != nil {
		t.Fatalf("setGivenParameters: %v", err)
	}
	return nm
}

// TestUsedClassOutranksBaseClassValue is the regression test for the hazard that
// use: introduced: ClassLabels lists the classes reached through use: after
// everything else, so a resolution that walks the slice and keeps the first
// value it sees hands the win to the base class. It did worse than that - it
// reported the two as a same-tier conflict, failing the build outright.
func TestUsedClassOutranksBaseClassValue(t *testing.T) {
	nm := buildForTierOrder(t, `
name: tier_order_values
class_policy:
  node:
    base: [all]
nodeclass:
  - name: all
    values: {mtu: "9000"}
  - name: router
    use: [helper]
  - name: helper
    values: {mtu: "1500"}
`, `graph { r1 [class="router"]; r2; r1 -- r2; }`)

	for _, n := range nm.Nodes {
		if n.Name != "r1" {
			continue
		}
		if got := n.GetParams()["mtu"]; got != "1500" {
			t.Errorf("node r1: mtu = %q, want 1500 from the used user class (labels: %v)",
				got, n.ClassLabels())
		}
	}
}

// TestUsedClassOutranksBaseClassPrefix covers the same hazard in the one place
// that resolved a tiered attribute outside tieredValues: connection names were
// built from the first non-empty prefix in GetClasses() rather than from the
// prefix SetClasses had already resolved, so the two could disagree.
func TestUsedClassOutranksBaseClassPrefix(t *testing.T) {
	nm := buildForTierOrder(t, `
name: tier_order_prefix
class_policy:
  connection:
    base: [all]
connectionclass:
  - name: all
    prefix: weak
  - name: named
    use: [helper]
  - name: helper
    prefix: strong
`, `graph { r1 -- r2 [label="named"]; }`)

	if len(nm.Connections) != 1 {
		t.Fatalf("got %d connections, want 1", len(nm.Connections))
	}
	conn := nm.Connections[0]
	if conn.NamePrefix != "strong" {
		t.Errorf("prefix = %q, want strong from the used user class", conn.NamePrefix)
	}
	if conn.Name != "strong0" {
		t.Errorf("name = %q, want strong0 (the assigned name must follow the resolved prefix)", conn.Name)
	}
}
