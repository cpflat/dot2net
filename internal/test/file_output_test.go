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
		{
			// A group-scope file lands in the group's directory by default,
			// the same way a node-scope file lands in the node's. This holds
			// whether or not output_group_class (unset here) makes the node
			// files join it.
			name: "group scope file",
			dot: `graph {
				subgraph cluster_h1 { label="worker"; r1 [class="router"] }
				subgraph cluster_h2 { label="worker"; r2 [class="router"] }
				r1 -- r2
			}`,
			yaml: `
file:
  - name: host.yaml
    scope: group
  - name: config.txt
    scope: node

nodeclass:
  - name: router
    config:
      - file: config.txt
        template:
          - "hostname={{ .name }}"

groupclass:
  - name: worker
    config:
      - file: host.yaml
        template:
          - "host: {{ .name }}"
`,
			expectedFiles: []string{
				"cluster_h1/host.yaml",
				"cluster_h2/host.yaml",
				"r1/config.txt",
				"r2/config.txt",
			},
		},
		{
			// output: root escapes the group directory, so the filename has to
			// carry the group name to stay unique.
			name: "group scope file at the output root",
			dot: `graph {
				subgraph cluster_h1 { label="worker"; r1 [class="router"] }
				subgraph cluster_h2 { label="worker"; r2 [class="router"] }
				r1 -- r2
			}`,
			yaml: `
file:
  - name: host
    name_suffix: ".yaml"
    scope: group
    output: root

nodeclass:
  - name: router

groupclass:
  - name: worker
    config:
      - file: host
        template:
          - "host: {{ .name }}"
`,
			expectedFiles: []string{
				"cluster_h1.yaml",
				"cluster_h2.yaml",
			},
		},
		{
			// The layout REQ-2 asks for: everything belonging to a host lives
			// under that host's directory, so the directory can be archived as
			// a unit.
			name: "output group class puts node files under the group directory",
			dot: `graph {
				subgraph cluster_h1 { label="worker"; r1 [class="router"]; r2 [class="router"] }
				subgraph cluster_h2 { label="worker"; r3 [class="router"] }
				r1 -- r2
				r2 -- r3
			}`,
			yaml: `
global:
  output_group_class: worker

file:
  - name: host.yaml
    scope: group
  - name: config.txt
    scope: node
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root
  - name: topology.yaml
    scope: network

nodeclass:
  - name: router
    config:
      - file: config.txt
        template:
          - "hostname={{ .name }}"
      - file: startup
        template:
          - "#!/bin/bash"

groupclass:
  - name: worker
    config:
      - file: host.yaml
        template:
          - "host: {{ .name }}"

networkclass:
  - name: _default
    config:
      - file: topology.yaml
        template:
          - "name: test_network"
`,
			expectedFiles: []string{
				// network scope belongs to no host, so it stays at the true root
				"topology.yaml",
				"cluster_h1/host.yaml",
				"cluster_h1/r1.startup",
				"cluster_h1/r2.startup",
				"cluster_h1/r1/config.txt",
				"cluster_h1/r2/config.txt",
				"cluster_h2/host.yaml",
				"cluster_h2/r3.startup",
				"cluster_h2/r3/config.txt",
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
			defer func() {
				if err := os.Chdir(origDir); err != nil {
					t.Errorf("failed to restore working directory: %v", err)
				}
			}()

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
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

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

// buildInDir writes the inputs into a fresh temporary directory, generates the
// config files there, and returns the directory. Errors from the build are
// returned rather than reported so that callers can assert on them.
func buildInDir(t *testing.T, dot, yaml string) (string, error) {
	t.Helper()

	tmpDir := t.TempDir()
	dotFile := filepath.Join(tmpDir, "input.dot")
	yamlFile := filepath.Join(tmpDir, "input.yaml")
	if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
		t.Fatalf("failed to write dot file: %v", err)
	}
	if err := os.WriteFile(yamlFile, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write yaml file: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	})

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
		return tmpDir, err
	}
	return tmpDir, model.BuildConfigFiles(cfg, nm, false)
}

const groupScopeDot = `graph {
	subgraph cluster_h1 { label="worker"; r1 [class="router"]; r2 [class="router"] }
	subgraph cluster_h2 { label="worker"; r3 [class="router"] }
	r1 -- r2
	r2 -- r3
}`

