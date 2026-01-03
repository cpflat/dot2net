package model

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

func TestDetermineFileFormat(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		explicitFormat string
		expected       string
	}{
		{"yaml extension", "data.yaml", "", "yaml"},
		{"yml extension", "data.yml", "", "yaml"},
		{"json extension", "data.json", "", "json"},
		{"csv extension", "data.csv", "", "csv"},
		{"txt extension defaults to text", "data.txt", "", "text"},
		{"unknown extension defaults to text", "data.dat", "", "text"},
		{"no extension defaults to text", "data", "", "text"},
		{"explicit format overrides extension", "data.txt", "yaml", "yaml"},
		{"explicit json overrides yaml extension", "data.yaml", "json", "json"},
		{"uppercase extension", "data.YAML", "", "yaml"},
		{"mixed case extension", "data.Json", "", "json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineFileFormat(tt.path, tt.explicitFormat)
			if result != tt.expected {
				t.Errorf("determineFileFormat(%q, %q) = %q, want %q",
					tt.path, tt.explicitFormat, result, tt.expected)
			}
		})
	}
}

func TestParseTextFile(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "value1\nvalue2\n\nvalue3\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params, err := parseTextFile(tmpFile)
	if err != nil {
		t.Fatalf("parseTextFile failed: %v", err)
	}

	if len(params) != 3 {
		t.Errorf("expected 3 params, got %d", len(params))
	}

	expected := []struct {
		value string
		index string
	}{
		{"value1", "0"},
		{"value2", "1"},
		{"value3", "2"},
	}

	for i, exp := range expected {
		if params[i]["value"] != exp.value {
			t.Errorf("params[%d][value] = %q, want %q", i, params[i]["value"], exp.value)
		}
		if params[i]["index"] != exp.index {
			t.Errorf("params[%d][index] = %q, want %q", i, params[i]["index"], exp.index)
		}
	}
}

func TestParseYAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yaml")
	content := `- network: "10.0.0.0/8"
  gateway: "192.168.1.1"
- network: "172.16.0.0/12"
  gateway: "192.168.1.2"
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params, err := parseYAMLFile(tmpFile)
	if err != nil {
		t.Fatalf("parseYAMLFile failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Check first entry
	if params[0]["network"] != "10.0.0.0/8" {
		t.Errorf("params[0][network] = %q, want %q", params[0]["network"], "10.0.0.0/8")
	}
	if params[0]["gateway"] != "192.168.1.1" {
		t.Errorf("params[0][gateway] = %q, want %q", params[0]["gateway"], "192.168.1.1")
	}
	if params[0]["index"] != "0" {
		t.Errorf("params[0][index] = %q, want %q", params[0]["index"], "0")
	}

	// Check second entry
	if params[1]["network"] != "172.16.0.0/12" {
		t.Errorf("params[1][network] = %q, want %q", params[1]["network"], "172.16.0.0/12")
	}
	if params[1]["index"] != "1" {
		t.Errorf("params[1][index] = %q, want %q", params[1]["index"], "1")
	}
}

func TestParseJSONFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.json")
	content := `[
  {"network": "10.0.0.0/8", "gateway": "192.168.1.1"},
  {"network": "172.16.0.0/12", "gateway": "192.168.1.2"}
]`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params, err := parseJSONFile(tmpFile)
	if err != nil {
		t.Fatalf("parseJSONFile failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	if params[0]["network"] != "10.0.0.0/8" {
		t.Errorf("params[0][network] = %q, want %q", params[0]["network"], "10.0.0.0/8")
	}
	if params[0]["gateway"] != "192.168.1.1" {
		t.Errorf("params[0][gateway] = %q, want %q", params[0]["gateway"], "192.168.1.1")
	}
	if params[0]["index"] != "0" {
		t.Errorf("params[0][index] = %q, want %q", params[0]["index"], "0")
	}

	if params[1]["network"] != "172.16.0.0/12" {
		t.Errorf("params[1][network] = %q, want %q", params[1]["network"], "172.16.0.0/12")
	}
	if params[1]["index"] != "1" {
		t.Errorf("params[1][index] = %q, want %q", params[1]["index"], "1")
	}
}

func TestParseCSVFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.csv")
	content := `network,gateway,metric
