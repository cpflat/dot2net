package example

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/internal/configsurface"
)

// knownUndocumented are the keys a topology may write that no wiki page
// explains. The list is a debt, not an exemption: it must only shrink. A key
// that gets written up has to come off it, which the test enforces.
var knownUndocumented = map[string]string{
	"anchor":              "0.8.1: to be written up with the sort chapter",
	"sort_groups":         "0.8.1: to be written up with the sort chapter",
	"generator":           "predates the ledger; module-facing, but a topology can write it",
	"mgmt_interfaceclass": "predates the ledger; one bundled topology uses it",
	"mountsourcepath":     "predates the ledger",
	"namespace_format":    "predates the ledger; part of the FormatStyle surface",
	"namespace_formats":   "predates the ledger; part of the FormatStyle surface",
}

// TestEveryKeyIsInTheWiki reports a key a topology may write that appears
// nowhere in the wiki.
//
// The ledger says what the config surface is; this says whether a reader can
// find out what any of it means. Prose is where a missing entry hides - nothing
// about a page says which keys it left out - so the list of keys is taken from
// the declarations and the pages are searched for each one.
//
// It cannot tell an explanation from a passing mention, and does not try: what
// it is for is absence. `platform` sat in the surface undocumented from 0.7 or
// before until it was deleted in 0.8.1, and nothing would have said so.
//
// The wiki is a separate repository, reached through a symlink that is not
// checked in, so this runs where someone has both and is skipped where they do
// not - CI among them. The release procedure names it as a step for that
// reason.
func TestEveryKeyIsInTheWiki(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	root := filepath.Join(wd, "..", "..")
	pages, err := filepath.Glob(filepath.Join(root, "wiki", "*.md"))
	if err != nil || len(pages) == 0 {
		t.Skip("no wiki beside this checkout: see CLAUDE.md for the symlink")
	}

	text := map[string]string{}
	for _, page := range pages {
		b, err := os.ReadFile(page)
		if err != nil {
			t.Fatalf("reading %s: %v", page, err)
		}
		text[filepath.Base(page)] = string(b)
	}

	fields, err := configsurface.Collect(filepath.Join(root, "pkg", "types", "config.go"))
	if err != nil {
		t.Fatalf("reading the config surface: %v", err)
	}

	var missing, documentedButListed []string
	seen := map[string]bool{}
	for _, f := range fields {
		if !f.Written() || seen[f.YAML] {
			continue
		}
		seen[f.YAML] = true

		found := false
		for _, body := range text {
			if mentions(body, f.YAML) {
				found = true
				break
			}
		}
		_, known := knownUndocumented[f.YAML]
		switch {
		case !found && !known:
			missing = append(missing, fmt.Sprintf("%s (%s.%s)", f.YAML, f.Struct, f.Name))
		case found && known:
			documentedButListed = append(documentedButListed, f.YAML)
		}
	}
	sort.Strings(missing)
	sort.Strings(documentedButListed)

	if len(missing) > 0 {
		t.Errorf("these keys can be written in a topology and are explained nowhere in the wiki:\n  %s\n"+
			"Write them up, or add them to knownUndocumented with a reason.",
			strings.Join(missing, "\n  "))
	}
	for _, key := range documentedButListed {
		t.Errorf("%s is in the wiki now - take it out of knownUndocumented (%s)", key, knownUndocumented[key])
	}
}

// mentions looks for the key as a key: as a marked-up word, or written the way
// a topology writes it, on a line of its own or as the head of a list entry.
func mentions(body, key string) bool {
	if strings.Contains(body, "`"+key+"`") {
		return true
	}
	pat := regexp.MustCompile(`(?m)^\s*(-\s+)?` + regexp.QuoteMeta(key) + `\s*:`)
	return pat.MatchString(body)
}
