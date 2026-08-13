package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

// topologyRoots are the directories a dot2net topology can live in: topologies/
// holds the ones worth deploying, example/ the ones that demonstrate a notation.
var topologyRoots = []string{"topologies", "example"}

// findTopologyDir returns the directory of the named topology, whichever root it
// sits under.
func findTopologyDir(t *testing.T, rootDir string, topology string) string {
	t.Helper()
	for _, root := range topologyRoots {
		dir := filepath.Join(rootDir, root, topology)
		if _, err := os.Stat(filepath.Join(dir, "input.dot")); err == nil {
			return dir
		}
	}
	t.Fatalf("topology %q not found under any of %v", topology, topologyRoots)
	return ""
}

// listGeneratedFiles computes the set of files that a build would generate for
// the topology in the current working directory, using the same model pipeline
// as CmdClean/CmdFiles.
func listGeneratedFiles(t *testing.T, dotFile, cfgFile string) []string {
	t.Helper()
	d, err := model.DiagramFromDotFile(dotFile)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	cfg, err := types.LoadConfig(cfgFile)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	nm, err := model.BuildNetworkModelForFileList(cfg, d)
	if err != nil {
		t.Fatalf("BuildNetworkModelForFileList: %v", err)
	}
	files, err := model.ListGeneratedFiles(cfg, nm, false)
	if err != nil {
		t.Fatalf("ListGeneratedFiles: %v", err)
	}
	return files
}

