package example_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

// TestFileOutput tests various file output configurations
func TestFileOutput(t *testing.T) {
	tests := []struct {
		name          string
		dot           string
		yaml          string
		expectedFiles []string // relative paths from output directory
	}{
		{
			name: "traditional node scope output",
			dot: `graph {
				r1 [class="router"]
				r2 [class="router"]
				r1 -- r2
			}`,
			yaml: `
file:
  - name: config.txt
    scope: node

nodeclass:
  - name: router
    primary: true
    config:
      - file: config.txt
        template:
          - "hostname={{ .name }}"
`,
			expectedFiles: []string{
				"r1/config.txt",
				"r2/config.txt",
			},
		},
		{
			name: "output root with name_suffix (Kathara-style)",
			dot: `graph {
				r1 [class="router"]
				r2 [class="router"]
				r1 -- r2
			}`,
			yaml: `
file:
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root

nodeclass:
  - name: router
    primary: true
    config:
      - file: startup
        template:
          - "#!/bin/bash"
          - "hostname {{ .name }}"
`,
			expectedFiles: []string{
				"r1.startup",
				"r2.startup",
			},
		},
		{
			name: "output root with name_prefix",
			dot: `graph {
				r1 [class="router"]
				r2 [class="router"]
				r1 -- r2
			}`,
			yaml: `
file:
  - name: script
    name_prefix: "init_"
    scope: node
    output: root

nodeclass:
  - name: router
    primary: true
    config:
      - file: script
        template:
          - "echo {{ .name }}"
`,
			expectedFiles: []string{
				"init_r1",
				"init_r2",
			},
		},
		{
			name: "output root with both prefix and suffix",
			dot: `graph {
				r1 [class="router"]
				r2 [class="router"]
				r1 -- r2
			}`,
			yaml: `
file:
  - name: script
    name_prefix: "startup_"
    name_suffix: ".sh"
    scope: node
    output: root

nodeclass:
  - name: router
    primary: true
    config:
      - file: script
        template:
          - "#!/bin/bash"
`,
			expectedFiles: []string{
				"startup_r1.sh",
				"startup_r2.sh",
			},
		},
		{
			name: "mixed output locations",
			dot: `graph {
				r1 [class="router"]
				r2 [class="router"]
				r1 -- r2
			}`,
			yaml: `
file:
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root
  - name: config.txt
    scope: node

nodeclass:
  - name: router
    primary: true
    config:
      - file: startup
        template:
          - "#!/bin/bash"
      - file: config.txt
        template:
          - "hostname={{ .name }}"
`,
			expectedFiles: []string{
				"r1.startup",
				"r2.startup",
				"r1/config.txt",
				"r2/config.txt",
			},
		},
		{
			name: "network scope file",
			dot: `graph {
				r1 [class="router"]
				r2 [class="router"]
				r1 -- r2
			}`,
			yaml: `
file:
  - name: topology.yaml
    scope: network

nodeclass:
  - name: router
    primary: true

networkclass:
  - name: _default
    config:
      - file: topology.yaml
        template:
          - "name: test_network"
`,
			expectedFiles: []string{
				"topology.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()

			// Write input files
			dotFile := filepath.Join(tmpDir, "input.dot")
			yamlFile := filepath.Join(tmpDir, "input.yaml")

			if err := os.WriteFile(dotFile, []byte(tt.dot), 0644); err != nil {
				t.Fatalf("failed to write dot file: %v", err)
			}
			if err := os.WriteFile(yamlFile, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("failed to write yaml file: %v", err)
			}

			// Change to temp directory for file generation
			origDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to change directory: %v", err)
			}
			defer os.Chdir(origDir)

			// Load config and build model
			cfg, err := types.LoadConfig(yamlFile)
			if err != nil {
				t.Fatalf("failed to load config: %v", err)
			}

			d, err := model.DiagramFromDotFile(dotFile)
			if err != nil {
				t.Fatalf("failed to parse dot file: %v", err)
			}

			nm, err := model.BuildNetworkModel(cfg, d, false)
			if err != nil {
				t.Fatalf("failed to build network model: %v", err)
			}

			// Generate config files
			err = model.BuildConfigFiles(cfg, nm, false)
			if err != nil {
				t.Fatalf("failed to build config files: %v", err)
			}

			// Check generated files
			var generatedFiles []string
			err = filepath.WalkDir(tmpDir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}

				relPath, err := filepath.Rel(tmpDir, path)
				if err != nil {
					return err
				}

				// Skip input files
				if relPath == "input.dot" || relPath == "input.yaml" {
					return nil
				}

				generatedFiles = append(generatedFiles, relPath)
				return nil
			})
			if err != nil {
				t.Fatalf("failed to walk directory: %v", err)
			}

			// Sort for comparison
			sort.Strings(generatedFiles)
			sort.Strings(tt.expectedFiles)

			// Compare
			if len(generatedFiles) != len(tt.expectedFiles) {
				t.Errorf("file count mismatch:\n  got:      %v\n  expected: %v", generatedFiles, tt.expectedFiles)
				return
			}

			for i, expected := range tt.expectedFiles {
				// Normalize path separators for cross-platform
				expected = filepath.FromSlash(expected)
				if generatedFiles[i] != expected {
					t.Errorf("file mismatch at index %d:\n  got:      %s\n  expected: %s", i, generatedFiles[i], expected)
				}
			}
		})
	}
}

