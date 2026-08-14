package example_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TutorialDirName is the demonstration kit: a self-contained copy of a topology
// plus a much larger real-world graph to swap in, so that the whole thing can be
// carried to a machine as one directory.
const TutorialDirName string = "tutorial"

// TutorialSourceTopology is the topology tutorial/ holds a copy of.
const TutorialSourceTopology string = "ospf_simple"

// TestTutorialMatchesItsSource guards a copy, which is the thing that goes
// stale. tutorial/ carries its own input files rather than pointing at
// topologies/ospf_simple, because the point of it is to be one directory you can
// put on a machine. That copy has fallen behind before - it missed `raw: true`
// and the kathara module for a whole release - and neither the golden tests nor
// a build would have said so, because the copy still built and still produced
// the same bytes it always had.
//
// So the copy is checked instead: identical but for the lab's name. A change to
// ospf_simple now fails here until tutorial/ is brought along.
func TestTutorialMatchesItsSource(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	rootDir := filepath.Join(wd, "..", "..")
	tutorialDir := filepath.Join(rootDir, TutorialDirName)
	sourceDir := findTopologyDir(t, rootDir, TutorialSourceTopology)

	// Everything dot2net reads. tutorial/ has more beside these - its readme,
	// the rendered PDFs, the larger graph - and those are its own.
	for _, name := range []string{TopologyFileName, "daemons", "vtysh.conf"} {
		assertSameFile(t, filepath.Join(tutorialDir, name), filepath.Join(sourceDir, name), "")
	}

	// The lab's name is the one thing that differs, and it differs on purpose:
	// the tutorial deploys as "tutorial".
	assertSameFile(t,
		filepath.Join(tutorialDir, DefinitionFileName),
		filepath.Join(sourceDir, DefinitionFileName),
		"name: ")
}

// assertSameFile compares two files line by line. Lines starting with
// exceptPrefix are allowed to differ; every other difference is reported with
// its line number, so that what drifted is named rather than left to a diff.
func assertSameFile(t *testing.T, gotPath, wantPath, exceptPrefix string) {
	t.Helper()

	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", gotPath, err)
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", wantPath, err)
	}

	gotLines := strings.Split(string(got), "\n")
	wantLines := strings.Split(string(want), "\n")

	if len(gotLines) != len(wantLines) {
		t.Errorf("%s has %d lines and %s has %d: the copy has drifted from its source",
			gotPath, len(gotLines), wantPath, len(wantLines))
		return
	}

	for i := range gotLines {
		if gotLines[i] == wantLines[i] {
			continue
		}
		if exceptPrefix != "" &&
			strings.HasPrefix(gotLines[i], exceptPrefix) &&
			strings.HasPrefix(wantLines[i], exceptPrefix) {
			continue
		}
		t.Errorf("%s:%d has drifted from %s\n  it has:     %q\n  source has: %q",
			gotPath, i+1, wantPath, gotLines[i], wantLines[i])
	}
}
