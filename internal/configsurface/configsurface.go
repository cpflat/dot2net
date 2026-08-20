// Package configsurface lists the keys a topology may write in its YAML, read
// out of the type declarations rather than kept by hand.
//
// Two things ask for this list and they must not disagree: the ledger that says
// how much of the surface anything uses (tool/fieldreport), and the test that
// every key is explained somewhere in the wiki. A list maintained twice drifts,
// and a drifting list of what exists is exactly what this was written against.
package configsurface

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// Field is one key a topology may write.
type Field struct {
	Struct  string // the type declaring it
	Name    string // the Go field name
	YAML    string // the key as written in YAML; "-" when only Go sets it
	Comment string // the field's own comment, first line
}

// Written reports a key a topology can actually write. A tag of "-" marks a
// field a module sets from Go, which no wiki page has any reason to name.
func (f Field) Written() bool {
	return f.YAML != "" && f.YAML != "-"
}

var yamlTag = regexp.MustCompile(`yaml:"([^"]*)"`)

// yamlKey pulls the key out of a struct tag, dropping ",flow" and friends.
func yamlKey(tag string) (string, bool) {
	m := yamlTag.FindStringSubmatch(tag)
	if m == nil {
		return "", false
	}
	return strings.Split(m[1], ",")[0], true
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

// Collect reads the fields declared in one Go source file.
func Collect(path string) ([]Field, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var out []Field
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
			out = append(out, Field{
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