10.0.0.0/8,192.168.1.1,100
172.16.0.0/12,192.168.1.2,200
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params, err := parseCSVFile(tmpFile)
	if err != nil {
		t.Fatalf("parseCSVFile failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Check first entry
	if params[0]["network"] != "10.0.0.0/8" {
		t.Errorf("params[0][network] = %q, want %q", params[0]["network"], "10.0.0.0/8")
	}
	if params[0]["gateway"] != "192.168.1.1" {
		t.Errorf("params[0][gateway] = %q, want %q", params[0]["gateway"], "192.168.1.1")
	}
	if params[0]["metric"] != "100" {
		t.Errorf("params[0][metric] = %q, want %q", params[0]["metric"], "100")
	}
	if params[0]["index"] != "0" {
		t.Errorf("params[0][index] = %q, want %q", params[0]["index"], "0")
	}

	// Check second entry
	if params[1]["network"] != "172.16.0.0/12" {
		t.Errorf("params[1][network] = %q, want %q", params[1]["network"], "172.16.0.0/12")
	}
	if params[1]["metric"] != "200" {
		t.Errorf("params[1][metric] = %q, want %q", params[1]["metric"], "200")
	}
	if params[1]["index"] != "1" {
		t.Errorf("params[1][index] = %q, want %q", params[1]["index"], "1")
	}
}

func TestParseFileSource(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		filename       string
		content        string
		explicitFormat string
		expectedLen    int
		checkFirst     map[string]string
	}{
		{
			name:        "auto-detect yaml",
			filename:    "data.yaml",
			content:     "- id: \"100\"\n  name: test\n",
			expectedLen: 1,
			checkFirst:  map[string]string{"id": "100", "name": "test", "index": "0"},
		},
		{
			name:        "auto-detect json",
			filename:    "data.json",
			content:     `[{"id": "200", "name": "json-test"}]`,
			expectedLen: 1,
			checkFirst:  map[string]string{"id": "200", "name": "json-test", "index": "0"},
		},
		{
			name:        "auto-detect csv",
			filename:    "data.csv",
			content:     "id,name\n300,csv-test\n",
			expectedLen: 1,
			checkFirst:  map[string]string{"id": "300", "name": "csv-test", "index": "0"},
		},
		{
			name:        "auto-detect text",
			filename:    "data.txt",
			content:     "line1\nline2\n",
			expectedLen: 2,
			checkFirst:  map[string]string{"value": "line1", "index": "0"},
		},
		{
			name:           "explicit format overrides extension",
			filename:       "data.txt",
			content:        `[{"key": "value"}]`,
			explicitFormat: "json",
			expectedLen:    1,
			checkFirst:     map[string]string{"key": "value", "index": "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			params, err := parseFileSource(tmpFile, tt.explicitFormat)
			if err != nil {
				t.Fatalf("parseFileSource failed: %v", err)
			}

			if len(params) != tt.expectedLen {
				t.Errorf("expected %d params, got %d", tt.expectedLen, len(params))
			}

			if len(params) > 0 {
				for key, expectedVal := range tt.checkFirst {
					if params[0][key] != expectedVal {
						t.Errorf("params[0][%s] = %q, want %q", key, params[0][key], expectedVal)
					}
				}
			}
		})
	}
}

