package example_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cpflat/dot2net/pkg/model"
	"github.com/cpflat/dot2net/pkg/types"
)

const TopologyFileName string = "input.dot"
const DefinitionFileName string = "input.yaml"
const GoldenDirName string = "expected"

func TestExampleScenarios(t *testing.T) {
	// Get absolute path to project root
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	rootDir := filepath.Join(wd, "..", "..") // project root
	exampleDir := filepath.Join(rootDir, "example")
	
	// Find all scenarios with input.dot and input.yaml
	entries, err := os.ReadDir(exampleDir)
	if err != nil {
		t.Fatalf("failed to read example directory: %v", err)
	}
	
	var scenarios []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		scenarioPath := filepath.Join(exampleDir, entry.Name())
		dotFile := filepath.Join(scenarioPath, TopologyFileName)
		yamlFile := filepath.Join(scenarioPath, DefinitionFileName)
		
		if _, err := os.Stat(dotFile); err == nil {
			if _, err := os.Stat(yamlFile); err == nil {
				scenarios = append(scenarios, entry.Name())
			}
		}
	}
	
	if len(scenarios) == 0 {
		t.Fatalf("no valid scenarios found in %s", exampleDir)
	}
	
	t.Logf("Found %d scenarios: %v", len(scenarios), scenarios)
	
	for _, scenarioName := range scenarios {
		t.Run(scenarioName, func(t *testing.T) {
			tryScenario(t, rootDir, scenarioName)
		})
	}
}

func tryScenario(t *testing.T, rootDir string, scenarioName string) {
	scenarioDir := filepath.Join(rootDir, "example", scenarioName)

	// create tmp dir
	tmpDir, err := os.MkdirTemp("", "dot2net_test")
	if err != nil {
		t.Fatalf("failed to create tmp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// copy input files into tmp dir
	exampleDir := scenarioDir
	err = filepath.WalkDir(exampleDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// skip subdirectories
		if d.IsDir() && path != exampleDir {
			return filepath.SkipDir
		}

		// copy only files
		if !d.IsDir() {
			relPath, err := filepath.Rel(exampleDir, path)
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
	// copyFile(t, filepath.Join(scenarioDir, TopologyFileName), topoFile)
	// copyFile(t, filepath.Join(scenarioDir, DefinitionFileName), defFile)

	// execute dot2net
	oldWd, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}
	defer os.Chdir(oldWd)

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
	err = model.BuildConfigFiles(cfg, nm, true)
	if err != nil {
		t.Fatalf("dot2net failed: %v\n", err)
	}

	// debug: list up files in tmpDir recursively
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// if info.IsDir() {
		// 	return nil
		// }
		fmt.Printf("found file in tmpDir: %s\n", path)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to list files in tmpDir: %v", err)
	}

	// recursively search golden files
	goldenDir := filepath.Join(scenarioDir, GoldenDirName)
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
