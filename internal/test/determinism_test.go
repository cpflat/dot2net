package example_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

// determinismScenarios are representative example scenarios exercising the
// map-iteration-heavy paths (grouping, IP assignment, parameter distribution,
// multi-layer config) most likely to expose non-deterministic output.
var determinismScenarios = []string{
	"vlan_multihost",
	"basic_clos",
	"bgp_evpn_vxlan_topo1",
	"basic_bgp",
	"param_share",
}

// TestBuildDeterminism verifies that building the same scenario twice from a
// clean state produces byte-identical output (CR-079). Go randomizes map
// iteration order on every range statement, so two independent in-process
// builds already exercise different iteration orders; running the suite with
// `go test -count=2` additionally covers cross-process variation.
//
// Unlike TestExampleScenarios (which pins output against committed golden
// files), this test compares two fresh runs against *each other*, so it
// detects determinism regressions independently of whether the golden files
// happen to be up to date.
func TestBuildDeterminism(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	rootDir := filepath.Join(wd, "..", "..") // project root

	for _, scenarioName := range determinismScenarios {
		scenarioName := scenarioName
		t.Run(scenarioName, func(t *testing.T) {
			// Skip gracefully if a scenario is renamed/removed.
			dotFile := filepath.Join(rootDir, "example", scenarioName, TopologyFileName)
			if _, err := os.Stat(dotFile); err != nil {
				t.Skipf("scenario %s not present: %v", scenarioName, err)
			}

			runA := buildScenarioFiles(t, rootDir, scenarioName)
			runB := buildScenarioFiles(t, rootDir, scenarioName)

			if len(runA) == 0 {
				t.Fatalf("scenario %s generated no files", scenarioName)
			}
			if diff := cmp.Diff(runA, runB); diff != "" {
				t.Errorf("non-deterministic output for %s (-runA +runB):\n%s", scenarioName, diff)
			}
		})
	}
}

// buildScenarioFiles runs the full DiagramFromDotFile -> LoadConfig ->
// BuildNetworkModel -> BuildConfigFiles pipeline for a scenario in an isolated
// temporary directory and returns a map of generated file path -> normalized
// content. Each invocation reloads config and model from scratch so that
// non-determinism in model construction (IP assignment, parameter
// distribution) is exercised in addition to file formatting.
func buildScenarioFiles(t *testing.T, rootDir, scenarioName string) map[string]string {
	t.Helper()

	scenarioDir := filepath.Join(rootDir, "example", scenarioName)

	tmpDir, err := os.MkdirTemp("", "dot2net_determinism")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// copy input files (top-level only) into tmp dir
	err = filepath.WalkDir(scenarioDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != scenarioDir {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			relPath, err := filepath.Rel(scenarioDir, path)
			if err != nil {
				return err
			}
			copyFile(t, path, filepath.Join(tmpDir, relPath))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to copy input files: %v", err)
	}

	topoFile := filepath.Join(tmpDir, TopologyFileName)
	defFile := filepath.Join(tmpDir, DefinitionFileName)

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer os.Chdir(oldWd)

	d, err := model.DiagramFromDotFile(topoFile)
	if err != nil {
		t.Fatalf("DiagramFromDotFile failed: %v", err)
	}
	cfg, err := types.LoadConfig(defFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	nm, err := model.BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("BuildNetworkModel failed: %v", err)
	}

	// Remove any pre-existing outputs so only freshly generated files remain.
	expectedFiles, err := model.ListGeneratedFiles(cfg, nm, false)
	if err != nil {
		t.Fatalf("ListGeneratedFiles failed: %v", err)
	}
	for _, file := range expectedFiles {
		os.Remove(filepath.Join(tmpDir, file))
	}

	if err := model.BuildConfigFiles(cfg, nm, false); err != nil {
		t.Fatalf("BuildConfigFiles failed: %v", err)
	}

	// Read generated files into memory (before deferred cleanup runs).
	result := make(map[string]string)
	for _, rel := range collectGeneratedFiles(tmpDir) {
		if rel == TopologyFileName || rel == DefinitionFileName {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tmpDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("failed to read generated file %s: %v", rel, err)
		}
		result[rel] = normalizePaths(string(data), tmpDir)
	}
	return result
}