// TestGroupScopeAggregation checks that a group-scope file sees only the
// objects of its own group: its member nodes, and the connections with both
// endpoints inside it. The r2--r3 connection crosses the group boundary and
// must appear in neither host file.
func TestGroupScopeAggregation(t *testing.T) {
	const yaml = `
file:
  - name: host.yaml
    scope: group

nodeclass:
  - name: router
    config:
      - name: node_entry
        template:
          - "  - {{ .name }}"

connectionclass:
  - name: _default
    config:
      - name: link_entry
        template:
          - "  - {{ .name }}"

class_policy:
  connection:
    default: [_default]

groupclass:
  - name: worker
    config:
      - file: host.yaml
        template:
          - "nodes:"
          - "{{ .nodes_node_entry }}"
          - "links:"
          - "{{ .connections_link_entry }}"
`

	tmpDir, err := buildInDir(t, groupScopeDot, yaml)
	if err != nil {
		t.Fatalf("failed to build config files: %v", err)
	}

	expected := map[string]string{
		"cluster_h1/host.yaml": "nodes:\n  - r1\n  - r2\nlinks:\n  - conn0",
		"cluster_h2/host.yaml": "nodes:\n  - r3\nlinks:\n",
	}
	for name, want := range expected {
		got, err := os.ReadFile(filepath.Join(tmpDir, name))
		if err != nil {
			t.Errorf("failed to read %s: %v", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s mismatch:\n  got:      %q\n  expected: %q", name, string(got), want)
		}
	}
}

// TestOutputGroupClassAmbiguity checks that a node in two groups of the output
// directory class is rejected instead of having one of them picked silently.
func TestOutputGroupClassAmbiguity(t *testing.T) {
	const dot = `graph {
		subgraph cluster_h1 { label="worker"; r1 [class="router"] }
		subgraph cluster_h2 { label="worker"; r1 }
		r1 -- r2
		r2 [class="router"]
	}`
	const yaml = `
global:
  output_group_class: worker

file:
  - name: config.txt
    scope: node

nodeclass:
  - name: router
    config:
      - file: config.txt
        template:
          - "hostname={{ .name }}"

groupclass:
  - name: worker
`

	_, err := buildInDir(t, dot, yaml)
	if err == nil {
		t.Fatal("expected an error for a node in two output-directory groups, got none")
	}
	if !strings.Contains(err.Error(), "belongs to more than one worker group") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestSegmentClassValues checks that the values of a segment class reach the
// segment's namespace, the same way they do for the other class types. Segment
// classes are attached through relational labels on the edges, and a hub node
// collapses its edges into one segment.
func TestSegmentClassValues(t *testing.T) {
	const dot = `digraph {
		r1 [xlabel="router"]; r2 [xlabel="router"]; r3 [xlabel="router"];
		hub1 [xlabel="hub"];
		r1 -> hub1 [dir="none", label="segment#shared"];
		r2 -> hub1 [dir="none", label="segment#shared"];
		r3 -> hub1 [dir="none", label="segment#shared"];
	}`
	const yaml = `
file:
  - name: seg.txt
    scope: network

layer:
  - name: ip
    default_connect: true
    policy:
      - name: ip
        range: 10.0.0.0/16
        prefix: 24

nodeclass:
  - name: router
    interface_policy: [ip]
  - name: hub

segmentclass:
  - name: shared
    layer: ip
    values:
      kind: ovs-bridge
      mtu: "9000"
    config:
      - name: seg_entry
        template:
          - "{{ .name }} kind={{ .kind }} mtu={{ .mtu }}"

networkclass:
  - name: _default
    config:
      - file: seg.txt
        template:
          - "{{ .segments_ip_seg_entry }}"
`

	tmpDir, err := buildInDir(t, dot, yaml)
	if err != nil {
		t.Fatalf("failed to build config files: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "seg.txt"))
	if err != nil {
		t.Fatalf("failed to read seg.txt: %v", err)
	}
	// The hub is layer-unaware, so the three edges collapse into one segment.
	const want = "seg0 kind=ovs-bridge mtu=9000"
	if string(got) != want {
		t.Errorf("seg.txt mismatch:\n  got:      %q\n  expected: %q", string(got), want)
	}
}

// TestSegmentAggregationSeparatesLayers checks that segments of different
// layers land in different aggregation parameters. The parent sees every
// layer's segments in one call, so without the layer in the parameter name the
// two layers merge silently.
func TestSegmentAggregationSeparatesLayers(t *testing.T) {
	const dot = `digraph {
		r1 [xlabel="router"]; r2 [xlabel="router"];
		r1 -> r2 [dir="none", label="segment#seg_a; segment#seg_b"];
	}`
	const yaml = `
file:
  - name: seg.txt
    scope: network

layer:
  - name: la
    default_connect: true
    policy:
      - name: ipa
        range: 10.0.0.0/16
        prefix: 24
  - name: lb
    default_connect: true
    policy:
      - name: ipb
        range: 10.1.0.0/16
        prefix: 24

nodeclass:
  - name: router
    interface_policy: [ipa, ipb]

segmentclass:
  - name: seg_a
    layer: la
    values:
      tag: "A"
    config:
      - name: entry
        template:
          - "{{ .tag }}"
  - name: seg_b
    layer: lb
    values:
      tag: "B"
    config:
      - name: entry
        template:
          - "{{ .tag }}"

networkclass:
  - name: _default
    config:
      - file: seg.txt
        template:
          - "la={{ .segments_la_entry }} lb={{ .segments_lb_entry }}"
`

	tmpDir, err := buildInDir(t, dot, yaml)
	if err != nil {
		t.Fatalf("failed to build config files: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(tmpDir, "seg.txt"))
	if err != nil {
		t.Fatalf("failed to read seg.txt: %v", err)
	}
	const want = "la=A lb=B"
	if string(got) != want {
		t.Errorf("seg.txt mismatch:\n  got:      %q\n  expected: %q", string(got), want)
	}
}

// TestClabBridgeSetupClasses covers the ready-made setup classes the
// containerlab module offers. They are opt-in: which command creates a bridge
// is not decided by the kind, so a topology that provisions its bridges
// differently names neither class and writes its own template.
//
// What they write into is worker_deploy, the group every platform's entry
// script gathers - the classes are containerlab's because the kinds are, not
// because the hook is. A topology writing into the same group says which side
// of the platform's own command it goes, by naming it.
func TestClabBridgeSetupClasses(t *testing.T) {
	const dot = `digraph {
		r1 [xlabel="router"];
		sw [xlabel="my_sw"];
		r1 -> sw [dir="none"];
	}`
	head := func(use string) string {
		return `
module: [containerlab]

class_policy:
  interface:
    default: [default]

module_config:
  containerlab:
    generate_scripts: true

layer:
  - name: ip
    default_connect: true
    policy:
      - name: ip
        range: 10.0.0.0/16
        prefix: 24

nodeclass:
  - name: router
    interface_policy: [ip]
    values: {kind: linux, image: alpine}
    config:
      - name: startup
        template: []
  - name: my_sw
    deploy: platform
    values: {kind: ovs-bridge}
` + use + `
interfaceclass:
  - name: default

`
	}

	tests := []struct {
		name  string
		yaml  string
		block string   // which function of the script the block lands in
		want  []string // substrings, in the order they must appear
	}{
		{
			name: "the OVS class supplies its command",
			yaml: head("    use: [clabOvsBridgeSetup]\n"),
			want: []string{"ovs-vsctl br-exists", "ovs-vsctl add-br"},
		},
		{
			name: "the Linux bridge class supplies a different one",
			yaml: head("    use: [clabLinuxBridgeSetup]\n"),
			want: []string{"ip link show", "ip link add", "type bridge", "ip link set"},
		},
		{
			// What the module has to do comes first, so the topology's own
			// commands run on ground it has prepared - here, a bridge that
			// exists by the time anything is attached to it.
			name: "a topology adds to the class, and the class goes first",
			yaml: head(`    use: [clabOvsBridgeSetup]
    config:
      - group: worker_deploy
        placed:
          before: [worker_deploy]
        template:
          - "ovs-vsctl add-port {{ .clab_bridge }} eth9"
`),
			want: []string{"ovs-vsctl add-br", "ovs-vsctl add-port"},
		},
		{
			// The same rule seen from the other side: worker_destroy takes
			// something apart, so the module's part is taken up last.
			name: "the class goes last where the hook undoes something",
			yaml: head(`    use: [clabOvsBridgeSetup]
    config:
      - group: worker_destroy
        placed:
          after: [worker_destroy]
        template:
          - "ovs-vsctl del-port {{ .clab_bridge }} eth9"
`),
			block: "run_worker_destroy() {\n  :\n",
			want:  []string{"containerlab destroy", "ovs-vsctl del-port", "ovs-vsctl --if-exists del-br"},
		},
		{
			// Opting out is the point: nothing is chosen from the kind.
			name: "a topology can write its own instead",
			yaml: head(`    config:
      - group: worker_deploy
        placed:
          before: [worker_deploy]
        template:
          - "ansible-playbook provision-bridge.yml -e name={{ .name }}"
`),
			want: []string{"ansible-playbook provision-bridge.yml -e name=sw"},
		},
		{
			// A block that has to wait for the platform can say so with a
			// number as well, the command being the origin of the axis.
			name: "a positive priority puts the block after the platform's command",
			yaml: head(`    config:
      - group: worker_deploy
        priority: 10
        template:
          - "tc qdisc add dev {{ .clab_bridge }} root netem delay 1ms"
`),
			want: []string{"containerlab deploy", "tc qdisc add dev"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := buildInDir(t, dot, tt.yaml)
			if err != nil {
				t.Fatalf("failed to build config files: %v", err)
			}
			got, err := os.ReadFile(filepath.Join(tmpDir, "containerlab.sh"))
			if err != nil {
				t.Fatalf("failed to read containerlab.sh: %v", err)
			}
			opening := tt.block
			if opening == "" {
				opening = "run_worker_deploy() {\n  :\n"
			}
			body := between(string(got), opening, "\n}\n")
			at := 0
			for _, want := range tt.want {
				i := strings.Index(body[at:], want)
				if i < 0 {
					t.Fatalf("%q not found after position %d in:\n%s", want, at, body)
				}
				at += i + len(want)
			}
		})
	}
}

// between returns what lies between two markers, or "" if either is missing.
func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}
