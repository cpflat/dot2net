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
)

type field struct {
	Struct  string
	Name    string
	YAML    string // "" when the field is not written in YAML at all
	Comment string
	Uses    int // bundled topologies that write this key
	SetBy   int // places in mod/ that set this field from Go
}

// yamlKey pulls the key out of a struct tag, dropping ",flow" and friends.
// A tag of "-" means the field is set in Go and never written by a topology.
func yamlKey(tag string) (string, bool) {
	m := regexp.MustCompile(`yaml:"([^"]*)"`).FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	key := strings.Split(m[1], ",")[0]
	return key, true
}

// firstLine is the field's own comment, trimmed to one line: enough to tell
// what it is, short enough for a table.
func firstLine(doc *ast.CommentGroup) string {
	if doc == nil {
		return ""
	}
	for _, c := range doc.List {
		s := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		if s != "" {
			return s
		}
	}
	return ""
}

func collect(path string) ([]field, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []field
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, fl := range st.Fields.List {
			if fl.Tag == nil || len(fl.Names) == 0 {
				continue
			}
			key, ok := yamlKey(fl.Tag.Value)
			if !ok {
				continue
			}
			out = append(out, field{
				Struct:  ts.Name.Name,
				Name:    fl.Names[0].Name,
				YAML:    key,
				Comment: firstLine(fl.Doc),
			})
		}
		return true
	})
	return out, nil
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
func countUses(roots []string, key string) int {
	if key == "" || key == "-" {
		return 0
	}
	pat := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(key) + `\s*:`)
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

func main() {
	fields, err := collect("pkg/types/config.go")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	roots := []string{"topologies", "example"}
	for i := range fields {
		fields[i].Uses = countUses(roots, fields[i].YAML)
		fields[i].SetBy = countSetBy(fields[i].Name)
	}
	sort.SliceStable(fields, func(i, j int) bool {
		if fields[i].Struct != fields[j].Struct {
			return fields[i].Struct < fields[j].Struct
		}
		return fields[i].Name < fields[j].Name
	})

	fmt.Printf("%-20s %-20s %-22s %5s %5s  %s\n",
		"STRUCT", "FIELD", "YAML KEY", "TOPO", "MOD", "WHAT IT IS")
	for _, f := range fields {
		key := f.YAML
		if key == "-" {
			key = "(module only)"
		}
		fmt.Printf("%-20s %-20s %-22s %5d %5d  %s\n",
			f.Struct, f.Name, key, f.Uses, f.SetBy, f.Comment)
	}
}