func TestParseYAMLFileWithNumericValues(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yaml")
	// Test that numeric values are properly converted to strings
	content := `- vlan_id: 100
  priority: 1
- vlan_id: 200
  priority: 2
`
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	params, err := parseYAMLFile(tmpFile)
	if err != nil {
		t.Fatalf("parseYAMLFile failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Numeric values should be converted to strings
	if params[0]["vlan_id"] != "100" {
		t.Errorf("params[0][vlan_id] = %q, want %q", params[0]["vlan_id"], "100")
	}
	if params[0]["priority"] != "1" {
		t.Errorf("params[0][priority] = %q, want %q", params[0]["priority"], "1")
	}
}

func TestParseEmptyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("empty text file", func(t *testing.T) {
		tmpFile := filepath.Join(tmpDir, "empty.txt")
		if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		params, err := parseTextFile(tmpFile)
		if err != nil {
			t.Fatalf("parseTextFile failed: %v", err)
		}
		if len(params) != 0 {
			t.Errorf("expected 0 params for empty file, got %d", len(params))
		}
	})

	t.Run("empty csv file", func(t *testing.T) {
		tmpFile := filepath.Join(tmpDir, "empty.csv")
		if err := os.WriteFile(tmpFile, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		params, err := parseCSVFile(tmpFile)
		if err != nil {
			t.Fatalf("parseCSVFile failed: %v", err)
		}
		if params != nil && len(params) != 0 {
			t.Errorf("expected nil or empty params for empty csv, got %d", len(params))
		}
	})

	t.Run("csv with header only", func(t *testing.T) {
		tmpFile := filepath.Join(tmpDir, "header_only.csv")
		if err := os.WriteFile(tmpFile, []byte("id,name\n"), 0644); err != nil {
			t.Fatal(err)
		}

		params, err := parseCSVFile(tmpFile)
		if err != nil {
			t.Fatalf("parseCSVFile failed: %v", err)
		}
		if len(params) != 0 {
			t.Errorf("expected 0 params for header-only csv, got %d", len(params))
		}
	})
}

func TestGenerateValuesFromSource_Sequence(t *testing.T) {
	cfg := &types.Config{}

	tests := []struct {
		name        string
		start       int
		end         int
		expectedLen int
		checkFirst  map[string]string
		checkLast   map[string]string
	}{
		{
			name:        "basic sequence 0-5",
			start:       0,
			end:         5,
			expectedLen: 5,
			checkFirst:  map[string]string{"value": "0", "index": "0"},
			checkLast:   map[string]string{"value": "4", "index": "4"},
		},
		{
			name:        "sequence with offset",
			start:       10,
			end:         13,
			expectedLen: 3,
			checkFirst:  map[string]string{"value": "0", "index": "0"},
			checkLast:   map[string]string{"value": "2", "index": "2"},
		},
		{
			name:        "zero count defaults to 10",
			start:       0,
			end:         0,
			expectedLen: 10,
			checkFirst:  map[string]string{"value": "0", "index": "0"},
			checkLast:   map[string]string{"value": "9", "index": "9"},
		},
		{
			name:        "negative count defaults to 10",
			start:       5,
			end:         3,
			expectedLen: 10,
			checkFirst:  map[string]string{"value": "0", "index": "0"},
			checkLast:   map[string]string{"value": "9", "index": "9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &types.ParameterRule{
				Name: "test_seq",
				Mode: "attach",
				Source: &types.ParameterRuleSource{
					Type:  "sequence",
					Start: tt.start,
					End:   tt.end,
				},
			}

			params, err := generateValueParamsFromSource(cfg, rule)
			if err != nil {
				t.Fatalf("generateValueParamsFromSource failed: %v", err)
			}

			if len(params) != tt.expectedLen {
				t.Errorf("expected %d params, got %d", tt.expectedLen, len(params))
			}

			if len(params) > 0 {
				for key, expectedVal := range tt.checkFirst {
					if params[0][key] != expectedVal {
						t.Errorf("first param[%s] = %q, want %q", key, params[0][key], expectedVal)
					}
				}
				for key, expectedVal := range tt.checkLast {
					if params[len(params)-1][key] != expectedVal {
						t.Errorf("last param[%s] = %q, want %q", key, params[len(params)-1][key], expectedVal)
					}
				}
			}
		})
	}
}

func TestGenerateValuesFromSource_Range(t *testing.T) {
	cfg := &types.Config{}

	tests := []struct {
		name        string
		start       int
		end         int
		expectedLen int
		checkFirst  map[string]string
		checkLast   map[string]string
	}{
		{
			name:        "range 100-102",
			start:       100,
			end:         102,
			expectedLen: 3,
			checkFirst:  map[string]string{"value": "100", "index": "0"},
			checkLast:   map[string]string{"value": "102", "index": "2"},
		},
		{
			name:        "range 0-2",
			start:       0,
			end:         2,
			expectedLen: 3,
			checkFirst:  map[string]string{"value": "0", "index": "0"},
			checkLast:   map[string]string{"value": "2", "index": "2"},
		},
		{
			name:        "single value range",
			start:       50,
			end:         50,
			expectedLen: 1,
			checkFirst:  map[string]string{"value": "50", "index": "0"},
			checkLast:   map[string]string{"value": "50", "index": "0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &types.ParameterRule{
				Name: "test_range",
				Mode: "attach",
				Source: &types.ParameterRuleSource{
					Type:  "range",
					Start: tt.start,
					End:   tt.end,
				},
			}

			params, err := generateValueParamsFromSource(cfg, rule)
			if err != nil {
				t.Fatalf("generateValueParamsFromSource failed: %v", err)
			}

			if len(params) != tt.expectedLen {
				t.Errorf("expected %d params, got %d", tt.expectedLen, len(params))
			}

			if len(params) > 0 {
				for key, expectedVal := range tt.checkFirst {
					if params[0][key] != expectedVal {
						t.Errorf("first param[%s] = %q, want %q", key, params[0][key], expectedVal)
					}
				}
				for key, expectedVal := range tt.checkLast {
					if params[len(params)-1][key] != expectedVal {
						t.Errorf("last param[%s] = %q, want %q", key, params[len(params)-1][key], expectedVal)
					}
				}
			}
		})
	}
}

