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

// TestModuleAndTopologyShareAHook is the line between a module and a topology:
// a module writes what it has to do into the hook's group, the topology writes
// what it wants there, and neither has to know the other's names. What is
// pinned here is the order - a topology's commands run on ground the module has
// prepared - and that it takes a priority on the module's own block to say so,
// not a rule about hooks anywhere in the core.
func TestModuleAndTopologyShareAHook(t *testing.T) {
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
      - group: startup
        template: ["topology-command"]
      - file: out
        style: sort
        sort_group: startup
file:
  - name: out
`, hookDot)

	out := generateFor(t, cfg, nm, "r1", "out")
	moduleAt := strings.Index(out, "touch")
	topologyAt := strings.Index(out, "topology-command")
	if moduleAt < 0 || topologyAt < 0 {
		t.Fatalf("both parts belong in the hook, got:\n%s", out)
	}
	if moduleAt > topologyAt {
		t.Errorf("the module's part comes first, got:\n%s", out)
	}
}

// TestTwoTopologyClassesMayShareAHook is what the rework was for. Naming one
// hook from two classes used to be reported as two classes claiming one name,
// though wanting to add commands from both is ordinary - a class per role, each
// with something to run. A group is written into by whoever has something to
// say.
func TestTwoTopologyClassesMayShareAHook(t *testing.T) {
	cfg, nm := buildFullModel(t, `
name: hook_share
global:
  path: local
nodeclass:
  - name: router
    use: [extra]
    config:
      - group: startup
        template: ["from-router"]
      - file: out
        style: sort
        sort_group: startup
  - name: extra
    config:
      - group: startup
        template: ["from-extra"]
`+"file:\n  - name: out\n", hookDot)

	out := generateFor(t, cfg, nm, "r1", "out")
	for _, want := range []string{"from-router", "from-extra"} {
		if !strings.Contains(out, want) {
			t.Errorf("%s belongs in the hook, got:\n%s", want, out)
		}
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
