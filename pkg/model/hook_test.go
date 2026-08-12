package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

const hookDot = `graph {
  r1 [class="router"];
  r2 [class="router"];
  r1 -- r2;
}`

// TestModuleAndScenarioShareAHook is the line between a module and a scenario:
// a module adds what it has to do to a hook, the scenario says what it wants
// there, and neither has to know the other's names. What is pinned here is the
// order - a scenario's commands run on ground the module has prepared.
func TestModuleAndScenarioShareAHook(t *testing.T) {
	cfg, nm := buildFullModel(t, `
name: hook_merge
global:
  path: local
module:
  - frr
nodeclass:
  - name: router
    use: [frrLogFile]
    config:
      - name: startup
        template: ["scenario-command"]
      - file: out
        depends: [startup]
        template: ["{{ .self_startup }}"]
file:
  - name: out
`, hookDot)
	_ = nm

	out := generateFor(t, cfg, nm, "r1", "out")
	moduleAt := strings.Index(out, "touch")
	scenarioAt := strings.Index(out, "scenario-command")
	if moduleAt < 0 || scenarioAt < 0 {
		t.Fatalf("both parts belong in the hook, got:\n%s", out)
	}
	if moduleAt > scenarioAt {
		t.Errorf("the module's part comes first, got:\n%s", out)
	}
}

// TestTwoScenarioClassesCannotShareAHook keeps the relaxation narrow: two
// classes of the scenario's own naming one hook is still the accident the
// duplicate check exists for, since nothing says which of them wins.
func TestTwoScenarioClassesCannotShareAHook(t *testing.T) {
	_, err := buildForProvideErr(t, `
name: hook_clash
global:
  path: local
nodeclass:
  - name: router
    use: [extra]
    config:
      - name: startup
        template: ["a"]
  - name: extra
    config:
      - name: startup
        template: ["b"]
`, hookDot)
	if err == nil {
		t.Fatal("two scenario classes naming one hook must be rejected")
	}
	if !strings.Contains(err.Error(), "startup") {
		t.Errorf("the message should name the hook: %v", err)
	}
}

// buildFullModel builds the model the way a real run does. The cheap path used
// elsewhere skips parameter generation, and a class's values are parameters.
func buildFullModel(t *testing.T, yaml, dot string) (*types.Config, *types.NetworkModel) {
	t.Helper()
	cfgPath, dotPath := writeTempInput(t, yaml, dot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg, err = types.LoadTemplates(cfg)
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return cfg, nm
}

// generateFor writes the files into a directory of the test's own and returns
// one node's file.
func generateFor(t *testing.T, cfg *types.Config, nm *types.NetworkModel, node, file string) string {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(wd)

	if err := BuildConfigFiles(cfg, nm, false); err != nil {
		t.Fatalf("BuildConfigFiles: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(node, file))
	if err != nil {
		t.Fatalf("reading the generated file: %v", err)
	}
	return string(content)
}