func TestGenerateValuesFromSource_List(t *testing.T) {
	cfg := &types.Config{}

	rule := &types.ParameterRule{
		Name: "test_list",
		Mode: "attach",
		Source: &types.ParameterRuleSource{
			Type: "list",
			Values: []map[string]interface{}{
				{"network": "10.0.0.0/8", "gateway": "192.168.1.1"},
				{"network": "172.16.0.0/12", "gateway": "192.168.1.2"},
			},
		},
	}

	params, err := generateValueParamsFromSource(cfg, rule)
	if err != nil {
		t.Fatalf("generateValueParamsFromSource failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Check first entry
	if params[0]["network"] != "10.0.0.0/8" {
		t.Errorf("params[0][network] = %q, want %q", params[0]["network"], "10.0.0.0/8")
	}
	if params[0]["gateway"] != "192.168.1.1" {
		t.Errorf("params[0][gateway] = %q, want %q", params[0]["gateway"], "192.168.1.1")
	}
	if params[0]["index"] != "0" {
		t.Errorf("params[0][index] = %q, want %q", params[0]["index"], "0")
	}

	// Check second entry
	if params[1]["network"] != "172.16.0.0/12" {
		t.Errorf("params[1][network] = %q, want %q", params[1]["network"], "172.16.0.0/12")
	}
	if params[1]["index"] != "1" {
		t.Errorf("params[1][index] = %q, want %q", params[1]["index"], "1")
	}
}

func TestGenerateValuesFromSource_ListWithNumericValues(t *testing.T) {
	cfg := &types.Config{}

	rule := &types.ParameterRule{
		Name: "test_list_numeric",
		Mode: "attach",
		Source: &types.ParameterRuleSource{
			Type: "list",
			Values: []map[string]interface{}{
				{"vlan_id": 100, "priority": 1},
				{"vlan_id": 200, "priority": 2},
			},
		},
	}

	params, err := generateValueParamsFromSource(cfg, rule)
	if err != nil {
		t.Fatalf("generateValueParamsFromSource failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Numeric values should be converted to strings
	if params[0]["vlan_id"] != "100" {
		t.Errorf("params[0][vlan_id] = %q, want %q", params[0]["vlan_id"], "100")
	}
	if params[0]["priority"] != "1" {
		t.Errorf("params[0][priority] = %q, want %q", params[0]["priority"], "1")
	}
}

func TestGenerateValuesFromSource_ParamFormat(t *testing.T) {
	cfg := &types.Config{}

	rule := &types.ParameterRule{
		Name: "test_param_format",
		Mode: "attach",
		Source: &types.ParameterRuleSource{
			Type:  "range",
			Start: 100,
			End:   101,
		},
		ParamFormat: map[string]string{
			"vlan_id":   "{{ .value }}",
			"vlan_name": "VLAN{{ .value }}",
		},
	}

	params, err := generateValueParamsFromSource(cfg, rule)
	if err != nil {
		t.Fatalf("generateValueParamsFromSource failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	// Check that param_format adds new keys
	// Note: current implementation does simple value assignment, not template expansion
	// This test documents the current behavior
	if _, ok := params[0]["vlan_id"]; !ok {
		t.Errorf("params[0] should have vlan_id key")
	}
	if _, ok := params[0]["vlan_name"]; !ok {
		t.Errorf("params[0] should have vlan_name key")
	}

	// Original keys should still exist
	if params[0]["value"] != "100" {
		t.Errorf("params[0][value] = %q, want %q", params[0]["value"], "100")
	}
	if params[0]["index"] != "0" {
		t.Errorf("params[0][index] = %q, want %q", params[0]["index"], "0")
	}
}

func TestGenerateValuesFromSource_FileWithFormat(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &types.Config{
		GlobalSettings: types.GlobalSettings{
			PathSpecification: "default",
		},
	}

	// Create a YAML file
	yamlFile := filepath.Join(tmpDir, "data.yaml")
	yamlContent := `- id: "100"
  name: test1
- id: "200"
  name: test2
`
	if err := os.WriteFile(yamlFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	rule := &types.ParameterRule{
		Name: "test_file",
		Mode: "attach",
		Source: &types.ParameterRuleSource{
			Type: "file",
			File: yamlFile,
		},
	}

	params, err := generateValueParamsFromSource(cfg, rule)
	if err != nil {
		t.Fatalf("generateValueParamsFromSource failed: %v", err)
	}

	if len(params) != 2 {
		t.Errorf("expected 2 params, got %d", len(params))
	}

	if params[0]["id"] != "100" {
		t.Errorf("params[0][id] = %q, want %q", params[0]["id"], "100")
	}
	if params[0]["name"] != "test1" {
		t.Errorf("params[0][name] = %q, want %q", params[0]["name"], "test1")
	}
}

func TestGenerateValuesFromSource_NoSource(t *testing.T) {
	cfg := &types.Config{}

	rule := &types.ParameterRule{
		Name:   "test_no_source",
		Mode:   "attach",
		Source: nil,
	}

	_, err := generateValueParamsFromSource(cfg, rule)
	if err == nil {
		t.Error("expected error for rule with no source, got nil")
	}
}

func TestGenerateValuesFromSource_UnknownType(t *testing.T) {
	cfg := &types.Config{}

	rule := &types.ParameterRule{
		Name: "test_unknown",
		Mode: "attach",
		Source: &types.ParameterRuleSource{
			Type: "unknown_type",
		},
	}

	_, err := generateValueParamsFromSource(cfg, rule)
	if err == nil {
		t.Error("expected error for unknown source type, got nil")
	}
}

// ============================================================
// Distribute Mode Tests (getParameterCandidates)
// ============================================================

func TestGetParameterCandidates_Integer(t *testing.T) {
	cfg := &types.Config{}

	tests := []struct {
		name     string
		rule     *types.ParameterRule
		cnt      int
		expected []string
	}{
		{
			name: "basic integer sequence",
			rule: &types.ParameterRule{
				Name:   "vlan_id",
				Type:   "integer",
				Min:    100,
				Max:    200,
				Header: "",
				Footer: "",
			},
			cnt:      3,
			expected: []string{"100", "101", "102"},
		},
		{
			name: "with header and footer",
			rule: &types.ParameterRule{
				Name:   "as_number",
				Type:   "integer",
				Min:    65001,
				Max:    65100,
				Header: "AS",
				Footer: "",
			},
			cnt:      2,
			expected: []string{"AS65001", "AS65002"},
		},
		{
			name: "default type (empty) treated as integer",
			rule: &types.ParameterRule{
				Name:   "id",
				Type:   "",
				Min:    1,
				Max:    10,
				Header: "ID-",
				Footer: "-END",
			},
			cnt:      2,
			expected: []string{"ID-1-END", "ID-2-END"},
		},
		{
			name: "single value",
			rule: &types.ParameterRule{
				Name:   "single",
				Type:   "integer",
				Min:    50,
				Max:    100,
				Header: "",
				Footer: "",
			},
			cnt:      1,
			expected: []string{"50"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := getParameterCandidates(cfg, tt.rule, tt.cnt)
			if err != nil {
				t.Fatalf("getParameterCandidates failed: %v", err)
			}

			if len(params) != len(tt.expected) {
				t.Errorf("expected %d params, got %d", len(tt.expected), len(params))
			}

			for i, exp := range tt.expected {
				if i < len(params) && params[i] != exp {
					t.Errorf("params[%d] = %q, want %q", i, params[i], exp)
				}
			}
		})
	}
}

func TestGetParameterCandidates_IntegerNotEnough(t *testing.T) {
	cfg := &types.Config{}

	rule := &types.ParameterRule{
		Name:   "limited",
		Type:   "integer",
		Min:    1,
		Max:    3,
		Header: "",
		Footer: "",
	}

	// Request more than available (max - min = 2, but requesting 5)
	_, err := getParameterCandidates(cfg, rule, 5)
	if err == nil {
		t.Error("expected error when requesting more params than available, got nil")
	}
}

func TestGetParameterCandidates_File(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "names.txt")
	content := "router-tokyo\nrouter-osaka\nrouter-nagoya\nrouter-fukuoka\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &types.Config{
		GlobalSettings: types.GlobalSettings{
			PathSpecification: "default",
		},
	}

	rule := &types.ParameterRule{
		Name:       "hostname",
		Type:       "file",
		SourceFile: tmpFile,
	}

	t.Run("read all values", func(t *testing.T) {
		params, err := getParameterCandidates(cfg, rule, 4)
		if err != nil {
			t.Fatalf("getParameterCandidates failed: %v", err)
		}

		expected := []string{"router-tokyo", "router-osaka", "router-nagoya", "router-fukuoka"}
		if len(params) != len(expected) {
			t.Errorf("expected %d params, got %d", len(expected), len(params))
		}

		for i, exp := range expected {
			if i < len(params) && params[i] != exp {
				t.Errorf("params[%d] = %q, want %q", i, params[i], exp)
			}
		}
	})

	t.Run("read partial values", func(t *testing.T) {
		params, err := getParameterCandidates(cfg, rule, 2)
		if err != nil {
			t.Fatalf("getParameterCandidates failed: %v", err)
		}

		if len(params) != 2 {
			t.Errorf("expected 2 params, got %d", len(params))
		}
		if params[0] != "router-tokyo" {
			t.Errorf("params[0] = %q, want %q", params[0], "router-tokyo")
		}
		if params[1] != "router-osaka" {
			t.Errorf("params[1] = %q, want %q", params[1], "router-osaka")
		}
	})
}

func TestGetParameterCandidates_FileWithEmptyLines(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "names.txt")
	content := "value1\n\nvalue2\n\n\nvalue3\n"
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &types.Config{
		GlobalSettings: types.GlobalSettings{
			PathSpecification: "default",
		},
	}

	rule := &types.ParameterRule{
		Name:       "values",
		Type:       "file",
		SourceFile: tmpFile,
	}

	params, err := getParameterCandidates(cfg, rule, 3)
	if err != nil {
		t.Fatalf("getParameterCandidates failed: %v", err)
	}

	// Empty lines should be skipped
	expected := []string{"value1", "value2", "value3"}
	if len(params) != len(expected) {
		t.Errorf("expected %d params, got %d", len(expected), len(params))
	}

	for i, exp := range expected {
		if i < len(params) && params[i] != exp {
			t.Errorf("params[%d] = %q, want %q", i, params[i], exp)
		}
	}
}
