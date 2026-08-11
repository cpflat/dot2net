package types

import (
	"strings"
	"testing"
)

type probeOpts struct {
	ManagementNetwork bool   `yaml:"management_network"`
	Name              string `yaml:"name"`
}

func TestDecodeModuleConfigReadsTheSectionForOneModule(t *testing.T) {
	cfg, err := loadConfigFrom(t, `
name: mc
module:
  - containerlab
  - tinet
module_config:
  containerlab:
    management_network: true
    name: from_clab
  tinet:
    name: from_tinet
`, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var opts probeOpts
	found, err := cfg.DecodeModuleConfig("containerlab", &opts)
	if err != nil {
		t.Fatalf("DecodeModuleConfig: %v", err)
	}
	if !found {
		t.Fatal("the section is written but was not found")
	}
	if !opts.ManagementNetwork || opts.Name != "from_clab" {
		t.Errorf("a module read the wrong section: %+v", opts)
	}
}

func TestDecodeModuleConfigReportsAMissingSection(t *testing.T) {
	cfg, err := loadConfigFrom(t, "name: mc\nmodule:\n  - containerlab\n", nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var opts probeOpts
	found, err := cfg.DecodeModuleConfig("containerlab", &opts)
	if err != nil {
		t.Fatalf("DecodeModuleConfig: %v", err)
	}
	if found {
		t.Error("no section was written, so none should be reported")
	}
}

// TestModuleConfigForAnUnloadedModuleIsRejected catches the case the check
// exists for: a section that does nothing, because the module name is a typo or
// the module was dropped from the list while its settings stayed.
func TestModuleConfigForAnUnloadedModuleIsRejected(t *testing.T) {
	cfg, err := loadConfigFrom(t, `
name: mc
module:
  - containerlab
module_config:
  kathara:
    bridged: true
`, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	err = cfg.CheckModuleConfigNames()
	if err == nil {
		t.Fatal("a section for a module that is not loaded must be rejected")
	}
	if !strings.Contains(err.Error(), "kathara") {
		t.Errorf("the message should name the offending section: %v", err)
	}
}
