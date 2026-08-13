package model

import (
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
