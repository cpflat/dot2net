// fieldreport lists what a topology may write, and how much of it anything does.
//
// It exists because the same failure happened twice in one day: a field was
// added that duplicated one already sitting in the same struct, and a live
// field was called dead without looking. Both are answered by one table -
// every field in one place, with the number of bundled topologies that use it.
// A field nothing uses is either new, or dead and worth removing.
//
// Read from the source rather than maintained by hand, because a hand-written
// list drifts: the count in .claude/rules/topologies.md was wrong within days.
//
// What it does not do: follow a value once it leaves the field. Platform, which
// this was written after and which is gone now, was read twice and still did
// nothing, because both readers only copied it into a set nobody looked at.
// Seeing that needs to follow the value through a local variable, which is a
// different kind of tool - a linter. So READS = 0 means dead, but READS > 0
// does not mean alive.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cpflat/dot2net/internal/configsurface"
)

type field struct {
	configsurface.Field
	Uses  int  // bundled topologies that write this key
	SetBy int  // places in mod/ that set this field from Go
	Reads int  // places that read the field back
	Ambig bool // the same field name is on more than one struct
	// AmbigKey marks a yaml key more than one struct declares. The topologies
	// are searched for the key alone, so the count is the two keys together:
	// `after` is both a config's anchor and an entry of blocks:.
	AmbigKey bool
}

// countSetBy is how many places under mod/ assign the field. A field no
// topology writes may still be alive because a module sets it - Executable and
// Provide are like that - and the two have to be told apart, or every such
// field reads as dead.
func countSetBy(name string) int {
	pat := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\s*:`)
	n := 0
	_ = filepath.Walk("mod", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err == nil && pat.Match(b) {
			n++
		}
		return nil
	})
	return n
}

// countUses is a plain search of the bundled topologies for "<key>:". It can
// be fooled - a key that is also an ordinary word will over-count - so the
// number is a signal, not a measurement. Zero is the interesting value.
//
// A key is written either on its own line or as the first key of a list entry,
// "- group: x". Missing the second form is what makes the number lie in the
// direction that matters: config templates are a list, so their keys are read
// as unused.
func countUses(roots []string, key string) int {
	if key == "" || key == "-" {
		return 0
	}
	pat := regexp.MustCompile(`(?m)^\s*(-\s+)?` + regexp.QuoteMeta(key) + `\s*:`)
	n := 0
	for _, root := range roots {
		dirs, _ := os.ReadDir(root)
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			b, err := os.ReadFile(filepath.Join(root, d.Name(), "input.yaml"))
			if err != nil {
				continue
			}
			if pat.Match(b) {
				n++
			}
		}
	}
	return n
}

// countReads counts where the field is read rather than written: a selector
// (x.Field) that is not the left of an assignment. A field set from a struct
// literal appears as a plain key, not a selector, so it is not counted here -
// that is what SetBy is for.
//
// A field nobody reads does nothing, whatever else the numbers say. That is the
// column this report exists for: Platform was declared and read by no one, and a
// second field was written to do the same job because nothing said so. It was
// deleted in 0.8.1, which is the answer this column is meant to lead to.
//
// Names are matched without resolving types, so a name carried by more than one
// struct pools their reads. Such names are marked ambiguous rather than
// silently trusted.
func countReads(dirs []string, name string) int {
	n := 0
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
				strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return nil
			}
			written := map[*ast.SelectorExpr]bool{}
			ast.Inspect(f, func(nd ast.Node) bool {
				as, ok := nd.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for _, lhs := range as.Lhs {
					if se, ok := lhs.(*ast.SelectorExpr); ok {
						written[se] = true
					}
				}
				return true
			})
			ast.Inspect(f, func(nd ast.Node) bool {
				se, ok := nd.(*ast.SelectorExpr)
				if ok && se.Sel.Name == name && !written[se] {
					n++
				}
				return true
			})
			return nil
		})
	}
	return n
}

func main() {
	declared, err := configsurface.Collect("pkg/types/config.go")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fields := make([]field, 0, len(declared))
	for _, d := range declared {
		fields = append(fields, field{Field: d})
	}
	roots := []string{"topologies", "example"}
	for i := range fields {
		fields[i].Uses = countUses(roots, fields[i].YAML)
		fields[i].SetBy = countSetBy(fields[i].Name)
		fields[i].Reads = countReads([]string{"pkg", "mod", "."}, fields[i].Name)
	}
	// Mark the names that more than one struct carries: their read counts are
	// pooled and cannot be trusted on their own.
	byName := map[string]int{}
	for _, f := range fields {
		byName[f.Name]++
	}
	byYAML := map[string]int{}
	for _, f := range fields {
		if f.YAML != "-" {
			byYAML[f.YAML]++
		}
	}
	for i := range fields {
		fields[i].Ambig = byName[fields[i].Name] > 1
		fields[i].AmbigKey = byYAML[fields[i].YAML] > 1
	}

	sort.SliceStable(fields, func(i, j int) bool {
		if fields[i].Struct != fields[j].Struct {
			return fields[i].Struct < fields[j].Struct
		}
		return fields[i].Name < fields[j].Name
	})

	fmt.Printf("%-20s %-20s %-22s %5s %5s %6s  %s\n",
		"STRUCT", "FIELD", "YAML KEY", "TOPO", "MOD", "READS", "WHAT IT IS")
	for _, f := range fields {
		key := f.YAML
		if key == "-" {
			key = "(module only)"
		}
		reads := fmt.Sprintf("%d", f.Reads)
		if f.Ambig {
			reads += "?"
		}
		uses := fmt.Sprintf("%d", f.Uses)
		if f.AmbigKey {
			uses += "?"
		}
		fmt.Printf("%-20s %-20s %-22s %5s %5d %6s  %s\n",
			f.Struct, f.Name, key, uses, f.SetBy, reads, f.Comment)
	}
}
