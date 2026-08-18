package example_test

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

const TopologyFileName string = "input.dot"
const DefinitionFileName string = "input.yaml"
const GoldenDirName string = "expected"

// TopologyRoots are the directories a dot2net topology can live in: topologies/
// holds the ones worth deploying, example/ the ones that demonstrate a notation.
// Both are golden-tested, so the tests walk them the same way.
var TopologyRoots = []string{"topologies", "example"}

// findTopologyDir returns the directory of the named topology, whichever root it
// sits under. It fails the test rather than returning an error, since a name a
// test asks for by hand and cannot find is a broken test, not a broken topology.
func findTopologyDir(t *testing.T, rootDir string, topologyName string) string {
	t.Helper()
	for _, root := range TopologyRoots {
		dir := filepath.Join(rootDir, root, topologyName)
		if _, err := os.Stat(filepath.Join(dir, TopologyFileName)); err == nil {
			return dir
		}
	}
	t.Fatalf("topology %q not found under any of %v", topologyName, TopologyRoots)
	return ""
}

func TestExampleTopologies(t *testing.T) {
	// Get absolute path to project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	rootDir := filepath.Join(wd, "..", "..") // project root

	// Find all topologies with input.dot and input.yaml
	var topologies []string
	for _, root := range TopologyRoots {
		dir := filepath.Join(rootDir, root)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read directory %s: %v", dir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			topologyPath := filepath.Join(dir, entry.Name())
			dotFile := filepath.Join(topologyPath, TopologyFileName)
			yamlFile := filepath.Join(topologyPath, DefinitionFileName)

			if _, err := os.Stat(dotFile); err == nil {
				if _, err := os.Stat(yamlFile); err == nil {
					topologies = append(topologies, entry.Name())
				}
			}
		}
	}

	if len(topologies) == 0 {
		t.Fatalf("no valid topologies found under %v", TopologyRoots)
	}

	t.Logf("Found %d topologies: %v", len(topologies), topologies)

	for _, topologyName := range topologies {
		t.Run(topologyName, func(t *testing.T) {
			tryTopology(t, rootDir, topologyName)
		})
	}
}

func tryTopology(t *testing.T, rootDir string, topologyName string) {
	topologyDir := findTopologyDir(t, rootDir, topologyName)

	// create tmp dir
	tmpDir, err := os.MkdirTemp("", "dot2net_test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// copy input files into tmp dir
	err = filepath.WalkDir(topologyDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// skip subdirectories
		if d.IsDir() && path != topologyDir {
			return filepath.SkipDir
		}

		// copy only files
		if !d.IsDir() {
			relPath, err := filepath.Rel(topologyDir, path)
			if err != nil {
				return err
			}
			dst := filepath.Join(tmpDir, relPath)
			copyFile(t, path, dst)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to copy input files: %v", err)
	}

	topoFile := filepath.Join(tmpDir, TopologyFileName)
	defFile := filepath.Join(tmpDir, DefinitionFileName)
	// copyFile(t, filepath.Join(topologyDir, TopologyFileName), topoFile)
	// copyFile(t, filepath.Join(topologyDir, DefinitionFileName), defFile)

	// execute dot2net
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWd); err != nil {
			t.Errorf("failed to restore working directory: %v", err)
		}
	}()

	d, err := model.DiagramFromDotFile(topoFile)
	if err != nil {
		t.Fatalf("dot2net failed: %v\n", err)
	}

	cfg, err := types.LoadConfig(defFile)
	if err != nil {
		t.Fatalf("dot2net failed: %v\n", err)
	}

	nm, err := model.BuildNetworkModel(cfg, d, false)
	if err != nil {
		t.Fatalf("dot2net failed: %v\n", err)
	}

	// Get expected file list before building files (simulates "dot2net files" command)
	expectedFiles, err := model.ListGeneratedFiles(cfg, nm, false)
	if err != nil {
		t.Fatalf("ListGeneratedFiles failed: %v", err)
	}

	// Remove any existing output files that would be regenerated
	// (to ensure they're counted as "newly generated" in the verification)
	for _, file := range expectedFiles {
		filePath := filepath.Join(tmpDir, file)
		os.Remove(filePath) // ignore errors if file doesn't exist
	}

	// Take snapshot of files before build
	filesBeforeBuild := collectGeneratedFiles(tmpDir)
	fileSetBeforeBuild := make(map[string]bool)
	for _, f := range filesBeforeBuild {
		fileSetBeforeBuild[f] = true
	}

	err = model.BuildConfigFiles(cfg, nm, true)
	if err != nil {
		t.Fatalf("dot2net failed: %v\n", err)
	}

	// Collect files after build and filter out files that existed before
	filesAfterBuild := collectGeneratedFiles(tmpDir)
	var actualFiles []string
	for _, f := range filesAfterBuild {
		if !fileSetBeforeBuild[f] {
			actualFiles = append(actualFiles, f)
		}
	}

	// Verify that ListGeneratedFiles output matches actual generated files
	if !slicesEqualSorted(expectedFiles, actualFiles) {
		// Find missing and extra files for better error messages
		expectedSet := make(map[string]bool)
		for _, f := range expectedFiles {
			expectedSet[f] = true
		}
		actualSet := make(map[string]bool)
		for _, f := range actualFiles {
			actualSet[f] = true
		}

		var missing []string
		for _, f := range expectedFiles {
			if !actualSet[f] {
				missing = append(missing, f)
			}
		}

		var extra []string
		for _, f := range actualFiles {
			if !expectedSet[f] {
				extra = append(extra, f)
			}
		}

		t.Errorf("File list mismatch (ListGeneratedFiles vs actual build):\n"+
			"Expected %d files, got %d files\n"+
			"Missing files (in ListGeneratedFiles but not generated): %v\n"+
			"Extra files (generated but not in ListGeneratedFiles): %v",
			len(expectedFiles), len(actualFiles), missing, extra)
	}

	// recursively search golden files
	goldenDir := filepath.Join(topologyDir, GoldenDirName)
	err = filepath.Walk(goldenDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(goldenDir, path)
		if err != nil {
			return err
		}

		goldenData, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read golden file %s: %v", path, err)
		}

		actualPath := filepath.Join(tmpDir, relPath)
		actualData, err := os.ReadFile(actualPath)
		if err != nil {
			t.Fatalf("failed to read actual file %s: %v", actualPath, err)
		}

		// Normalize paths in actualData for consistent testing
		normalizedActual := normalizePaths(string(actualData), tmpDir)

		if diff := cmp.Diff(string(goldenData), normalizedActual); diff != "" {
			t.Errorf("Mismatch in %s (-expected +actual):\n%s", relPath, diff)
		}

		return nil
	})

	if err != nil {
		t.Errorf("error in example test: %v\n", err)
	}
}

