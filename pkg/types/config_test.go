package types

import "testing"

func TestFileDefinition_GetFileName(t *testing.T) {
	tests := []struct {
		name       string
		fileDef    FileDefinition
		objectName string
		want       string
	}{
		{
			name: "Name only (traditional)",
			fileDef: FileDefinition{
				Name: "frr.conf",
			},
			objectName: "r1",
			want:       "frr.conf",
		},
		{
			name: "Prefix/Suffix overrides Name",
			fileDef: FileDefinition{
				Name:       "startup", // Used as ID for referencing
				NamePrefix: "pre_",
				NameSuffix: ".suffix",
			},
			objectName: "r1",
			want:       "pre_r1.suffix",
		},
		{
			name: "Suffix only (Kathara-style startup)",
			fileDef: FileDefinition{
				Name:       "startup", // Used as ID
				NameSuffix: ".startup",
			},
			objectName: "r1",
			want:       "r1.startup",
		},
		{
			name: "Prefix only",
			fileDef: FileDefinition{
				Name:       "config",
				NamePrefix: "config_",
			},
			objectName: "r1",
			want:       "config_r1",
		},
		{
			name: "Both prefix and suffix",
			fileDef: FileDefinition{
				Name:       "script",
				NamePrefix: "startup_",
				NameSuffix: ".sh",
			},
			objectName: "r1",
			want:       "startup_r1.sh",
		},
		{
			name: "Empty object name (network scope)",
			fileDef: FileDefinition{
				Name: "topo.yaml",
			},
			objectName: "",
			want:       "topo.yaml",
		},
		{
			name: "No name or prefix/suffix returns Name",
			fileDef: FileDefinition{
				Name: "default.txt",
			},
			objectName: "r1",
			want:       "default.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fileDef.GetFileName(tt.objectName)
			if got != tt.want {
				t.Errorf("GetFileName(%q) = %q, want %q", tt.objectName, got, tt.want)
			}
		})
	}
}

func TestFileDefinition_GetOutputLocation(t *testing.T) {
	tests := []struct {
		name    string
		fileDef FileDefinition
		want    string
	}{
		{
			name: "Output explicitly set to root",
			fileDef: FileDefinition{
				Output: "root",
			},
			want: "root",
		},
		{
			name: "Output explicitly set to node",
			fileDef: FileDefinition{
				Output: "node",
			},
			want: "node",
		},
		{
			name: "Output empty, Scope network -> root",
			fileDef: FileDefinition{
				Scope: ClassTypeNetwork,
			},
			want: "root",
		},
		{
			name: "Output empty, Scope node -> node",
			fileDef: FileDefinition{
				Scope: ClassTypeNode,
			},
			want: "node",
		},
		{
			name: "Both Output and Scope empty -> node (default)",
			fileDef: FileDefinition{},
			want:    "node",
		},
		{
			name: "Output root overrides Scope node",
			fileDef: FileDefinition{
				Scope:  ClassTypeNode,
				Output: "root",
			},
			want: "root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fileDef.GetOutputLocation()
			if got != tt.want {
				t.Errorf("GetOutputLocation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigTemplate_HasRequiredParams(t *testing.T) {
	tests := []struct {
		name           string
		requiredParams []string
		params         map[string]string
		want           bool
	}{
		{
			name:           "No required params - always true",
			requiredParams: nil,
			params:         map[string]string{},
			want:           true,
		},
		{
			name:           "Empty required params - always true",
			requiredParams: []string{},
			params:         map[string]string{},
			want:           true,
		},
		{
			name:           "Single required param exists",
			requiredParams: []string{"mem"},
			params:         map[string]string{"mem": "256m", "image": "frr"},
			want:           true,
		},
		{
			name:           "Single required param missing",
			requiredParams: []string{"mem"},
			params:         map[string]string{"image": "frr"},
			want:           false,
		},
		{
			name:           "Multiple required params all exist",
			requiredParams: []string{"mem", "cpus"},
			params:         map[string]string{"mem": "256m", "cpus": "0.5", "image": "frr"},
			want:           true,
		},
		{
			name:           "Multiple required params one missing",
			requiredParams: []string{"mem", "cpus"},
			params:         map[string]string{"mem": "256m", "image": "frr"},
			want:           false,
		},
		{
			name:           "Multiple required params all missing",
			requiredParams: []string{"mem", "cpus"},
			params:         map[string]string{"image": "frr"},
			want:           false,
		},
		{
			name:           "Required param exists with empty value",
			requiredParams: []string{"mem"},
			params:         map[string]string{"mem": ""},
			want:           false, // Empty value is treated as "not exists"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ct := &ConfigTemplate{
				RequiredParams: tt.requiredParams,
			}
			got := ct.HasRequiredParams(tt.params)
			if got != tt.want {
				t.Errorf("HasRequiredParams() = %v, want %v", got, tt.want)
			}
		})
	}
}
