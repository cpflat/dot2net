package example_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dotlike"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
	"github.com/cpflat/dot2net/pkg/visual"
)

// buildModelForVisual builds a NetworkModel for a scenario without writing any
// output files (so it can run without changing the working directory).
func buildModelForVisual(t *testing.T, scenarioName string) (*types.Config, *types.NetworkModel) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	scenarioDir := filepath.Join(wd, "..", "..", "example", scenarioName)

	d, err := model.DiagramFromDotFile(filepath.Join(scenarioDir, TopologyFileName))
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	cfg, err := types.LoadConfig(filepath.Join(scenarioDir, DefinitionFileName))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	nm, err := model.BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("BuildNetworkModel: %v", err)
	}
	return cfg, nm
}

// TestGraphToDot verifies that visual.GraphToDot produces a parseable DOT graph
// that includes the scenario's nodes, and that an unknown layer is rejected.
func TestGraphToDot(t *testing.T) {
	cfg, nm := buildModelForVisual(t, "ospf_simple")

	out, err := visual.GraphToDot(cfg, nm, "")
	if err != nil {
		t.Fatalf("GraphToDot: %v", err)
	}
	if !strings.Contains(out, "digraph") {
		t.Errorf("output is not a digraph:\n%s", out)
	}

	// The emitted DOT must be syntactically valid (re-parseable).
	if _, err := dotlike.Parse([]byte(out)); err != nil {
		t.Errorf("GraphToDot produced unparseable DOT: %v\n%s", err, out)
	}

	// Every model node name must appear in the DOT output.
	for _, node := range nm.Nodes {
		if !strings.Contains(out, node.Name) {
			t.Errorf("DOT output missing node %q", node.Name)
		}
	}

	if _, err := visual.GraphToDot(cfg, nm, "no_such_layer"); err == nil {
		t.Errorf("expected error for unknown layer")
	} else if !strings.Contains(err.Error(), "unknown layer") {
		t.Errorf("unexpected error for unknown layer: %v", err)
	}
}

// TestGetDataJSON verifies that visual.GetDataJSON emits valid JSON whose node
// and connection sets match the model.
func TestGetDataJSON(t *testing.T) {
	cfg, nm := buildModelForVisual(t, "ospf_simple")

	raw, err := visual.GetDataJSON(cfg, nm)
	if err != nil {
		t.Fatalf("GetDataJSON: %v", err)
	}

	var data visual.NetworkModelData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("GetDataJSON produced invalid JSON: %v\n%s", err, raw)
	}

	if len(data.Nodes) != len(nm.Nodes) {
		t.Errorf("JSON has %d nodes, model has %d", len(data.Nodes), len(nm.Nodes))
	}
	if len(data.Connections) != len(nm.Connections) {
		t.Errorf("JSON has %d connections, model has %d", len(data.Connections), len(nm.Connections))
	}

	modelNodeNames := make(map[string]bool)
	for _, n := range nm.Nodes {
		modelNodeNames[n.Name] = true
	}
	for _, n := range data.Nodes {
		if !modelNodeNames[n.Name] {
			t.Errorf("JSON node %q not present in model", n.Name)
		}
	}

	// Each connection must reference real node names on both ends.
	for _, c := range data.Connections {
		if !modelNodeNames[c.SrcNode] || !modelNodeNames[c.DstNode] {
			t.Errorf("connection references unknown node(s): %s -> %s", c.SrcNode, c.DstNode)
		}
	}
}
