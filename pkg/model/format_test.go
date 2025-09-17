package model

import (
	"reflect"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// Helper function to compare ConfigTemplate content instead of pointers
func compareConfigTemplate(a, b *types.ConfigTemplate) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Style == b.Style &&
		a.SortGroup == b.SortGroup &&
		a.Group == b.Group &&
		a.Name == b.Name &&
		a.File == b.File &&
		a.Priority == b.Priority &&
		reflect.DeepEqual(a.Template, b.Template) &&
		reflect.DeepEqual(a.Depends, b.Depends)
}

// Helper function to find template index in result by content
func findTemplateIndex(result []*types.ConfigTemplate, target *types.ConfigTemplate) int {
	for i, template := range result {
		if compareConfigTemplate(template, target) {
			return i
		}
	}
	return -1
}

// Helper function to verify dependency constraints
func verifyDependencyConstraints(t *testing.T, result []*types.ConfigTemplate, testName string) {
	// Create position map for easier lookup
	posMap := make(map[*types.ConfigTemplate]int)
	for i, template := range result {
		posMap[template] = i
	}

	for i, template := range result {
		// Check sorter dependencies: sorter should come after all its group templates
		if template.Style == types.ConfigTemplateStyleSort {
			for j, other := range result {
				if other.Group == template.SortGroup {
					if j >= i {
						t.Errorf("%s: Sorter template (pos %d, SortGroup=%s) should come after group template (pos %d, Group=%s)",
							testName, i, template.SortGroup, j, other.Group)
					}
				}
			}
		}

		// Check named dependencies: template should come after all its dependencies
		for _, depName := range template.Depends {
			for j, other := range result {
				if other.Name == depName {
					if j >= i {
						t.Errorf("%s: Template %s (pos %d) should come after its dependency %s (pos %d)",
							testName, template.Name, i, depName, j)
					}
				}
			}
		}
	}
}

func TestReorderConfigTemplates(t *testing.T) {
	tests := []struct {
		name        string
		templates   []*types.ConfigTemplate
		expectError bool
	}{
		// Test Case 1: Sorter dependencies only
		{
			name: "Sorter dependencies only",
			templates: []*types.ConfigTemplate{
				// Index 0: sorter template (depends on group template)
				{
					Style:     types.ConfigTemplateStyleSort,
					SortGroup: "main.conf",
					File:      "main.conf",
				},
				// Index 1: group template (should come before sorter)
				{
					Group:    "main.conf",
					Priority: -1,
					Template: []string{"log file /var/log/main.log", "!"},
				},
				// Index 2: another group template for same group
				{
					Group:    "main.conf",
					Priority: 0,
					Template: []string{"interface config"},
				},
			},
			expectError: false,
		},

		// Test Case 2: Named dependencies only
		{
			name: "Named dependencies only",
			templates: []*types.ConfigTemplate{
				// Index 0: depends on base_config
				{
					Name:     "advanced_config",
					Depends:  []string{"base_config"},
					Template: []string{"advanced settings"},
				},
				// Index 1: depends on advanced_config and util_config
				{
					Name:     "final_config",
					Depends:  []string{"advanced_config", "util_config"},
					Template: []string{"final settings"},
				},
				// Index 2: base config (no dependencies)
				{
					Name:     "base_config",
					Template: []string{"base settings"},
				},
				// Index 3: utility config (no dependencies)
				{
					Name:     "util_config",
					Template: []string{"utility settings"},
				},
			},
			expectError: false,
		},

		// Test Case 3: Combined dependencies (both Sorter and Named)
		{
			name: "Combined Sorter and Named dependencies",
			templates: []*types.ConfigTemplate{
				// Index 0: sorter that depends on group templates
				{
					Style:     types.ConfigTemplateStyleSort,
					SortGroup: "daemon.conf",
					File:      "daemon.conf",
				},
				// Index 1: group template that depends on named template
				{
					Group:    "daemon.conf",
					Priority: 0,
					Depends:  []string{"init_settings"},
					Template: []string{"daemon settings"},
				},
				// Index 2: named template (depended by group template)
				{
					Name:     "init_settings",
					Template: []string{"initialization"},
				},
				// Index 3: priority group template (no other dependencies)
				{
					Group:    "daemon.conf",
					Priority: -1,
					Template: []string{"log settings"},
				},
			},
			expectError: false,
		},

		// Test Case 4: No dependencies (independent templates)
		{
			name: "No dependencies - independent templates",
			templates: []*types.ConfigTemplate{
				// Index 0: simple file template
				{
					File:     "config1.conf",
					Template: []string{"config1 content"},
				},
				// Index 1: simple named template
				{
					Name:     "standalone_config",
					Template: []string{"standalone content"},
				},
				// Index 2: another file template
				{
					File:     "config2.conf",
					Template: []string{"config2 content"},
				},
				// Index 3: group template without sorter
				{
					Group:    "orphan_group",
					Priority: 0,
					Template: []string{"orphan content"},
				},
			},
			expectError: false,
		},

		// Test Case 5: Error case - circular dependency
		{
			name: "Error case - circular dependency",
			templates: []*types.ConfigTemplate{
				// Index 0: depends on config_b
				{
					Name:     "config_a",
					Depends:  []string{"config_b"},
					Template: []string{"config a"},
				},
				// Index 1: depends on config_a (creates cycle)
				{
					Name:     "config_b",
					Depends:  []string{"config_a"},
					Template: []string{"config b"},
				},
			},
			expectError: true,
		},

		// Test Case 6: Error case - missing dependency
		{
			name: "Error case - missing dependency",
			templates: []*types.ConfigTemplate{
				// Index 0: depends on non-existent template
				{
					Name:     "dependent_config",
					Depends:  []string{"non_existent_config"},
					Template: []string{"dependent content"},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reorderConfigTemplates(tt.templates)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.templates) {
				t.Errorf("Expected %d templates, got %d", len(tt.templates), len(result))
				return
			}

			// Verify dependency constraints are satisfied
			verifyDependencyConstraints(t, result, tt.name)
		})
	}
}

