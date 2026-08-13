package example_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	yaml "github.com/goccy/go-yaml"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

// clabTopo is a partial view of a containerlab topology file, enough to assert
// its structural validity (node kind/image and link endpoints).
type clabTopo struct {
	Name     string `yaml:"name"`
	Topology struct {
		Nodes map[string]struct {
			Kind  string   `yaml:"kind"`
			Image string   `yaml:"image"`
			Binds []string `yaml:"binds"`
		} `yaml:"nodes"`
		Links []struct {
			Endpoints []string `yaml:"endpoints"`
		} `yaml:"links"`
	} `yaml:"topology"`
}

// tinetSpec is a partial view of a tinet spec file.
type tinetSpec struct {
	Nodes []struct {
		Name       string `yaml:"name"`
		Image      string `yaml:"image"`
		Interfaces []struct {
			Name string `yaml:"name"`
			Type string `yaml:"type"`
			Args string `yaml:"args"`
		} `yaml:"interfaces"`
	} `yaml:"nodes"`
}

// modelNodeNames returns the set of node names and the connection count for a
// topology, used to cross-check module output against the model.
func modelNodeNames(t *testing.T, rootDir, topologyName string) (map[string]bool, int) {
	t.Helper()
	topologyDir := findTopologyDir(t, rootDir, topologyName)
	d, err := model.DiagramFromDotFile(filepath.Join(topologyDir, TopologyFileName))
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	cfg, err := types.LoadConfig(filepath.Join(topologyDir, DefinitionFileName))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	nm, err := model.BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("BuildNetworkModel: %v", err)
	}
	names := make(map[string]bool)
	for _, n := range nm.Nodes {
		names[n.Name] = true
	}
	return names, len(nm.Connections)
}

// TestContainerlabModuleOutput parses the generated containerlab topology and
// verifies its structure instead of relying on a byte-for-byte golden
// (CR-078): every model node appears with a non-empty kind and image, and every
// link has exactly two endpoints referencing real nodes.
func TestContainerlabModuleOutput(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	rootDir := filepath.Join(wd, "..", "..")
	const topology = "basic_ospfv2_frr"

	files := buildTopologyFiles(t, rootDir, topology)
	content, ok := files["topo.yaml"]
	if !ok {
		t.Fatalf("topology %s did not generate topo.yaml (got %v)", topology, keysOf(files))
	}

	var topo clabTopo
	if err := yaml.Unmarshal([]byte(content), &topo); err != nil {
		t.Fatalf("topo.yaml is not valid YAML: %v\n%s", err, content)
	}

	if topo.Name == "" {
		t.Errorf("topo.yaml has empty name")
	}

	nodeNames, connCount := modelNodeNames(t, rootDir, topology)
	if len(topo.Topology.Nodes) != len(nodeNames) {
		t.Errorf("topo has %d nodes, model has %d", len(topo.Topology.Nodes), len(nodeNames))
	}
	for name, n := range topo.Topology.Nodes {
		if !nodeNames[name] {
			t.Errorf("topo node %q not in model", name)
		}
		if n.Kind == "" {
			t.Errorf("topo node %q has empty kind", name)
		}
		if n.Image == "" {
			t.Errorf("topo node %q has empty image", name)
		}
	}

	if len(topo.Topology.Links) != connCount {
		t.Errorf("topo has %d links, model has %d connections", len(topo.Topology.Links), connCount)
	}
	for i, l := range topo.Topology.Links {
		if len(l.Endpoints) != 2 {
			t.Errorf("link %d has %d endpoints, want 2: %v", i, len(l.Endpoints), l.Endpoints)
			continue
		}
		for _, ep := range l.Endpoints {
			node, _, ok := strings.Cut(ep, ":")
			if !ok || !nodeNames[node] {
				t.Errorf("link %d endpoint %q does not reference a real node", i, ep)
			}
		}
	}
}

// TestTinetModuleOutput parses the generated tinet spec and verifies each node
// has a name/image and interfaces with name/type/args (CR-078).
func TestTinetModuleOutput(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	rootDir := filepath.Join(wd, "..", "..")
	const topology = "basic_ospfv2_frr"

	files := buildTopologyFiles(t, rootDir, topology)
	content, ok := files["spec.yaml"]
	if !ok {
		t.Fatalf("topology %s did not generate spec.yaml (got %v)", topology, keysOf(files))
	}

	var spec tinetSpec
	if err := yaml.Unmarshal([]byte(content), &spec); err != nil {
		t.Fatalf("spec.yaml is not valid YAML: %v\n%s", err, content)
	}

	nodeNames, _ := modelNodeNames(t, rootDir, topology)
	if len(spec.Nodes) != len(nodeNames) {
		t.Errorf("spec has %d nodes, model has %d", len(spec.Nodes), len(nodeNames))
	}
	for _, n := range spec.Nodes {
		if !nodeNames[n.Name] {
			t.Errorf("spec node %q not in model", n.Name)
		}
		if n.Image == "" {
			t.Errorf("spec node %q has empty image", n.Name)
		}
		for _, iface := range n.Interfaces {
			if iface.Name == "" || iface.Type == "" || iface.Args == "" {
				t.Errorf("spec node %q has malformed interface %+v", n.Name, iface)
			}
			// args are "peerNode#peerIface"; the peer node must be real.
			peer, _, ok := strings.Cut(iface.Args, "#")
			if !ok || !nodeNames[peer] {
				t.Errorf("spec node %q interface %q args %q does not reference a real peer node",
					n.Name, iface.Name, iface.Args)
			}
		}
	}
}

func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