// TestCmdClean_DeletesOnlyGeneratedFiles is the safety test called out by
// CR-077: `clean` must delete only the files a build would generate (and only
// directories it emptied), never user-authored files. It runs the real
// commandBuild/commandClean CLI actions end-to-end in a temp dir.
func TestCmdClean_DeletesOnlyGeneratedFiles(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	topologyDir := findTopologyDir(t, wd, "ospf_simple")

	tmpDir := t.TempDir()
	// copy top-level input files
	entries, err := os.ReadDir(topologyDir)
	if err != nil {
		t.Fatalf("ReadDir topology: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(topologyDir, e.Name()))
		if err != nil {
			t.Fatalf("read input %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, e.Name()), data, 0644); err != nil {
			t.Fatalf("write input %s: %v", e.Name(), err)
		}
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(wd)

	const dotFile = "input.dot"
	const cfgFile = "input.yaml"

	app := &cli.App{
		Writer:    io.Discard,
		ErrWriter: io.Discard,
		Commands:  []*cli.Command{commandBuild, commandClean},
	}

	// 1. build -> generate files
	if err := app.Run([]string{"dot2net", "build", "-c", cfgFile, dotFile}); err != nil {
		t.Fatalf("build run: %v", err)
	}

	generated := listGeneratedFiles(t, dotFile, cfgFile)
	if len(generated) == 0 {
		t.Fatalf("topology generated no files")
	}
	// sanity: generated files actually exist on disk after build
	for _, f := range generated {
		if _, err := os.Stat(filepath.FromSlash(f)); err != nil {
			t.Fatalf("expected generated file %s to exist after build: %v", f, err)
		}
	}

	// Identify a directory that holds generated files, to (a) drop a user file
	// inside it and confirm the directory (and the user file) survive clean,
	// and (b) find another generated directory that should be emptied+removed.
	dirsWithGenerated := map[string]bool{}
	for _, f := range generated {
		if dir := filepath.Dir(f); dir != "." && dir != "" {
			dirsWithGenerated[dir] = true
		}
	}
	if len(dirsWithGenerated) < 2 {
		t.Fatalf("topology needs >=2 generated subdirectories, got %v", dirsWithGenerated)
	}
	var keepDir, emptyDir string
	for d := range dirsWithGenerated {
		if keepDir == "" {
			keepDir = d
		} else if emptyDir == "" && d != keepDir {
			emptyDir = d
		}
	}

	// 2. add user-authored files: one at top level, one inside a generated dir
	topUserFile := "user_notes.txt"
	if err := os.WriteFile(topUserFile, []byte("keep me"), 0644); err != nil {
		t.Fatalf("write user file: %v", err)
	}
	nestedUserFile := filepath.Join(keepDir, "user_keep.conf")
	if err := os.WriteFile(nestedUserFile, []byte("keep me too"), 0644); err != nil {
		t.Fatalf("write nested user file: %v", err)
	}

	// 3. clean
	if err := app.Run([]string{"dot2net", "clean", "-c", cfgFile, dotFile}); err != nil {
		t.Fatalf("clean run: %v", err)
	}

	// 4a. every generated file must be gone
	for _, f := range generated {
		if _, err := os.Stat(filepath.FromSlash(f)); err == nil {
			t.Errorf("generated file %s should have been deleted", f)
		}
	}
	// 4b. user files must survive
	if _, err := os.Stat(topUserFile); err != nil {
		t.Errorf("top-level user file was deleted: %v", err)
	}
	if _, err := os.Stat(nestedUserFile); err != nil {
		t.Errorf("nested user file was deleted: %v", err)
	}
	// 4c. input files must survive
	for _, f := range []string{dotFile, cfgFile} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("input file %s was deleted: %v", f, err)
		}
	}
	// 4d. a directory still holding a user file must NOT be removed
	if _, err := os.Stat(keepDir); err != nil {
		t.Errorf("directory %s with a surviving user file was removed: %v", keepDir, err)
	}
	// 4e. a directory that only held generated files must be removed (empty)
	if _, err := os.Stat(emptyDir); err == nil {
		t.Errorf("emptied generated directory %s should have been removed", emptyDir)
	}
}

// copyTopologyInputs copies a topology's top-level files into a fresh temp dir
// and chdirs into it, returning to the original directory when the test ends.
func copyTopologyInputs(t *testing.T, topology string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	topologyDir := findTopologyDir(t, wd, topology)
	tmpDir := t.TempDir()
	entries, err := os.ReadDir(topologyDir)
	if err != nil {
		t.Fatalf("ReadDir topology: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(topologyDir, e.Name()))
		if err != nil {
			t.Fatalf("read input %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, e.Name()), data, 0644); err != nil {
			t.Fatalf("write input %s: %v", e.Name(), err)
		}
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })
}

// TestCmdFiles_ListsPerMachineTopologyFiles guards the file-list pipeline against
// missing what a module hands out during classification. A topology file scoped
// to a worker group is assigned there, not when the module is loaded, so a
// pipeline that stops before classification reports a short list - and `clean`,
// which deletes exactly that list, used to leave every per-machine topo.yaml
// behind. Single-machine topologies cannot catch this: their topology file is
// network-scoped and registered at load time.
func TestCmdFiles_ListsPerMachineTopologyFiles(t *testing.T) {
	copyTopologyInputs(t, "ospf_multihost")

	const dotFile = "input.dot"
	const cfgFile = "input.yaml"

	app := &cli.App{Writer: io.Discard, ErrWriter: io.Discard, Commands: []*cli.Command{commandBuild, commandClean}}
	if err := app.Run([]string{"dot2net", "build", "-c", cfgFile, dotFile}); err != nil {
		t.Fatalf("build run: %v", err)
	}

	listed := map[string]bool{}
	for _, f := range listGeneratedFiles(t, dotFile, cfgFile) {
		listed[f] = true
	}
	for _, want := range []string{"host1/topo.yaml", "host2/topo.yaml"} {
		if _, err := os.Stat(filepath.FromSlash(want)); err != nil {
			t.Fatalf("the build did not write %s: %v", want, err)
		}
		if !listed[want] {
			t.Errorf("%s was generated but is missing from the file list", want)
		}
	}

	if err := app.Run([]string{"dot2net", "clean", "-c", cfgFile, dotFile}); err != nil {
		t.Fatalf("clean run: %v", err)
	}
	for _, gone := range []string{"host1/topo.yaml", "host2/topo.yaml"} {
		if _, err := os.Stat(filepath.FromSlash(gone)); err == nil {
			t.Errorf("clean left %s behind", gone)
		}
	}
}

// TestCmdClean_DryRun verifies that --dry-run reports without deleting.
func TestCmdClean_DryRun(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	topologyDir := findTopologyDir(t, wd, "ospf_simple")

	tmpDir := t.TempDir()
	entries, err := os.ReadDir(topologyDir)
	if err != nil {
		t.Fatalf("ReadDir topology: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(topologyDir, e.Name()))
		if err != nil {
			t.Fatalf("read input: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tmpDir, e.Name()), data, 0644); err != nil {
			t.Fatalf("write input: %v", err)
		}
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(wd)

	const dotFile = "input.dot"
	const cfgFile = "input.yaml"

	// capture stdout to confirm the dry-run report mentions "Would delete"
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	app := &cli.App{Writer: io.Discard, ErrWriter: io.Discard, Commands: []*cli.Command{commandBuild, commandClean}}
	if err := app.Run([]string{"dot2net", "build", "-c", cfgFile, dotFile}); err != nil {
		os.Stdout = oldStdout
		t.Fatalf("build run: %v", err)
	}
	generated := listGeneratedFiles(t, dotFile, cfgFile)

	runErr := app.Run([]string{"dot2net", "clean", "--dry-run", "-c", cfgFile, dotFile})

	w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	if runErr != nil {
		t.Fatalf("clean --dry-run run: %v", runErr)
	}

	// dry-run must not delete anything
	for _, f := range generated {
		if _, err := os.Stat(filepath.FromSlash(f)); err != nil {
			t.Errorf("dry-run deleted generated file %s: %v", f, err)
		}
	}
	if !strings.Contains(string(out), "Would delete") {
		t.Errorf("dry-run output lacks 'Would delete':\n%s", out)
	}
}
