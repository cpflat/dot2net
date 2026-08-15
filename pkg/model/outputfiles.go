package model

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

// generatedFile is one file the model will write, together with the definition
// that asks for it. Knowing the definition is what lets a collision be reported
// in the topology's own words rather than as a bare path.
type generatedFile struct {
	path    string
	fileDef *types.FileDefinition
}

// generatedFiles walks the file definitions and works out where each one lands.
// ListGeneratedFiles and the uniqueness check both read from here, so that the
// files `dot2net files` names are the files `dot2net build` writes.
func generatedFiles(cfg *types.Config, nm *types.NetworkModel) ([]generatedFile, error) {
	contains := func(list []string, name string) bool {
		for _, item := range list {
			if item == name {
				return true
			}
		}
		return false
	}

	var files []generatedFile
	for _, fileDef := range cfg.FileDefinitions {
		// A definition that can name no file generates none.
		if fileDef.Name == "" && fileDef.NamePrefix == "" && fileDef.NameSuffix == "" {
			continue
		}

		switch fileDef.Scope {
		case types.ClassTypeNetwork:
			if contains(nm.FilesToGenerate(cfg), fileDef.Name) {
				files = append(files, generatedFile{
					path:    filepath.Join(fileDef.Subdir, fileDef.GetFileName("")),
					fileDef: fileDef,
				})
			}
		case types.ClassTypeGroup:
			outputLocation := fileDef.GetOutputLocation()
			for _, group := range nm.Groups {
				if group.IsVirtual() || !contains(group.FilesToGenerate(cfg), fileDef.Name) {
					continue
				}
				dirname := ""
				if outputLocation != "root" {
					dirname = filepath.Join(group.Name, fileDef.Subdir)
				}
				files = append(files, generatedFile{
					path:    filepath.Join(dirname, fileDef.GetFileName(group.Name)),
					fileDef: fileDef,
				})
			}
		case types.ClassTypeNode, "":
			for _, node := range nm.Nodes {
				// A node whose configuration is withheld has no configuration
				// files, so neither the build nor this list has one to name.
				if !node.IsMaterialised() || node.IsVirtual() ||
					!contains(node.FilesToGenerate(cfg), fileDef.Name) {
					continue
				}
				outputPath, err := node.OutputPath(cfg, fileDef)
				if err != nil {
					return nil, err
				}
				files = append(files, generatedFile{path: outputPath, fileDef: fileDef})
			}
		}
	}
	return files, nil
}

// checkCopyTargets rejects a file whose copy would land inside a directory that
// is mounted. The mount is read only, so the copy would fail at deploy time -
// and on Kathara it fails without a word, leaving a lab that looks deployed and
// is missing a file.
func checkCopyTargets(cfg *types.Config, nm *types.NetworkModel) error {
	var mounted, copied []*types.FileDefinition
	for _, filedef := range cfg.FileDefinitions {
		if filedef.Path == "" {
			continue
		}
		if filedef.GetProvide() == types.ProvideCopy {
			copied = append(copied, filedef)
		} else {
			mounted = append(mounted, filedef)
		}
	}
	if len(copied) == 0 || len(mounted) == 0 {
		return nil
	}

	for _, node := range nm.Nodes {
		if !node.IsMaterialised() {
			continue
		}
		generated := make(map[string]bool)
		for _, name := range node.FilesToGenerate(cfg) {
			generated[name] = true
		}
		for _, dst := range copied {
			if !generated[dst.Name] {
				continue
			}
			for _, src := range mounted {
				if !generated[src.Name] {
					continue
				}
				dir := path.Dir(src.Path)
				if path.Dir(dst.Path) == dir || strings.HasPrefix(dst.Path, dir+"/") {
					return fmt.Errorf(
						"file %s is copied to %s, which is inside %s - the directory %s is mounted into. "+
							"A mount is read only, so the copy cannot land there: give %s another path, or "+
							"provide it by mount as well",
						dst.Name, dst.Path, dir, src.Name, dst.Name)
				}
			}
		}
	}
	return nil
}

// checkOutputFilesUnique rejects two file definitions that write the same file.
//
// A file has one set of config templates behind it, which is what decides the
// order of what it holds. Two definitions landing on one path have no such
// order: whichever is written last wins and the other's content is gone without
// a word. This stayed possible because a definition's name and the name of the
// file it writes are separate things - name_prefix and name_suffix build the
// filename, so `startup` and `kathara_startup` are different definitions that
// both write r1.startup.
func checkOutputFilesUnique(cfg *types.Config, nm *types.NetworkModel) error {
	files, err := generatedFiles(cfg, nm)
	if err != nil {
		return err
	}
	seen := make(map[string]*types.FileDefinition, len(files))
	for _, file := range files {
		previous, exists := seen[file.path]
		if !exists {
			seen[file.path] = file.fileDef
			continue
		}
		return fmt.Errorf(
			"file definitions %s and %s both generate %s; a file is written by one definition, "+
				"so give one of them another name_prefix/name_suffix or merge their config entries",
			previous.Name, file.fileDef.Name, file.path)
	}
	return nil
}

// GeneratedFile describes one file for a person reading a list of them: where it
// is written, where it is meant to end up, and how it gets there.
type GeneratedFile struct {
	Path          string
	ContainerPath string
	Provide       string
}

// DescribeGeneratedFiles lists the files with what is known about their
// delivery. ListGeneratedFiles stays a list of paths, because `dot2net clean`
// reads it.
func DescribeGeneratedFiles(cfg *types.Config, nm *types.NetworkModel) ([]GeneratedFile, error) {
	generated, err := generatedFiles(cfg, nm)
	if err != nil {
		return nil, err
	}
	described := make([]GeneratedFile, 0, len(generated))
	for _, file := range generated {
		gf := GeneratedFile{Path: file.path}
		if file.fileDef.Path != "" {
			gf.ContainerPath = file.fileDef.Path
			gf.Provide = file.fileDef.GetProvide()
		}
		described = append(described, gf)
	}
	sort.Slice(described, func(i, j int) bool { return described[i].Path < described[j].Path })
	return described, nil
}
