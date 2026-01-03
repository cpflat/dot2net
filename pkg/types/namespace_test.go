package types

import (
	"strings"
	"testing"
)

func TestReservedPrefixes(t *testing.T) {
	prefixes := ReservedPrefixes()

	// Verify expected prefixes are included
	expectedPrefixes := []string{
		NumberPrefixNode,
		NumberPrefixConnection,
		NumberPrefixGroup,
		NumberPrefixOppositeInterface,
		NumberPrefixNeighbor,
		NumberPrefixMember,
		SelfConfigHeader,
		ValueReferencePrefix,
	}

	for _, expected := range expectedPrefixes {
		found := false
		for _, p := range prefixes {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ReservedPrefixes() should include %q", expected)
		}
	}

	// Verify all prefixes end with underscore (separator)
	for _, p := range prefixes {
		if !strings.HasSuffix(p, NumberSeparator) {
			t.Errorf("Reserved prefix %q should end with separator %q", p, NumberSeparator)
		}
	}
}

func TestReservedNames(t *testing.T) {
	names := ReservedNames()

	// Verify "name" is reserved
	found := false
	for _, n := range names {
		if n == ReservedParamName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ReservedNames() should include %q", ReservedParamName)
	}
}

func TestCheckReservedParamName(t *testing.T) {
	tests := []struct {
		name            string
		paramName       string
		wantError       bool
		wantContains    []string // strings that should appear in error message
		wantNotContains []string // strings that should not appear
	}{
		// Reserved names
		{
			name:         "reserved name 'name'",
			paramName:    "name",
			wantError:    true,
			wantContains: []string{"'name'", "reserved name", "object names", "choose a different name"},
		},
		// Reserved prefixes
		{
			name:         "reserved prefix node_",
			paramName:    "node_id",
			wantError:    true,
			wantContains: []string{"'node_id'", NumberPrefixNode, "cross-object", "please rename"},
		},
		{
			name:         "reserved prefix conn_",
			paramName:    "conn_id",
			wantError:    true,
			wantContains: []string{"'conn_id'", NumberPrefixConnection, "connection_id"}, // should suggest connection_id
		},
		{
			name:         "reserved prefix values_",
			paramName:    "values_test",
			wantError:    true,
			wantContains: []string{"'values_test'", ValueReferencePrefix, "Value class"},
		},
		{
			name:         "reserved prefix interfaces_",
			paramName:    "interfaces_custom",
			wantError:    true,
			wantContains: []string{"'interfaces_custom'", ChildInterfacesConfigHeader, "child interfaces"},
		},
		{
			name:         "reserved prefix self_",
			paramName:    "self_config",
			wantError:    true,
			wantContains: []string{"'self_config'", SelfConfigHeader, "interface config"},
		},
		// Valid names
		{
			name:      "valid simple name",
			paramName: "vlan_id",
			wantError: false,
		},
		{
			name:      "valid name with underscore",
			paramName: "connection_id",
			wantError: false,
		},
		{
			name:      "valid name similar to reserved prefix",
			paramName: "nodename", // doesn't have underscore after "node"
			wantError: false,
		},
		{
			name:      "valid name 'names'",
			paramName: "names", // similar to reserved "name" but different
			wantError: false,
		},
		{
			name:      "valid name 'my_name'",
			paramName: "my_name",
			wantError: false,
		},
		{
			name:      "empty name",
			paramName: "",
			wantError: false, // empty is not reserved (validation happens elsewhere)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := CheckReservedParamName(tt.paramName)
			gotError := msg != ""

			if gotError != tt.wantError {
				if tt.wantError {
					t.Errorf("CheckReservedParamName(%q) = %q, want error", tt.paramName, msg)
				} else {
					t.Errorf("CheckReservedParamName(%q) = %q, want no error", tt.paramName, msg)
				}
				return
			}

			// Verify expected content in error message
			for _, want := range tt.wantContains {
				if !strings.Contains(msg, want) {
					t.Errorf("CheckReservedParamName(%q) = %q, should contain %q",
						tt.paramName, msg, want)
				}
			}
			for _, notWant := range tt.wantNotContains {
				if strings.Contains(msg, notWant) {
					t.Errorf("CheckReservedParamName(%q) = %q, should not contain %q",
						tt.paramName, msg, notWant)
				}
			}
		})
	}
}

func TestCheckReservedParamName_AllPrefixes(t *testing.T) {
	// Test that all reserved prefixes are actually checked
	for _, prefix := range ReservedPrefixes() {
		paramName := prefix + "test"
		msg := CheckReservedParamName(paramName)
		if msg == "" {
			t.Errorf("CheckReservedParamName(%q) should return error for reserved prefix %q",
				paramName, prefix)
		}
		// Verify prefix is mentioned
		if !strings.Contains(msg, prefix) {
			t.Errorf("CheckReservedParamName(%q) = %q, should mention prefix %q",
				paramName, msg, prefix)
		}
		// Verify suggestion is provided
		if !strings.Contains(msg, "please rename") {
			t.Errorf("CheckReservedParamName(%q) = %q, should provide rename suggestion",
				paramName, msg)
		}
	}
}

func TestCheckReservedParamName_AllNames(t *testing.T) {
	// Test that all reserved names are actually checked
	for _, name := range ReservedNames() {
		msg := CheckReservedParamName(name)
		if msg == "" {
			t.Errorf("CheckReservedParamName(%q) should return error for reserved name", name)
		}
		// Verify explanation is provided
		if !strings.Contains(msg, "reserved name") {
			t.Errorf("CheckReservedParamName(%q) = %q, should explain it's a reserved name",
				name, msg)
		}
	}
}

func TestDescribeReservedPrefix(t *testing.T) {
	// Verify all reserved prefixes have descriptions
	for _, prefix := range ReservedPrefixes() {
		desc := describeReservedPrefix(prefix)
		if desc == "" {
			t.Errorf("describeReservedPrefix(%q) should return non-empty description", prefix)
		}
		if desc == "reserved for internal use" {
			t.Errorf("describeReservedPrefix(%q) should have a specific description, not generic fallback", prefix)
		}
	}
}

func TestSuggestAlternativeName(t *testing.T) {
	tests := []struct {
		paramName string
		prefix    string
		want      string
	}{
		{"conn_id", NumberPrefixConnection, "connection_id"},
		{"conn_", NumberPrefixConnection, "connection_param"},
		{"node_id", NumberPrefixNode, "node_id"},
		{"values_test", ValueReferencePrefix, "my_values_test"},
	}

	for _, tt := range tests {
		t.Run(tt.paramName, func(t *testing.T) {
			got := suggestAlternativeName(tt.paramName, tt.prefix)
			if got != tt.want {
				t.Errorf("suggestAlternativeName(%q, %q) = %q, want %q",
					tt.paramName, tt.prefix, got, tt.want)
			}
		})
	}
}
