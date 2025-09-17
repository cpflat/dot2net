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
			errorMsg:    "conflicting values for 'mtu' in interface class",
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
			errorMsg:    "conflicting prefix values in interface classes",
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

			// Check expected files
			for _, expectedFile := range tt.expectFiles {
				found := false
				for _, file := range files {
					if strings.HasSuffix(file, expectedFile) {
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