// TestFileOutputContent tests that file contents are generated correctly
func TestFileOutputContent(t *testing.T) {
	tmpDir := t.TempDir()

	dot := `graph {
		r1 [class="router"]
		r1 -- r2
		r2 [class="router"]
	}`

	yaml := `
file:
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root

nodeclass:
  - name: router
    primary: true
    config:
      - file: startup
        template:
          - "#!/bin/bash"
          - "hostname {{ .name }}"
`

	// Write input files
	dotFile := filepath.Join(tmpDir, "input.dot")
	yamlFile := filepath.Join(tmpDir, "input.yaml")

	if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
		t.Fatalf("failed to write dot file: %v", err)
	}
	if err := os.WriteFile(yamlFile, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write yaml file: %v", err)
	}

	// Change to temp directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	defer os.Chdir(origDir)

	// Load and build
	cfg, err := types.LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	d, err := model.DiagramFromDotFile(dotFile)
	if err != nil {
		t.Fatalf("failed to parse dot file: %v", err)
	}

	nm, err := model.BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("failed to build network model: %v", err)
	}

	err = model.BuildConfigFiles(cfg, nm, false)
	if err != nil {
		t.Fatalf("failed to build config files: %v", err)
	}

	// Check r1.startup content
	content, err := os.ReadFile(filepath.Join(tmpDir, "r1.startup"))
	if err != nil {
		t.Fatalf("failed to read r1.startup: %v", err)
	}

	expectedContent := "#!/bin/bash\nhostname r1"
	if string(content) != expectedContent {
		t.Errorf("content mismatch:\n  got:\n%s\n  expected:\n%s", string(content), expectedContent)
	}

	// Check r2.startup exists
	if _, err := os.Stat(filepath.Join(tmpDir, "r2.startup")); os.IsNotExist(err) {
		t.Error("r2.startup was not created")
	}
}

// TestListGeneratedFilesWithOutput tests ListGeneratedFiles with Output field
func TestListGeneratedFilesWithOutput(t *testing.T) {
	tmpDir := t.TempDir()

	dot := `graph {
		r1 [class="router"]
		r2 [class="router"]
		r1 -- r2
	}`

	yaml := `
file:
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root
  - name: config.txt
    scope: node

nodeclass:
  - name: router
    primary: true
    config:
      - file: startup
        template: ["#!/bin/bash"]
      - file: config.txt
        template: ["hostname={{ .name }}"]
`

	// Write input files
	dotFile := filepath.Join(tmpDir, "input.dot")
	yamlFile := filepath.Join(tmpDir, "input.yaml")

	if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
		t.Fatalf("failed to write dot file: %v", err)
	}
	if err := os.WriteFile(yamlFile, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write yaml file: %v", err)
	}

	// Load and build model (no need to change directory for ListGeneratedFiles)
	cfg, err := types.LoadConfig(yamlFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	d, err := model.DiagramFromDotFile(dotFile)
	if err != nil {
		t.Fatalf("failed to parse dot file: %v", err)
	}

	nm, err := model.BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("failed to build network model: %v", err)
	}

	// Get list of files
	files, err := model.ListGeneratedFiles(cfg, nm, false)
	if err != nil {
		t.Fatalf("failed to list generated files: %v", err)
	}

	expectedFiles := []string{
		"r1.startup",
		"r1/config.txt",
		"r2.startup",
		"r2/config.txt",
	}

	sort.Strings(files)
	sort.Strings(expectedFiles)

	if len(files) != len(expectedFiles) {
		t.Errorf("file count mismatch:\n  got:      %v\n  expected: %v", files, expectedFiles)
		return
	}

	for i, expected := range expectedFiles {
		// Normalize for comparison
		got := strings.ReplaceAll(files[i], "\\", "/")
		if got != expected {
			t.Errorf("file mismatch at index %d:\n  got:      %s\n  expected: %s", i, got, expected)
		}
	}
}
