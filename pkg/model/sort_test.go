package model

import (
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// TestSorterGathersSeveralGroups pins what a multi-group sorter is for: the
// blocks of every group it names land in one column, ordered by priority
// across the groups rather than group by group.
func TestSorterGathersSeveralGroups(t *testing.T) {
	cfg, nm := buildFullModel(t, `
name: sort_groups
global:
  path: local
nodeclass:
  - name: router
    use: [own, shared]
    config:
      - file: out
        style: sort
        sort_groups: [private, common]
        template: ["header"]
  - name: own
    config:
      - group: private
        priority: 10
        template: ["private-late"]
      - group: private
        priority: -10
        template: ["private-early"]
  - name: shared
    config:
      - group: common
        template: ["common-middle"]
file:
  - name: out
`, hookDot)

	out := generateFor(t, cfg, nm, "r1", "out")
	want := []string{"private-early", "header", "common-middle", "private-late"}
	at := make([]int, len(want))
	for i, block := range want {
		at[i] = strings.Index(out, block)
		if at[i] < 0 {
			t.Fatalf("%s is missing from the column:\n%s", block, out)
		}
	}
	for i := 1; i < len(at); i++ {
		if at[i-1] > at[i] {
			t.Errorf("%s should come before %s:\n%s", want[i-1], want[i], out)
		}
	}
}

// TestSorterGroupNamesAreCheckedTogether: naming several groups does not let a
// mistyped one through, and neither name may be given twice.
func TestSorterGroupNamesAreCheckedTogether(t *testing.T) {
	cfgPath, _ := writeTempInput(t, `
name: sort_groups_typo
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        style: sort
        sort_groups: [private, common]
      - group: comon
        template: ["typo"]
file:
  - name: out
`, hookDot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	_, err = types.LoadTemplates(cfg)
	if err == nil {
		t.Fatal("a group none of the sorters gathers must be an error")
	}
	if !strings.Contains(err.Error(), "comon") {
		t.Errorf("the message should name the group written: %v", err)
	}
}
