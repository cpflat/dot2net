package model

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// buildForOutputFiles builds the skeleton, classifies it, and checks where the
// files land. Classification has to run: which files an object writes follows
// from its classes.
func buildForOutputFiles(t *testing.T, yaml, dot string) error {
	t.Helper()
	cfgPath, dotPath := writeTempInput(t, yaml, dot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := buildSkeleton(cfg, d)
	if err != nil {
		t.Fatalf("buildSkeleton: %v", err)
	}
	if err := checkClasses(cfg, nm); err != nil {
		t.Fatalf("checkClasses: %v", err)
	}
	return checkOutputFilesUnique(cfg, nm)
}

const twoNodes = `graph {
  r1 [class="all"];
  r2 [class="all"];
  r1 -- r2;
}`

// TestSameOutputPathIsRejected is the case this exists for: two definitions
// whose names differ but whose files do not. The later one used to overwrite
// the earlier without a word, which is how a topology writing its own
// <node>.startup silently lost it to the Kathara module's.
func TestSameOutputPathIsRejected(t *testing.T) {
	err := buildForOutputFiles(t, `
name: same_path
global:
  path: local
file:
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root
  - name: other_startup
    name_suffix: ".startup"
    scope: node
    output: root
nodeclass:
  - name: all
    config:
      - file: startup
        template: ["from startup"]
      - file: other_startup
        template: ["from other_startup"]
`, twoNodes)
	if err == nil {
		t.Fatal("two file definitions writing one file must be rejected")
	}
	// The message has to name both definitions and the file, or the author has
	// to go looking for which pair collided.
	for _, want := range []string{"startup", "other_startup", "r1.startup"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %v", want, err)
		}
	}
}

// TestDifferentOutputPathsAreAccepted keeps the check from firing on the ordinary
// case of one definition writing one file per node.
func TestDifferentOutputPathsAreAccepted(t *testing.T) {
	err := buildForOutputFiles(t, `
name: distinct_paths
global:
  path: local
file:
  - name: startup
    name_suffix: ".startup"
    scope: node
    output: root
  - name: conf
    name_suffix: ".conf"
    scope: node
    output: root
nodeclass:
  - name: all
    config:
      - file: startup
        template: ["a"]
      - file: conf
        template: ["b"]
`, twoNodes)
	if err != nil {
		t.Fatalf("distinct files must be accepted: %v", err)
	}
}

// TestListedPathsUseTheLocalSeparator pins the one thing Windows disagreed
// with: a node's file is named for two audiences, and the list is the local
// one. OutputPath is built with forward slashes because a bind line in a
// platform file names it and that line is read on the machine the lab runs on.
// What this list names is a file on this machine, so it has to be in this
// machine's terms - and a test that normalised both sides hid the difference
// instead of catching it.
func TestListedPathsUseTheLocalSeparator(t *testing.T) {
	const yaml = `
name: sep
module: []
class_policy:
  interface:
    default: [default]
file:
  - name: frr.conf
    path: /etc/frr/frr.conf
nodeclass:
  - name: router
    config:
      - file: frr.conf
        template: ["!"]
interfaceclass:
  - name: default
`
	const dot = `digraph { r1 [xlabel="router"]; r2 [xlabel="router"]; r1 -> r2 [dir="none"]; }`

	cfgPath, dotPath := writeTempInput(t, yaml, dot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	cfg, err = types.LoadTemplates(cfg)
	if err != nil {
		t.Fatalf("LoadTemplates: %v", err)
	}
	nm, err := BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("BuildNetworkModel: %v", err)
	}
	files, err := ListGeneratedFiles(cfg, nm, false)
	if err != nil {
		t.Fatalf("ListGeneratedFiles: %v", err)
	}

	want := filepath.Join("r1", "etc", "frr", "frr.conf")
	var found bool
	for _, f := range files {
		if f == want {
			found = true
		}
		if strings.ContainsRune(f, '/') && filepath.Separator != '/' {
			t.Errorf("%q is listed with a foreign separator", f)
		}
	}
	if !found {
		t.Errorf("%q not listed; got %v", want, files)
	}
}