// normalizePaths replaces tmpDir paths with relative paths for consistent testing
// Works across platforms (macOS, Linux, Windows)
func normalizePaths(content, tmpDir string) string {
	abs, err := filepath.Abs(tmpDir)
	if err != nil {
		abs = tmpDir
	}

	// Collect all possible path variations to replace
	pathsToReplace := []string{
		abs + string(filepath.Separator),
		tmpDir + string(filepath.Separator),
	}

	// Handle macOS /var -> /private/var symlink issue
	if strings.HasPrefix(abs, "/var/") {
		privatePath := "/private" + abs + string(filepath.Separator)
		pathsToReplace = append(pathsToReplace, privatePath)
	}
	if strings.HasPrefix(tmpDir, "/var/") && tmpDir != abs {
		privatePath := "/private" + tmpDir + string(filepath.Separator)
		pathsToReplace = append(pathsToReplace, privatePath)
	}

	// Sort by length descending to replace longer paths first
	for i := 0; i < len(pathsToReplace); i++ {
		for j := i + 1; j < len(pathsToReplace); j++ {
			if len(pathsToReplace[i]) < len(pathsToReplace[j]) {
				pathsToReplace[i], pathsToReplace[j] = pathsToReplace[j], pathsToReplace[i]
			}
		}
	}

	// Apply all replacements in order (longest first)
	for _, path := range pathsToReplace {
		content = strings.ReplaceAll(content, path, "")
	}

	return content
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	input, err := os.Open(src)
	if err != nil {
		t.Fatalf("failed to open source file %s: %v", src, err)
	}
	defer input.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatalf("failed to create destination dir: %v", err)
	}

	output, err := os.Create(dst)
	if err != nil {
		t.Fatalf("failed to create destination file %s: %v", dst, err)
	}
	defer output.Close()

	if _, err := io.Copy(output, input); err != nil {
		t.Fatalf("failed to copy file from %s to %s: %v", src, dst, err)
	}
}

// collectGeneratedFiles returns a sorted list of generated files in tmpDir,
// excluding input files (input.dot and input.yaml)
func collectGeneratedFiles(tmpDir string) []string {
	var files []string
	filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(tmpDir, path)
		if err != nil {
			return err
		}

		// Exclude input files
		if relPath != TopologyFileName && relPath != DefinitionFileName {
			// The path is left as the filesystem gives it. dot2net names a
			// generated file with filepath.Join, so its list is in the
			// separator this machine uses and so is this walk - comparing the
			// two is the point of the test, and normalising either would hide a
			// difference rather than allow for one.
			files = append(files, relPath)
		}
		return nil
	})
	return files
}

// slicesEqualSorted compares two string slices after sorting them
func slicesEqualSorted(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	// Create sorted copies
	sortedA := make([]string, len(a))
	sortedB := make([]string, len(b))
	copy(sortedA, a)
	copy(sortedB, b)

	sort.Strings(sortedA)
	sort.Strings(sortedB)

	for i := range sortedA {
		if sortedA[i] != sortedB[i] {
			return false
		}
	}
	return true
}
