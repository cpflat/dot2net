package types

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTemp puts a file beside the config so that sourcefile: can name it.
func writeTemp(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func loadConfigFrom(t *testing.T, yaml string, files map[string]string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		writeTemp(t, dir, name, content)
	}
	path := filepath.Join(dir, "input.yaml")
	writeTemp(t, dir, "input.yaml", yaml)
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	// The templates are parsed here, not by LoadConfig, and that is where a
	// config entry is checked over.
	return LoadTemplates(cfg)
}

// TestRawHandsTheFileThrough is the case the feature exists for: a file that is
// material rather than a template, carrying {{ of its own that belongs to
// whoever reads it later.
func TestRawHandsTheFileThrough(t *testing.T) {
	const material = "bindip: '{{ip.r1.net0}}'\nname: {{ .name }}\n"
	cfg, err := loadConfigFrom(t, `
name: raw_test
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        sourcefile: ./material.yml
        raw: true
`, map[string]string{"material.yml": material})
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	ct := cfg.NodeClasses[0].ConfigTemplates[0]
	content, ok := ct.RawContent()
	if !ok {
		t.Fatal("a raw entry must report itself as one")
	}
	if content != material {
		t.Errorf("content was changed:\n%q\nwant\n%q", content, material)
	}
	// Note what is being pinned: {{ .name }} survives untouched even though
	// dot2net knows a parameter by that name. Expanding it is exactly the silent
	// substitution this avoids.
	if !strings.Contains(content, "{{ .name }}") {
		t.Error("a raw file must keep even the actions dot2net could have filled in")
	}
}

// TestRawWithoutSourceFileIsRejected: raw says "hand this file through", so
// without a file it is asking for nothing. Saying so beats ignoring it.
func TestRawWithoutSourceFileIsRejected(t *testing.T) {
	_, err := loadConfigFrom(t, `
name: raw_no_file
nodeclass:
  - name: router
    config:
      - file: out
        raw: true
        template:
          - "hostname {{ .name }}"
`, nil)
	if err == nil {
		t.Fatal("raw without sourcefile must be rejected")
	}
	if !strings.Contains(err.Error(), "delimiters") {
		t.Errorf("the message should point at what to use instead: %v", err)
	}
}

// TestTemplateAndSourceFileTogetherAreRejected pins the decision that a config
// entry is one thing or the other. Holding both left the order between them to
// be settled silently, and nothing used it.
func TestTemplateAndSourceFileTogetherAreRejected(t *testing.T) {
	_, err := loadConfigFrom(t, `
name: both
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        sourcefile: ./part.txt
        template:
          - "first"
`, map[string]string{"part.txt": "second\n"})
	if err == nil {
		t.Fatal("naming both template and sourcefile must be rejected")
	}
	if !strings.Contains(err.Error(), "two config entries") {
		t.Errorf("the message should say how to write it instead: %v", err)
	}
}

// TestDelimitersLetDownstreamSyntaxThrough covers generating a file that is
// itself a template for another tool.
func TestDelimitersLetDownstreamSyntaxThrough(t *testing.T) {
	cfg, err := loadConfigFrom(t, `
name: delims
nodeclass:
  - name: router
    config:
      - file: out
        delimiters: ["[[", "]]"]
        template:
          - "bindip: '{{ip.r1.net0}}'"
          - "name: [[ .name ]]"
`, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	ct := cfg.NodeClasses[0].ConfigTemplates[0]
	var sb strings.Builder
	if err := ct.ParsedTemplate.Execute(&sb, map[string]string{"name": "r1"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := sb.String()
	if !strings.Contains(got, "bindip: '{{ip.r1.net0}}'") {
		t.Errorf("the downstream tool's own syntax was not passed through: %q", got)
	}
	if !strings.Contains(got, "name: r1") {
		t.Errorf("dot2net's own value was not filled in: %q", got)
	}
}

func TestDelimitersMustBeTwo(t *testing.T) {
	_, err := loadConfigFrom(t, `
name: delims_bad
nodeclass:
  - name: router
    config:
      - file: out
        delimiters: ["[["]
        template:
          - "x"
`, nil)
	if err == nil {
		t.Fatal("a single delimiter must be rejected")
	}
	if !strings.Contains(err.Error(), "two") {
		t.Errorf("the message should say how many are expected: %v", err)
	}
}

func TestRawAndDelimitersTogetherAreRejected(t *testing.T) {
	_, err := loadConfigFrom(t, `
name: raw_delims
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        sourcefile: ./material.yml
        raw: true
        delimiters: ["[[", "]]"]
`, map[string]string{"material.yml": "x\n"})
	if err == nil {
		t.Fatal("raw together with delimiters must be rejected")
	}
}

// TestUnknownKeyIsRejected is the accident this guards against: example/
// address_reservation wrote management_layer where the key is mgmt_layer, and
// went a year with its management network quietly switched off.
func TestUnknownKeyIsRejected(t *testing.T) {
	_, err := loadConfigFrom(t, "name: typo\nglobal:\n  pathh: local\n", nil)
	if err == nil {
		t.Fatal("a key the config does not know must be rejected")
	}
	if !strings.Contains(err.Error(), "pathh") {
		t.Errorf("the message should name the offending key: %v", err)
	}
}

// TestDuplicateKeyIsRejected covers the other quiet loss: the later value wins
// and the earlier one is gone without a word.
func TestDuplicateKeyIsRejected(t *testing.T) {
	_, err := loadConfigFrom(t, "name: first\nname: second\n", nil)
	if err == nil {
		t.Fatal("a duplicate key must be rejected")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("the message should say what is wrong: %v", err)
	}
}

// TestUnknownKeyInModuleConfigIsRejected: a module's own settings get the same
// care as the rest of the file.
func TestUnknownKeyInModuleConfigIsRejected(t *testing.T) {
	cfg, err := loadConfigFrom(t, `
name: mc_typo
module:
  - containerlab
module_config:
  containerlab:
    managment_network: true
`, nil)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	var opts struct {
		ManagementNetwork bool `yaml:"management_network"`
	}
	if _, err := cfg.DecodeModuleConfig("containerlab", &opts); err == nil {
		t.Fatal("a misspelled key inside module_config must be rejected")
	}
}
