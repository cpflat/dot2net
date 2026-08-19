package example

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A field that is written and never read does nothing, and says the opposite:
// it is declared, it is documented, and reading the declaration tells you a
// feature exists. Platform was like that - and a second field was added to do
// the same job, because nothing said the first one was hollow.
//
// So the rule this test enforces is: a written-but-unread declaration is either
// given a reader or removed. Leaving it is what let the mistake happen.
//
// What it does not do is follow a value once it leaves the field. That needs a
// different kind of analysis, and a half-done version reports things that are
// fine. It does not need to: a chain of dead values always ends in a field like
// this, so removing the end brings the next one into view. Platform is two
// steps away - take platformSet out and Platform's only two readers go with it.
//
// knownUnread are the ones already found and not yet decided. The list is a
// debt, not an exemption: it must only shrink. See doc/ROADMAP.md TODO 93.
var knownUnread = map[string]string{
	"platformSet":                "TODO 93: decide whether ConfigTemplate.Platform lives or goes",
	"SorterConfigTemplateGroups": "TODO 91(c): to be given a reader - it checks group names for typos",
}

// fieldUse counts, for every struct field name in the repository, the places
// that write it and the places that read it.
type fieldUse struct {
	structs map[string]bool // the structs declaring this name
	writes  int
	reads   int
	// Two kinds of field are read without ever being named. A field with a
	// serialisation tag is read by encoding/json or the YAML loader, which
	// leave no selector behind - ConnectionData is written for `dot2net data`
	// and read only by whoever parses that output. A field of a struct used as
	// a map key is read by the comparison the map does - sorterKey exists to be
	// compared. Neither is dead, and neither can be told apart from platformSet
	// by looking at mentions.
	tagged   bool
	inMapKey bool
}

func TestNoWriteOnlyFields(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	root := filepath.Join(wd, "..", "..")
	use := map[string]*fieldUse{}
	at := func(name string) *fieldUse {
		if use[name] == nil {
			use[name] = &fieldUse{structs: map[string]bool{}}
		}
		return use[name]
	}

	// Structs that appear as a map's key type: their fields are read by the
	// comparison the map does, which leaves no mention behind.
	mapKeyStructs := map[string]bool{}
	for _, dir := range []string{"pkg", "mod", "internal"} {
		walkGoFiles(t, filepath.Join(root, dir), func(path string, f *ast.File) {
			ast.Inspect(f, func(n ast.Node) bool {
				if mt, ok := n.(*ast.MapType); ok {
					if id, ok := mt.Key.(*ast.Ident); ok {
						mapKeyStructs[id.Name] = true
					}
				}
				return true
			})
		})
	}

	for _, dir := range []string{"pkg", "mod", "internal"} {
		walkGoFiles(t, filepath.Join(root, dir), func(path string, f *ast.File) {
			// Declarations: which names are struct fields at all, and where.
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
					for _, nm := range fl.Names {
						u := at(nm.Name)
						u.structs[ts.Name.Name] = true
						if fl.Tag != nil {
							u.tagged = true
						}
						if mapKeyStructs[ts.Name.Name] {
							u.inMapKey = true
						}
					}
				}
				return true
			})

			// Writes: the left of an assignment, and a key in a struct literal.
			written := map[ast.Node]bool{}
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.AssignStmt:
					for _, lhs := range v.Lhs {
						if se, ok := lhs.(*ast.SelectorExpr); ok {
							written[se] = true
							at(se.Sel.Name).writes++
						}
					}
				case *ast.CompositeLit:
					for _, el := range v.Elts {
						kv, ok := el.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if id, ok := kv.Key.(*ast.Ident); ok {
							at(id.Name).writes++
						}
					}
				}
				return true
			})

			// Filling a collection is not reading it. x.Set.Add(v) mentions
			// Set without taking anything out of it, and counting that as a
			// read is what hid platformSet: a set that is built and never
			// looked at reads, by this measure, exactly like a live one.
			//
			// Only the mutating calls are excused. x.Set.Contains(v) is a read,
			// and so is passing x.Set anywhere.
			filling := map[ast.Node]bool{}
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				method, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch method.Sel.Name {
				case "Add", "Append", "Set", "Put", "Insert", "Remove", "Delete", "Clear":
				default:
					return true
				}
				if inner, ok := method.X.(*ast.SelectorExpr); ok {
					filling[inner] = true
				}
				return true
			})

			// Reads: every other mention of the name after a dot.
			ast.Inspect(f, func(n ast.Node) bool {
				if se, ok := n.(*ast.SelectorExpr); ok && !written[se] && !filling[se] {
					at(se.Sel.Name).reads++
				}
				return true
			})
		})
	}

	var found, ambiguous []string
	_ = err
	for name, u := range use {
		if len(u.structs) == 0 || u.writes == 0 || u.reads > 0 {
			continue
		}
		if u.tagged || u.inMapKey {
			continue
		}
		// A name several structs carry pools their uses, so a read of one hides
		// the silence of another. Reported rather than judged.
		if len(u.structs) > 1 {
			ambiguous = append(ambiguous, name)
			continue
		}
		if _, known := knownUnread[name]; known {
			continue
		}
		for s := range u.structs {
			found = append(found, s+"."+name)
		}
	}
	sort.Strings(found)
	sort.Strings(ambiguous)

	if len(found) > 0 {
		t.Errorf("these fields are written and never read: %v\n"+
			"A declaration nothing reads says a feature exists when none does. "+
			"Give it a reader or take it out; if it is waiting on a decision, "+
			"add it to knownUnread with the item that decides it.", found)
	}
	if len(ambiguous) > 0 {
		t.Logf("names carried by more than one struct, not judged: %v", ambiguous)
	}
	for name, why := range knownUnread {
		u := use[name]
		if u != nil && u.reads > 0 {
			t.Errorf("%s is read now - take it out of knownUnread (%s)", name, why)
		}
	}
}

func walkGoFiles(t *testing.T, dir string, fn func(string, *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") ||
			strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}
		fn(path, f)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
}
