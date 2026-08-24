package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// The ordering among one object's config templates is worked out from what each
// one reads, rather than from depends:. These cover the shapes the bundled
// topologies do not have: a chain, a reference that only appears in blocks:, one
// that comes from a source file or through changed delimiters, one that names
// nothing, and one that points at another object and must not count.
//
// Every case is written so that the declaration order is wrong on purpose: the
// template doing the reading comes first, so nothing but the reference can put
// the two in order.

// buildOut writes the yaml with an out file and returns what r1 got.
func buildOut(t *testing.T, yaml string, files map[string]string) string {
	t.Helper()
	cfgPath, dotPath := writeTempInput(t, yaml, hookDot)
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(filepath.Dir(cfgPath), name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return generateFor(t, cfg, nm, "r1", "out")
}

func TestDerivedOrderFollowsAChain(t *testing.T) {
	out := buildOut(t, `
name: chain
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        template: ["[{{ .self_middle }}]"]
      - name: middle
        template: ["<{{ .self_inner }}>"]
      - name: inner
        template: ["core"]
file:
  - name: out
`, nil)
	if !strings.Contains(out, "[<core>]") {
		t.Errorf("each link of the chain has to be rendered before the one that reads it, got:\n%s", out)
	}
}

func TestDerivedOrderFromBlocksAlone(t *testing.T) {
	out := buildOut(t, `
name: blocks_only
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        template: []
        blocks:
          after: [self_inner]
      - name: inner
        template: ["core"]
file:
  - name: out
`, nil)
	if !strings.Contains(out, "core") {
		t.Errorf("a block merged with blocks: is read the same as one embedded, got:\n%s", out)
	}
}

func TestDerivedOrderFromASourceFile(t *testing.T) {
	out := buildOut(t, `
name: sourcefile_ref
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        sourcefile: ./body.tmpl
      - name: inner
        template: ["core"]
file:
  - name: out
`, map[string]string{"body.tmpl": "[{{ .self_inner }}]"})
	if !strings.Contains(out, "[core]") {
		t.Errorf("a reference read from a source file counts too, got:\n%s", out)
	}
}

func TestDerivedOrderThroughChangedDelimiters(t *testing.T) {
	out := buildOut(t, `
name: delims_ref
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        delimiters: ["[[", "]]"]
        template: ["<[[ .self_inner ]]>"]
      - name: inner
        template: ["core"]
file:
  - name: out
`, nil)
	if !strings.Contains(out, "<core>") {
		t.Errorf("the marks around an action do not hide it, got:\n%s", out)
	}
}

// A reference inside a branch counts as well: which way the branch goes is
// decided while rendering, and by then the order has been settled.
func TestDerivedOrderFromInsideABranch(t *testing.T) {
	out := buildOut(t, `
name: branch_ref
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        template: ["<{{ if .name }}{{ .self_inner }}{{ end }}>"]
      - name: inner
        template: ["core"]
file:
  - name: out
`, nil)
	if !strings.Contains(out, "<core>") {
		t.Errorf("a reference in a branch still has to be rendered first, got:\n%s", out)
	}
}

// A reference to another object is not an order among one object's templates -
// the other object is generated on its own - so it must not become one. Here it
// would be a cycle if it did.
func TestReferenceToAnotherObjectIsNotADependency(t *testing.T) {
	out := buildOut(t, `
name: cross_object
global:
  path: local
class_policy:
  interface:
    default: [default]
nodeclass:
  - name: router
    interface_policy: [ip]
    config:
      - file: out
        template: ["[{{ .interfaces_ifconf }}]"]
interfaceclass:
  - name: default
    config:
      - name: ifconf
        template: ["iface {{ .name }}"]
layer:
  - name: ip
    default_connect: true
    policy:
      - name: ip
        range: 10.0.0.0/16
        prefix: 24
file:
  - name: out
`, nil)
	if !strings.Contains(out, "iface eth0") {
		t.Errorf("the interface's block still reaches the node, got:\n%s", out)
	}
}

// A name nothing carries is left alone: the reference renders as nothing, which
// is what it did before the order was worked out from the text. Reporting it
// belongs with the other unresolved-name checks, not here.
func TestReferenceToNothingIsNotAnError(t *testing.T) {
	out := buildOut(t, `
name: missing_ref
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        template: ["[{{ .self_inner }}]"]
      - name: inner
        template: []
file:
  - name: out
`, nil)
	if !strings.Contains(out, "[]") {
		t.Errorf("an empty block renders as nothing, got:\n%s", out)
	}
}

// Two templates that read each other have no order at all, and the report has
// to name them - the reference is the only place the cycle is written, so the
// author has nothing else to go on.
func TestTemplatesThatReadEachOtherAreReported(t *testing.T) {
	cfgPath, dotPath := writeTempInput(t, `
name: ref_cycle
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        name: first
        template: ["{{ .self_second }}"]
      - name: second
        template: ["{{ .self_first }}"]
file:
  - name: out
`, hookDot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	err = buildConfigFilesErr(t, cfg, nm)
	if err == nil {
		t.Fatal("two templates reading each other must be reported")
	}
	if !strings.Contains(err.Error(), "first") || !strings.Contains(err.Error(), "second") {
		t.Errorf("the message should name both, got: %v", err)
	}
}
