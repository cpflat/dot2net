package model

import (
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// buildForBoundary builds far enough that class labels have been resolved.
func buildForBoundary(t *testing.T, yaml, dot string) (*types.Config, *types.NetworkModel, error) {
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
	if err := classifyBoundaryConnections(cfg, nm); err != nil {
		return cfg, nm, err
	}
	return cfg, nm, checkClasses(cfg, nm)
}

// connByNodes finds the connection between two named nodes.
func connByNodes(nm *types.NetworkModel, a, b string) *types.Connection {
	for _, conn := range nm.Connections {
		s, d := conn.Src.Node.Name, conn.Dst.Node.Name
		if (s == a && d == b) || (s == b && d == a) {
			return conn
		}
	}
	return nil
}

const boundaryYAML = `
name: boundary
groupclass:
  - name: worker
    boundary_crossing_connection_class: leaves_host
connectionclass:
  - name: leaves_host
    values: {crossing: "yes"}
`

// The two machines each hold two routers. r2 and r3 meet on a bridge that
// belongs to no machine, which is how the multi-host topologies are drawn.
const boundaryDOT = `graph {
  subgraph host1 { xlabel="worker"; r1; r2; }
  subgraph host2 { xlabel="worker"; r3; r4; }
  sw;
  r1 -- r2;
  r3 -- r4;
  r2 -- sw;
  r3 -- sw;
}`

// TestBoundaryClassMarksConnectionsLeavingAGroup is the feature TODO 28 needs:
// which links leave a machine follows from the subgraphs, so the author does
// not annotate the edges. A link between two members of one machine must stay
// unmarked, and one that reaches outside must be marked.
func TestBoundaryClassMarksConnectionsLeavingAGroup(t *testing.T) {
	_, nm, err := buildForBoundary(t, boundaryYAML, boundaryDOT)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	for _, tc := range []struct {
		a, b    string
		crosses bool
		why     string
	}{
		{"r1", "r2", false, "both ends are in host1"},
		{"r3", "r4", false, "both ends are in host2"},
		{"r2", "sw", true, "sw belongs to no machine, so the link leaves host1"},
		{"r3", "sw", true, "sw belongs to no machine, so the link leaves host2"},
	} {
		conn := connByNodes(nm, tc.a, tc.b)
		if conn == nil {
			t.Fatalf("connection %s--%s was not built", tc.a, tc.b)
		}
		if got := conn.HasClass("leaves_host"); got != tc.crosses {
			t.Errorf("%s--%s: marked=%v, want %v (%s)", tc.a, tc.b, got, tc.crosses, tc.why)
		}
	}
}

// TestBoundaryClassIsWeakerThanAWrittenClass keeps the derived label from
// clashing with the topology: the label is attached at the module tier, so a
// class the author put on the edge decides any value they both set.
func TestBoundaryClassIsWeakerThanAWrittenClass(t *testing.T) {
	_, nm, err := buildForBoundary(t, `
name: boundary_tier
groupclass:
  - name: worker
    boundary_crossing_connection_class: leaves_host
connectionclass:
  - name: leaves_host
    values: {mtu: "9000"}
  - name: written
    values: {mtu: "1500"}
`, `graph {
  subgraph host1 { xlabel="worker"; r1; }
  subgraph host2 { xlabel="worker"; r2; }
  r1 -- r2 [label="written"];
}`)
	if err != nil {
		t.Fatalf("the derived class must lose to the written one, got: %v", err)
	}
	conn := connByNodes(nm, "r1", "r2")
	if !conn.HasClass("leaves_host") {
		t.Fatal("the derived class was not attached")
	}
	if tier := conn.ClassTier("leaves_host"); tier != types.ClassTierModule {
		t.Errorf("derived class tier = %d, want %d (weakest)", tier, types.ClassTierModule)
	}
	if err := setGivenParameters(nm); err != nil {
		t.Fatalf("setGivenParameters: %v", err)
	}
	if got := conn.GetParams()["mtu"]; got != "1500" {
		t.Errorf("mtu = %q, want the written 1500", got)
	}
}

// TestBoundaryClassWorksForAnyGroupClass pins that the feature carries no host
// meaning: the same field on an "as" class marks the sessions between
// autonomous systems.
func TestBoundaryClassWorksForAnyGroupClass(t *testing.T) {
	_, nm, err := buildForBoundary(t, `
name: boundary_as
groupclass:
  - name: as
    boundary_crossing_connection_class: ebgp
connectionclass:
  - name: ebgp
`, `graph {
  subgraph as1 { xlabel="as"; r1; r2; }
  subgraph as2 { xlabel="as"; r3; }
  r1 -- r2;
  r2 -- r3;
}`)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if connByNodes(nm, "r1", "r2").HasClass("ebgp") {
		t.Error("r1--r2 is inside one AS and must not be marked")
	}
	if !connByNodes(nm, "r2", "r3").HasClass("ebgp") {
		t.Error("r2--r3 crosses an AS boundary and must be marked")
	}
}

// TestBoundaryClassIsWiredIntoTheBuild guards the wiring rather than the
// function: the other tests here call classifyBoundaryConnections themselves,
// so they would all keep passing if the call were dropped from
// BuildNetworkModel. This one goes through the real pipeline.
func TestBoundaryClassIsWiredIntoTheBuild(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, boundaryYAML, boundaryDOT)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("BuildNetworkModel: %v", err)
	}
	if !connByNodes(nm, "r2", "sw").HasClass("leaves_host") {
		t.Error("r2--sw leaves host1 but was not marked; is classifyBoundaryConnections still called?")
	}
	if connByNodes(nm, "r1", "r2").HasClass("leaves_host") {
		t.Error("r1--r2 stays inside host1 and must not be marked")
	}
}

// TestBoundaryClassRejectsUndefinedClass catches the typo at the point where
// the name is written, rather than letting the label silently resolve to
// nothing.
func TestBoundaryClassRejectsUndefinedClass(t *testing.T) {
	_, _, err := buildForBoundary(t, `
name: boundary_typo
groupclass:
  - name: worker
    boundary_crossing_connection_class: no_such_class
`, `graph { subgraph host1 { xlabel="worker"; r1; } r1 -- r2; }`)
	if err == nil {
		t.Fatal("an undefined boundary_crossing_connection_class must be rejected")
	}
	if !strings.Contains(err.Error(), "no_such_class") {
		t.Errorf("the error should quote the offending name, got: %v", err)
	}
}
