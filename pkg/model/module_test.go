package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// classifierProbe is a fake module used to pin down when ObjectClassifier runs.
// It records that it was called and adds a class label to every connection.
type classifierProbe struct {
	*types.StandardModule
	called bool
}

var (
	_ types.Module           = (*classifierProbe)(nil)
	_ types.ObjectClassifier = (*classifierProbe)(nil)
)

func (m *classifierProbe) UpdateConfig(cfg *types.Config) error { return nil }

func (m *classifierProbe) ClassifyObjects(cfg *types.Config, nm *types.NetworkModel) error {
	m.called = true
	for _, conn := range nm.Connections {
		conn.AddClassLabels("marked")
	}
	return nil
}

// plainProbe implements only the required Module interface. Dispatch must skip it
// instead of failing.
type plainProbe struct {
	*types.StandardModule
}

var _ types.Module = (*plainProbe)(nil)

func (m *plainProbe) UpdateConfig(cfg *types.Config) error { return nil }

// writeTempInput writes a config/DOT pair and returns their paths.
func writeTempInput(t *testing.T, yaml, dot string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "input.yaml")
	dotPath := filepath.Join(dir, "input.dot")
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := os.WriteFile(dotPath, []byte(dot), 0644); err != nil {
		t.Fatalf("write dot: %v", err)
	}
	return cfgPath, dotPath
}

// TestClassifyObjectsRunsBeforeClassResolution is the reason ObjectClassifier
// exists: a label added by a module must still be turned into a class.
//
// Class labels become class objects once, in checkClasses. A hook placed after
// that point could add labels but they would never resolve, which is exactly the
// trap this ordering avoids. The assertion is therefore on the resolved class, not
// merely on the label string.
func TestClassifyObjectsRunsBeforeClassResolution(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, `
name: classifier_probe
connectionclass:
  - name: marked
    params: [probe_param]
`, `graph { r1 -- r2; }`)

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

	probe := &classifierProbe{StandardModule: types.NewStandardModule()}
	cfg.LoadedModules = []types.Module{probe, &plainProbe{StandardModule: types.NewStandardModule()}}

	if err := classifyModuleObjects(cfg, nm); err != nil {
		t.Fatalf("classifyModuleObjects: %v", err)
	}
	if !probe.called {
		t.Fatal("ObjectClassifier hook was not called")
	}

	if err := checkClasses(cfg, nm); err != nil {
		t.Fatalf("checkClasses: %v", err)
	}

	if len(nm.Connections) == 0 {
		t.Fatal("no connections were built")
	}
	for _, conn := range nm.Connections {
		if !conn.HasClass("marked") {
			t.Errorf("connection %s: label added by the classifier is missing", conn.String())
		}
		resolved := false
		for _, cls := range conn.GetClasses() {
			if cc, ok := cls.(*types.ConnectionClass); ok && cc.Name == "marked" {
				resolved = true
			}
		}
		if !resolved {
			t.Errorf("connection %s: label added by the classifier was not resolved into a class; "+
				"the hook must run before checkClasses", conn.String())
		}
	}
}

// TestModuleGroupClassLabelsAreApplied covers the gap that blocked modules from
// contributing group classes: buildSkeleton used to pass an empty slice to
// Group.SetLabels while doing the opposite for nodes, interfaces and connections.
func TestModuleGroupClassLabelsAreApplied(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, `
name: group_label_probe
groupclass:
  - name: injected
`, `graph { subgraph cluster1 { r1; } r1 -- r2; }`)

	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}

	mod := &plainProbe{StandardModule: types.NewStandardModule()}
	mod.AddModuleGroupClassLabel("injected")
	cfg.LoadedModules = []types.Module{mod}

	nm, err := buildSkeleton(cfg, d)
	if err != nil {
		t.Fatalf("buildSkeleton: %v", err)
	}

	if len(nm.Groups) == 0 {
		t.Fatal("no groups were built")
	}
	for _, g := range nm.Groups {
		if !g.HasClass("injected") {
			t.Errorf("group %s: module-provided group class label was not applied", g.Name)
		}
		if tier := g.ClassTier("injected"); tier != types.ClassTierModule {
			t.Errorf("group %s: module class tier = %d, want %d (modules must be the weakest tier)",
				g.Name, tier, types.ClassTierModule)
		}
	}
}

// moduleTierProbe classifies through AddModuleClassLabels, which is what a
// module has to use so that its classes stay at the weakest tier.
type moduleTierProbe struct {
	*types.StandardModule
	className string
}

var (
	_ types.Module           = (*moduleTierProbe)(nil)
	_ types.ObjectClassifier = (*moduleTierProbe)(nil)
)

func (m *moduleTierProbe) UpdateConfig(cfg *types.Config) error { return nil }

func (m *moduleTierProbe) ClassifyObjects(cfg *types.Config, nm *types.NetworkModel) error {
	for _, conn := range nm.Connections {
		conn.AddModuleClassLabels(m.className)
	}
	return nil
}

// TestModuleClassifierLosesToUserClass checks that a class a module attaches
// through the ObjectClassifier hook is filed at the module tier, so a value the
// user set wins over it. Going through AddClassLabels would record it as
// user-written, turning the same setup into a same-tier conflict instead.
func TestModuleClassifierLosesToUserClass(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, `
name: module_tier_probe
connectionclass:
  - name: user_written
    values:
      mtu: "1500"
  - name: module_default
    values:
      mtu: "9000"
`, `graph { r1 -- r2 [label="user_written"]; }`)

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

	cfg.LoadedModules = []types.Module{
		&moduleTierProbe{StandardModule: types.NewStandardModule(), className: "module_default"},
	}
	if err := classifyModuleObjects(cfg, nm); err != nil {
		t.Fatalf("classifyModuleObjects: %v", err)
	}

	// Both classes set mtu. The module's is weaker, so this must not conflict.
	if err := checkClasses(cfg, nm); err != nil {
		t.Fatalf("the module class should lose to the user class, got: %v", err)
	}

	if err := setGivenParameters(nm); err != nil {
		t.Fatalf("setGivenParameters: %v", err)
	}
	for _, conn := range nm.Connections {
		if got := conn.GetParams()["mtu"]; got != "1500" {
			t.Errorf("connection %s: mtu = %q, want the user-written 1500", conn.String(), got)
		}
	}
}
