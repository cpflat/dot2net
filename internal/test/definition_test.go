package example

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

// TestDefinitionValidation tests various configuration definitions using table-driven tests
func TestDefinitionValidation(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		dotContent  string
		expectError bool
		errorMsg    string
	}{
		// Conflict Tests - should fail after Primary removal
		{
			name: "NodeClass_Values_Conflict",
			configYAML: `
name: conflict_test
nodeclass:
  - name: class1
    values:
      image: "alpine:3.15"
      kind: "linux"
  - name: class2
    values:
      image: "ubuntu:20.04"
      kind: "linux"
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: true,
			errorMsg:    "configuration conflict detected on node",
		},
		{
			name: "NodeClass_Prefix_Conflict",
			configYAML: `
name: conflict_test
nodeclass:
  - name: class1
    prefix: "router"
  - name: class2
    prefix: "switch"
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: true,
			errorMsg:    "configuration conflict detected on node",
		},
		{
			name: "NodeClass_MgmtInterface_Conflict",
			configYAML: `
name: conflict_test
interfaceclass:
  - name: mgmt1
  - name: mgmt2
nodeclass:
  - name: class1
    mgmt_interfaceclass: "mgmt1"
  - name: class2
    mgmt_interfaceclass: "mgmt2"
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: true,
			errorMsg:    "configuration conflict detected on node",
		},
		{
			name: "InterfaceClass_Values_Conflict",
			configYAML: `
name: conflict_test
interfaceclass:
  - name: class1
    values:
      mtu: "1500"
  - name: class2
    values:
      mtu: "9000"
`,
			dotContent: `
digraph {
  n1 -- n2 [taillabel="class1,class2"];
}
`,
			expectError: true,
			errorMsg:    "different values for 'mtu'",
		},
		{
			// Segment classes are attached through relational labels on the
			// edges, so a segment can carry several of them and needs the same
			// conflict check as the other class types.
			name: "SegmentClass_Values_Conflict",
			configYAML: `
name: conflict_test
layer:
  - name: ip
    default_connect: true
    policy:
      - name: ip
        range: 10.0.0.0/16
        prefix: 24
segmentclass:
  - name: seg1
    layer: ip
    values:
      kind: "bridge"
  - name: seg2
    layer: ip
    values:
      kind: "ovs-bridge"
`,
			dotContent: `
digraph {
  n1 -- n2 [label="segment#seg1; segment#seg2"];
}
`,
			expectError: true,
			errorMsg:    "different values for 'kind'",
		},
		{
			name: "InterfaceClass_Prefix_Conflict",
			configYAML: `
name: conflict_test
interfaceclass:
  - name: class1
    prefix: "eth"
  - name: class2
    prefix: "net"
`,
			dotContent: `
digraph {
  n1 -- n2 [taillabel="class1,class2"];
}
`,
			expectError: true,
			errorMsg:    "different prefix",
		},

		// Valid Cases - should succeed
		{
			name: "NodeClass_Same_Values_Valid",
			configYAML: `
name: valid_test
nodeclass:
  - name: class1
    values:
      image: "alpine:3.15"
      kind: "linux"
  - name: class2
    values:
      image: "alpine:3.15"
      environment: "prod"
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: false,
		},
		{
			name: "NodeClass_Empty_Values_Valid",
			configYAML: `
name: valid_test
nodeclass:
  - name: class1
    values:
      image: "alpine:3.15"
  - name: class2
    prefix: ""
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: false,
		},
		{
			name: "Multiple_Classes_Different_Policies_Valid",
			configYAML: `
name: valid_test
layer:
  - name: ipv4
    policy:
      - name: net1
        range: 192.168.1.0/24
        prefix: 30
      - name: net2
        range: 192.168.2.0/24
        prefix: 30
nodeclass:
  - name: class1
    policy: [net1]
  - name: class2
    policy: [net2]
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: false,
		},
		{
			name: "Virtual_Flag_Combination_Valid",
			configYAML: `
name: valid_test
nodeclass:
  - name: class1
    virtual: true
  - name: class2
    virtual: false
`,
			dotContent: `
digraph {
  n1 [class="class1,class2"];
}
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse configuration
			// Create temporary files for config and dot
			tmpDir, err := os.MkdirTemp("", "definition_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configFile := filepath.Join(tmpDir, "test.yaml")
			dotFile := filepath.Join(tmpDir, "test.dot")

			// Write config file
			err = os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Write dot file
			err = os.WriteFile(dotFile, []byte(tt.dotContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write dot file: %v", err)
			}

			// Load configuration using actual function
			cfg, err := types.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Load DOT diagram using actual function
			nd, err := model.DiagramFromDotFile(dotFile)
			if err != nil {
				t.Fatalf("Failed to load DOT diagram: %v", err)
			}

			// Test BuildNetworkModel
			_, err = model.BuildNetworkModel(cfg, nd, false)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestConnectionClassDefinitions tests Connection/Segment new specification
func TestConnectionClassDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		dotContent  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "ConnectionClass_With_Prefix_Valid",
			configYAML: `
name: connection_test
connectionclass:
  - name: trunk
    prefix: "trunk"
    values:
      vlan_mode: "trunk"
`,
			dotContent: `
digraph {
  n1 -- n2 [label="trunk"];
}
`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse configuration
			// Create temporary files for config and dot
			tmpDir, err := os.MkdirTemp("", "definition_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configFile := filepath.Join(tmpDir, "test.yaml")
			dotFile := filepath.Join(tmpDir, "test.dot")

			// Write config file
			err = os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Write dot file
			err = os.WriteFile(dotFile, []byte(tt.dotContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write dot file: %v", err)
			}

			// Load configuration using actual function
			cfg, err := types.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Load DOT diagram using actual function
			nd, err := model.DiagramFromDotFile(dotFile)
			if err != nil {
				t.Fatalf("Failed to load DOT diagram: %v", err)
			}

			// Test BuildNetworkModel
			_, err = model.BuildNetworkModel(cfg, nd, false)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// TestModuleSystemDefinitions tests module system behavior
func TestModuleSystemDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		configYAML  string
		dotContent  string
		expectFiles []string
	}{
		{
			name: "TiNET_Module_Generates_Spec",
			configYAML: `
name: tinet_test
module:
  - tinet
nodeclass:
  - name: router
    values:
      image: "frr:latest"
`,
			dotContent: `
digraph {
  n1;
}
`,
			expectFiles: []string{"spec.yaml"},
		},
		{
			name: "Containerlab_Module_Generates_Topo",
			configYAML: `
name: clab_test
module:
  - containerlab
nodeclass:
  - name: router
    values:
      image: "frr:latest"
`,
			dotContent: `
digraph {
  n1;
}
`,
			expectFiles: []string{"topo.yaml"},
		},
		{
			name: "Both_Modules_Generate_Both_Files",
			configYAML: `
name: both_test
module:
  - tinet
  - containerlab
nodeclass:
  - name: router
    values:
      image: "frr:latest"
`,
			dotContent: `
digraph {
  n1;
}
`,
			expectFiles: []string{"spec.yaml", "topo.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse configuration
			// Create temporary files for config and dot
			tmpDir, err := os.MkdirTemp("", "definition_test")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			configFile := filepath.Join(tmpDir, "test.yaml")
			dotFile := filepath.Join(tmpDir, "test.dot")

			// Write config file
			err = os.WriteFile(configFile, []byte(tt.configYAML), 0644)
			if err != nil {
				t.Fatalf("Failed to write config file: %v", err)
			}

			// Write dot file
			err = os.WriteFile(dotFile, []byte(tt.dotContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write dot file: %v", err)
			}

			// Load configuration using actual function
			cfg, err := types.LoadConfig(configFile)
			if err != nil {
				t.Fatalf("Failed to load config: %v", err)
			}

			// Load DOT diagram using actual function
			nd, err := model.DiagramFromDotFile(dotFile)
			if err != nil {
				t.Fatalf("Failed to load DOT diagram: %v", err)
			}

			// Build network model
			nm, err := model.BuildNetworkModel(cfg, nd, false)
			if err != nil {
				t.Fatalf("Failed to build network model: %v", err)
			}

			// Test file generation
			files, err := model.ListGeneratedFiles(cfg, nm, false)
			if err != nil {
				t.Fatalf("Failed to list generated files: %v", err)
			}

			// Check expected files.
			// Match on the exact file path (the expected names are top-level
			// generated files); using an exact match rather than HasSuffix
			// avoids "spec.yaml" spuriously matching e.g. "myspec.yaml".
			for _, expectedFile := range tt.expectFiles {
				found := false
				for _, file := range files {
					if file == expectedFile {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file '%s' not found in generated files: %v", expectedFile, files)
				}
			}
		})
	}
}

// TestLabelAttributes pins which DOT attributes carry class labels. The set
// differs per object kind, and an attribute that is not read is ignored in
// silence - example/value_class_basic used xlabel on an edge and its connection
// class simply never applied.
func TestLabelAttributes(t *testing.T) {
	const yaml = `
nodeclass:
  - name: router
connectionclass:
  - name: link
groupclass:
  - name: site
`
	tests := []struct {
		name string
		dot  string
		want bool // whether the class is expected to apply
	}{
		{name: "node xlabel", dot: `digraph { r1 [xlabel="router"]; r2; r1 -> r2 }`, want: true},
		{name: "node class", dot: `digraph { r1 [class="router"]; r2; r1 -> r2 }`, want: true},
		// label on a node is the record-shape syntax, so it must not be parsed
		// as class labels.
		{name: "node label", dot: `digraph { r1 [label="router"]; r2; r1 -> r2 }`, want: false},

		{name: "edge label", dot: `digraph { r1 -> r2 [label="link"] }`, want: true},
		{name: "edge xlabel", dot: `digraph { r1 -> r2 [xlabel="link"] }`, want: true},
		{name: "edge class", dot: `digraph { r1 -> r2 [class="link"] }`, want: true},

		{name: "subgraph label", dot: `digraph { subgraph cluster1 { label="site"; r1 }; r1 -> r2 }`, want: true},
		{name: "subgraph xlabel", dot: `digraph { subgraph cluster1 { xlabel="site"; r1 }; r1 -> r2 }`, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nm := buildModel(t, tt.dot, yaml)

			got := false
			for _, n := range nm.Nodes {
				got = got || containsString(n.ClassLabels(), "router")
			}
			for _, c := range nm.Connections {
				got = got || containsString(c.ClassLabels(), "link")
			}
			for _, g := range nm.Groups {
				got = got || containsString(g.ClassLabels(), "site")
			}

			if got != tt.want {
				t.Errorf("class applied = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// buildModel builds a network model from inline inputs, failing the test on any
// error along the way.
func buildModel(t *testing.T, dot, yaml string) *types.NetworkModel {
	t.Helper()

	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	dotFile := filepath.Join(tmpDir, "test.dot")
	if err := os.WriteFile(configFile, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
		t.Fatalf("failed to write dot file: %v", err)
	}

	cfg, err := types.LoadConfig(configFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	nd, err := model.DiagramFromDotFile(dotFile)
	if err != nil {
		t.Fatalf("failed to load DOT diagram: %v", err)
	}
	nm, err := model.BuildNetworkModel(cfg, nd, false)
	if err != nil {
		t.Fatalf("failed to build network model: %v", err)
	}
	return nm
}

// TestUseClassComposition covers use:, which attaches another class of the same
// type to whatever carries this one. It exists so that a module can offer a
// ready-made class and a scenario can opt into it without naming that class in
// the topology, which would tie the DOT to one platform.
func TestUseClassComposition(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		dot        string
		wantClass  string // class expected on node n1, "" to skip
		wantErrMsg string
	}{
		{
			name: "a used class is attached",
			yaml: `
nodeclass:
  - name: base_sw
    values:
      role: switch
  - name: my_sw
    use: [base_sw]
`,
			dot:       `digraph { n1 [class="my_sw"]; n2; n1 -> n2 }`,
			wantClass: "base_sw",
		},
		{
			name: "use is transitive",
			yaml: `
nodeclass:
  - name: deep
  - name: mid
    use: [deep]
  - name: my_sw
    use: [mid]
`,
			dot:       `digraph { n1 [class="my_sw"]; n2; n1 -> n2 }`,
			wantClass: "deep",
		},
		{
			// A cycle must terminate rather than hang; the visited set is what
			// makes that so.
			name: "a cycle in use terminates",
			yaml: `
nodeclass:
  - name: a
    use: [b]
  - name: b
    use: [a]
`,
			dot:       `digraph { n1 [class="a"]; n2; n1 -> n2 }`,
			wantClass: "b",
		},
		{
			name: "using an undefined class is an error",
			yaml: `
nodeclass:
  - name: my_sw
    use: [nonexistent]
`,
			dot:        `digraph { n1 [class="my_sw"]; n2; n1 -> n2 }`,
			wantErrMsg: "nodeclass my_sw uses undefined nodeclass nonexistent",
		},
		{
			// Two user classes are the same tier, so a clash between them is an
			// error just as it is when both are listed directly.
			name: "same-tier values still clash",
			yaml: `
nodeclass:
  - name: base_sw
    values:
      mtu: "1500"
  - name: my_sw
    use: [base_sw]
    values:
      mtu: "9000"
`,
			dot:        `digraph { n1 [class="my_sw"]; n2; n1 -> n2 }`,
			wantErrMsg: "different values for 'mtu'",
		},
		{
			name: "use works on connection classes too",
			yaml: `
connectionclass:
  - name: base_link
  - name: my_link
    use: [base_link]
`,
			dot: `digraph { n1 -> n2 [label="my_link"] }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErrMsg != "" {
				tmpDir := t.TempDir()
				cfgPath := filepath.Join(tmpDir, "test.yaml")
				dotPath := filepath.Join(tmpDir, "test.dot")
				if err := os.WriteFile(cfgPath, []byte(tt.yaml), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dotPath, []byte(tt.dot), 0644); err != nil {
					t.Fatal(err)
				}
				cfg, err := types.LoadConfig(cfgPath)
				if err != nil {
					t.Fatalf("LoadConfig: %v", err)
				}
				nd, err := model.DiagramFromDotFile(dotPath)
				if err != nil {
					t.Fatalf("DiagramFromDotFile: %v", err)
				}
				_, err = model.BuildNetworkModel(cfg, nd, false)
				if err == nil {
					t.Fatalf("expected an error containing %q, got none", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("unexpected error:\n  got:      %v\n  expected to contain: %s", err, tt.wantErrMsg)
				}
				return
			}

			nm := buildModel(t, tt.dot, tt.yaml)
			if tt.wantClass == "" {
				return
			}
			for _, n := range nm.Nodes {
				if n.Name != "n1" {
					continue
				}
				if !containsString(n.ClassLabels(), tt.wantClass) {
					t.Errorf("node n1 classes = %v, want to contain %q", n.ClassLabels(), tt.wantClass)
				}
			}
		})
	}
}
