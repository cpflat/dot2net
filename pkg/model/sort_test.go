package model

import (
	"os"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// buildConfigFilesErr generates into a directory of the test's own and returns
// what generating said, for the failures that are only visible then.
func buildConfigFilesErr(t *testing.T, cfg *types.Config, nm *types.NetworkModel) error {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(wd)

	return BuildConfigFiles(cfg, nm, false)
}

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

// TestAnchorBeatsPriority is what after: exists for: the block is placed by the
// name of its neighbour, and the numbers of the blocks around it stop mattering.
func TestAnchorBeatsPriority(t *testing.T) {
	cfg, nm := buildFullModel(t, `
name: sort_anchor
global:
  path: local
nodeclass:
  - name: router
    use: [platform, topology]
    config:
      - file: out
        style: sort
        sort_group: column
  - name: platform
    config:
      - group: column
        name: deploy
        priority: 10
        template: ["deploy-command"]
  - name: topology
    config:
      - group: column
        after: deploy
        template: ["after-deploy"]
      - group: column
        before: deploy
        template: ["before-deploy"]
file:
  - name: out
`, hookDot)

	out := generateFor(t, cfg, nm, "r1", "out")
	before := strings.Index(out, "before-deploy")
	deploy := strings.Index(out, "deploy-command")
	after := strings.Index(out, "after-deploy")
	if before < 0 || deploy < 0 || after < 0 {
		t.Fatalf("all three blocks belong in the column, got:\n%s", out)
	}
	if !(before < deploy && deploy < after) {
		t.Errorf("the anchored blocks sit either side of deploy, got:\n%s", out)
	}
}

// TestAnchorNotInTheColumnIsNotAnError: a block cannot know which objects its
// anchor is generated for, so an anchor that is absent here places nothing.
func TestAnchorNotInTheColumnIsNotAnError(t *testing.T) {
	cfg, nm := buildFullModel(t, `
name: sort_anchor_absent
global:
  path: local
nodeclass:
  - name: router
    use: [topology]
    config:
      - file: out
        style: sort
        sort_group: column
  - name: topology
    config:
      - group: column
        after: deploy
        template: ["lonely"]
  - name: elsewhere
    config:
      - group: column
        name: deploy
        template: ["deploy-command"]
file:
  - name: out
`, hookDot)

	out := generateFor(t, cfg, nm, "r1", "out")
	if !strings.Contains(out, "lonely") {
		t.Errorf("the block belongs in the column even with its anchor absent, got:\n%s", out)
	}
}

// TestCircularAnchorsAreReported: the blocks would have no order at all, and
// silently picking one is how a column ends up in an order nobody asked for.
func TestCircularAnchorsAreReported(t *testing.T) {
	cfg, nm := buildFullModel(t, `
name: sort_anchor_cycle
global:
  path: local
nodeclass:
  - name: router
    use: [topology]
    config:
      - file: out
        style: sort
        sort_group: column
  - name: topology
    config:
      - group: column
        name: first
        after: second
        template: ["a"]
      - group: column
        name: second
        after: first
        template: ["b"]
file:
  - name: out
`, hookDot)

	err := buildConfigFilesErr(t, cfg, nm)
	if err == nil {
		t.Fatal("blocks placed after each other in a circle must be reported")
	}
	if !strings.Contains(err.Error(), "circle") {
		t.Errorf("the message should say what is wrong: %v", err)
	}
}

// TestAnchorIsRefusedOutsideAColumn keeps the three orderings apart: depends:
// orders generation, blocks: merges, and after: only places a block in a column.
func TestAnchorIsRefusedOutsideAColumn(t *testing.T) {
	for _, c := range []struct {
		name   string
		config string
		want   string
	}{
		{"no group", `
      - file: out
        after: deploy
        template: ["x"]
      - group: column
        name: deploy
        template: ["y"]`, "writes into no group"},
		{"with priority", `
      - group: column
        after: deploy
        priority: 5
        template: ["x"]
      - group: column
        name: deploy
        template: ["y"]`, "not both"},
		{"unknown anchor", `
      - group: column
        after: deployy
        template: ["x"]
      - group: column
        name: deploy
        template: ["y"]`, "no config template is named"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cfgPath, _ := writeTempInput(t, `
name: sort_anchor_misuse
global:
  path: local
nodeclass:
  - name: router
    config:
      - file: out
        style: sort
        sort_group: column
`+c.config+`
file:
  - name: out
`, hookDot)
			cfg, err := types.LoadConfig(cfgPath)
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if _, err = types.LoadTemplates(cfg); err == nil {
				t.Fatal("this use of after: must be refused")
			} else if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the message should say %q, got: %v", c.want, err)
			}
		})
	}
}
