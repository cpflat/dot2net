package model

import (
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// buildForWorker builds the skeleton and runs the placement-unit check on it.
// Nothing here defines a file, so a failure cannot be coming from the output
// paths - which is the point: the invariant used to be enforced only by
// Node.OutputDir, and stayed silent when no node wrote a file.
func buildForWorker(t *testing.T, yaml, dot string) error {
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
	return checkWorkerGroupsDisjoint(cfg, nm)
}

const workerYAML = `
name: worker_check
groupclass:
  - name: worker
  - name: as
`

// TestWorkerGroupsRejectNesting is the silent case: a worker subgraph inside
// another one put its nodes in two topology files at once and made the links
// inside one machine look like they left it.
func TestWorkerGroupsRejectNesting(t *testing.T) {
	err := buildForWorker(t, workerYAML, `graph {
  subgraph host1 {
    xlabel="worker";
    subgraph inner { xlabel="worker"; r1; }
    r2;
  }
  r1 -- r2;
}`)
	if err == nil {
		t.Fatal("a node in a nested worker group must be rejected")
	}
	for _, want := range []string{"r1", "host1", "inner"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error names neither the node nor both groups: %v", err)
			break
		}
	}
}

// TestWorkerGroupsRejectSiblingOverlap covers the same invariant reached the
// other way: a node written into two subgraphs that do not contain each other.
func TestWorkerGroupsRejectSiblingOverlap(t *testing.T) {
	err := buildForWorker(t, workerYAML, `graph {
  subgraph host1 { xlabel="worker"; r1; r2; }
  subgraph host2 { xlabel="worker"; r2; r3; }
  r1 -- r2;
  r2 -- r3;
}`)
	if err == nil {
		t.Fatal("a node in two worker groups must be rejected")
	}
	if !strings.Contains(err.Error(), "r2") {
		t.Errorf("error does not name the shared node: %v", err)
	}
}

// TestWorkerGroupsRejectNestingFromClassPolicy pins that the check sees the
// classes class_policy hands out, not only the ones written in the DOT file:
// with group.default the subgraphs carry no label at all.
func TestWorkerGroupsRejectNestingFromClassPolicy(t *testing.T) {
	err := buildForWorker(t, `
name: worker_check_default
class_policy:
  group:
    default: [worker]
groupclass:
  - name: worker
`, `graph {
  subgraph host1 {
    subgraph inner { r1; }
    r2;
  }
  r1 -- r2;
}`)
	if err == nil {
		t.Fatal("class_policy defaults make these worker groups too, so nesting must be rejected")
	}
}

// TestWorkerGroupsAllowOtherNesting keeps the check narrow. Groups that only
// share parameters are free to nest and to overlap the machines - that is what
// groups are for, and an AS spanning two machines is the ordinary case.
func TestWorkerGroupsAllowOtherNesting(t *testing.T) {
	err := buildForWorker(t, workerYAML, `graph {
  subgraph as1 {
    xlabel="as";
    subgraph host1 { xlabel="worker"; r1; r2; }
    subgraph host2 { xlabel="worker"; r3; }
  }
  r1 -- r2;
  r2 -- r3;
}`)
	if err != nil {
		t.Fatalf("only worker groups have to be disjoint: %v", err)
	}
}

// TestWorkerGroupCheckIsWiredIntoTheBuild guards the call site: the tests above
// run the check themselves and would keep passing if BuildNetworkModel stopped
// calling it.
func TestWorkerGroupCheckIsWiredIntoTheBuild(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, workerYAML, `graph {
  subgraph host1 {
    xlabel="worker";
    subgraph inner { xlabel="worker"; r1; }
    r2;
  }
  r1 -- r2;
}`)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	if _, err := BuildNetworkModel(cfg, d, false); err == nil {
		t.Fatal("BuildNetworkModel accepted a nested worker group; is checkWorkerGroupsDisjoint still called?")
	}
}
