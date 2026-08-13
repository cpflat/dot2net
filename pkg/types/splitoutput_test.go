package types

import (
	"testing"
)

// TestSplitModuleOutputDefaultsToOff pins that a topology saying nothing keeps
// the flat layout it had.
func TestSplitModuleOutputDefaultsToOff(t *testing.T) {
	cfg, err := loadConfigFrom(t, "name: s\n", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.GlobalSettings.SplitModuleOutput {
		t.Error("splitting must be something a topology asks for")
	}
}

func TestSplitModuleOutputIsRead(t *testing.T) {
	cfg, err := loadConfigFrom(t, "name: s\nglobal:\n  split_module_output: true\n", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.GlobalSettings.SplitModuleOutput {
		t.Error("the setting was written but did not take")
	}
}

// TestSubdirPlacesTheFile checks the mechanism the split rides on: a file
// definition that names a subdirectory lands there rather than at the root.
func TestSubdirPlacesTheFile(t *testing.T) {
	fd := &FileDefinition{Name: "topo.yaml", Subdir: "containerlab"}
	if fd.Subdir != "containerlab" {
		t.Fatalf("Subdir = %q", fd.Subdir)
	}
	// The name itself is untouched: only where it goes changes.
	if got := fd.GetFileName(""); got != "topo.yaml" {
		t.Errorf("GetFileName = %q, want topo.yaml", got)
	}
}

// TestGenerateScriptsDefaultsToOff: the scripts are opt-in, so no topology
// gains files it did not ask for.
func TestGenerateScriptsDefaultsToOff(t *testing.T) {
	cfg, err := loadConfigFrom(t, "name: s\nmodule:\n  - containerlab\n", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var opts struct {
		GenerateScripts bool `yaml:"generate_scripts"`
	}
	found, err := cfg.DecodeModuleConfig("containerlab", &opts)
	if err != nil {
		t.Fatalf("DecodeModuleConfig: %v", err)
	}
	if found || opts.GenerateScripts {
		t.Error("a topology that says nothing must get no scripts")
	}
}

func TestGenerateScriptsIsPerModule(t *testing.T) {
	cfg, err := loadConfigFrom(t, `
name: s
module:
  - containerlab
  - tinet
module_config:
  containerlab:
    generate_scripts: true
`, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var clab, tinet struct {
		GenerateScripts bool `yaml:"generate_scripts"`
	}
	if _, err := cfg.DecodeModuleConfig("containerlab", &clab); err != nil {
		t.Fatalf("containerlab: %v", err)
	}
	if _, err := cfg.DecodeModuleConfig("tinet", &tinet); err != nil {
		t.Fatalf("tinet: %v", err)
	}
	if !clab.GenerateScripts {
		t.Error("containerlab asked for scripts and did not get them")
	}
	if tinet.GenerateScripts {
		t.Error("tinet did not ask for scripts but got them: the setting is per module")
	}
}

// TestSplitAndScriptsAreIndependent pins that the two are separate choices.
// They read well together, but a topology may want either alone.
func TestSplitAndScriptsAreIndependent(t *testing.T) {
	cfg, err := loadConfigFrom(t, `
name: s
global:
  split_module_output: true
module:
  - containerlab
`, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.GlobalSettings.SplitModuleOutput {
		t.Fatal("splitting did not take")
	}
	var opts struct {
		GenerateScripts bool `yaml:"generate_scripts"`
	}
	if _, err := cfg.DecodeModuleConfig("containerlab", &opts); err != nil {
		t.Fatalf("DecodeModuleConfig: %v", err)
	}
	if opts.GenerateScripts {
		t.Error("splitting the output must not switch the scripts on as well")
	}
}
