package example_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

// writeInputs writes dot/yaml content into a fresh temp dir and returns their
// paths. The temp dir is cleaned up automatically via t.TempDir.
func writeInputs(t *testing.T, dot, yaml string) (dotPath, yamlPath string) {
	t.Helper()
	dir := t.TempDir()
	dotPath = filepath.Join(dir, "input.dot")
	yamlPath = filepath.Join(dir, "input.yaml")
	if err := os.WriteFile(dotPath, []byte(dot), 0644); err != nil {
		t.Fatalf("write dot: %v", err)
	}
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return dotPath, yamlPath
}

// baseYAML returns a known-good config (ospf_simple) as the starting point for
// failure-injection cases.
func baseYAML(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(findTopologyDir(t, filepath.Join(wd, "..", ".."), "ospf_simple"), "input.yaml"))
	if err != nil {
		t.Fatalf("read base yaml: %v", err)
	}
	return string(data)
}

// TestFailure_InvalidDOT: a malformed topology file must fail at parse time.
func TestFailure_InvalidDOT(t *testing.T) {
	dotPath, _ := writeInputs(t, "this is not a valid dot graph {{{", "name: x")
	if _, err := model.DiagramFromDotFile(dotPath); err == nil {
		t.Fatalf("expected DiagramFromDotFile to reject malformed DOT")
	}
}

// TestFailure_InvalidYAML: a malformed config file must fail at load time.
func TestFailure_InvalidYAML(t *testing.T) {
	_, yamlPath := writeInputs(t, "digraph { r1 -> r2 [dir=none]; }", "name: x\n  : : bad indent :")
	if _, err := types.LoadConfig(yamlPath); err == nil {
		t.Fatalf("expected LoadConfig to reject malformed YAML")
	}
}

// TestFailure_UndefinedClass_Strict: referencing a nodeclass that is not
// defined must fail during model construction in strict mode (default).
func TestFailure_UndefinedClass_Strict(t *testing.T) {
	dot := `digraph {
	r1[xlabel="ghost"];
	r2[xlabel="router"];
	r1->r2[dir="none"];
}`
	dotPath, yamlPath := writeInputs(t, dot, baseYAML(t))

	d, err := model.DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	cfg, err := types.LoadConfig(yamlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	_, err = model.BuildNetworkModel(cfg, d, false)
	if err == nil {
		t.Fatalf("expected BuildNetworkModel to reject undefined class")
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("error %q should name the undefined class 'ghost'", err.Error())
	}
}

// TestFailure_UndefinedClass_Lenient: with ignore_undefined_class: true the
// same topology builds successfully (the class label is skipped). This is the
// complement of the strict case and confirms the CR-006 switch end-to-end.
func TestFailure_UndefinedClass_Lenient(t *testing.T) {
	dot := `digraph {
	r1[xlabel="ghost"];
	r2[xlabel="router"];
	r1->r2[dir="none"];
}`
	yaml := strings.Replace(baseYAML(t), "global:\n", "global:\n  ignore_undefined_class: true\n", 1)
	if !strings.Contains(yaml, "ignore_undefined_class") {
		t.Fatalf("failed to inject ignore_undefined_class into base yaml")
	}
	dotPath, yamlPath := writeInputs(t, dot, yaml)

	d, err := model.DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	cfg, err := types.LoadConfig(yamlPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if _, err := model.BuildNetworkModel(cfg, d, false); err != nil {
		t.Fatalf("lenient mode should build despite undefined class, got: %v", err)
	}
}