// Test the specific ospf6_topo1 scenario that was failing
func TestReorderConfigTemplates_OSPF6Scenario(t *testing.T) {
	// Simulate the problematic ospf6_topo1 scenario
	templates := []*types.ConfigTemplate{
		// Index 0: ospf6d.conf sorter (should come after group template)
		{
			Style:     types.ConfigTemplateStyleSort,
			SortGroup: "ospf6d.conf",
			File:      "ospf6d.conf",
		},
		// Index 1: ospf6d.conf group template with Priority -1 (should come before sorter)
		{
			Group:    "ospf6d.conf",
			Priority: -1,
			Template: []string{"log file /var/log/frr.log", "!"},
		},
		// Index 2: other templates that should not affect the order
		{
			File:     "other.conf",
			Template: []string{"other config"},
		},
	}

	result, err := reorderConfigTemplates(templates)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Find positions of the ospf6d.conf templates using content comparison
	groupPos := findTemplateIndex(result, templates[1])  // ospf6d.conf group template
	sorterPos := findTemplateIndex(result, templates[0]) // ospf6d.conf sorter template

	if groupPos == -1 {
		t.Error("ospf6d.conf group template not found in result")
	}
	if sorterPos == -1 {
		t.Error("ospf6d.conf sorter template not found in result")
	}

	// The critical test: group template must come before sorter template
	if groupPos >= sorterPos {
		t.Errorf("ospf6d.conf group template (pos %d) should come before sorter template (pos %d)", groupPos, sorterPos)

		// Print the actual order for debugging
		t.Log("Actual order:")
		for i, template := range result {
			t.Logf("  %d: Style=%s, SortGroup=%s, Group=%s, Name=%s, File=%s, Priority=%d",
				i, template.Style, template.SortGroup, template.Group, template.Name, template.File, template.Priority)
		}
	}

	// Additional verification: check all dependency constraints
	verifyDependencyConstraints(t, result, "OSPF6 Scenario")
}

// Test input order independence by verifying dependency constraints only
func TestReorderConfigTemplates_InputOrderIndependence(t *testing.T) {
	tests := []struct {
		name        string
		templates   []*types.ConfigTemplate
		expectError bool
	}{
		// Test Case 1: Sorter dependencies only (reversed)
		{
			name: "Sorter dependencies only (reversed input)",
			templates: []*types.ConfigTemplate{
				// Index 0: another group template for same group (was index 2)
				{
					Group:    "main.conf",
					Priority: 0,
					Template: []string{"interface config"},
				},
				// Index 1: group template (was index 1)
				{
					Group:    "main.conf",
					Priority: -1,
					Template: []string{"log file /var/log/main.log", "!"},
				},
				// Index 2: sorter template (was index 0)
				{
					Style:     types.ConfigTemplateStyleSort,
					SortGroup: "main.conf",
					File:      "main.conf",
				},
			},
			expectError: false,
		},

		// Test Case 2: Named dependencies only (reversed)
		{
			name: "Named dependencies only (reversed input)",
			templates: []*types.ConfigTemplate{
				// Index 0: utility config (was index 3)
				{
					Name:     "util_config",
					Template: []string{"utility settings"},
				},
				// Index 1: base config (was index 2)
				{
					Name:     "base_config",
					Template: []string{"base settings"},
				},
				// Index 2: final config (was index 1)
				{
					Name:     "final_config",
					Depends:  []string{"advanced_config", "util_config"},
					Template: []string{"final settings"},
				},
				// Index 3: advanced config (was index 0)
				{
					Name:     "advanced_config",
					Depends:  []string{"base_config"},
					Template: []string{"advanced settings"},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reorderConfigTemplates(tt.templates)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(result) != len(tt.templates) {
				t.Errorf("Expected %d templates, got %d", len(tt.templates), len(result))
				return
			}

			// The key test: verify dependency constraints are satisfied regardless of input order
			verifyDependencyConstraints(t, result, tt.name)

			// Log the result order for comparison (but don't fail on it)
			t.Logf("Result order for %s:", tt.name)
			for i, template := range result {
				t.Logf("  %d: Style=%s, SortGroup=%s, Group=%s, Name=%s, File=%s, Priority=%d",
					i, template.Style, template.SortGroup, template.Group, template.Name, template.File, template.Priority)
			}
		})
	}
}
