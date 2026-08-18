package model

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// provideYAML is a topology with one file of each kind: one mounted at its own
// path, one copied to a path in a directory nobody owns.
const provideYAML = `
name: provide_test
global:
  path: local
file:
  - name: conf
    path: /etc/app/app.conf
  - name: motd
    path: /etc/motd
    provide: copy
nodeclass:
  - name: all
    config:
      - file: conf
        template: ["conf for {{ .name }}"]
      - file: motd
        template: ["welcome to {{ .name }}"]
`

const provideDot = `graph {
  r1 [class="all"];
  r2 [class="all"];
  r1 -- r2;
}`

// TestCopyWaitsInStaging pins where a copied file is written. It cannot sit
// beside the mounted files: the staging directory is mounted into the container
// as a whole, so anything else under the node directory would ride along.
func TestCopyWaitsInStaging(t *testing.T) {
	cfg, nm := buildForProvide(t, provideYAML, provideDot)

	files, err := DescribeGeneratedFiles(cfg, nm)
	if err != nil {
		t.Fatalf("DescribeGeneratedFiles: %v", err)
	}
	got := map[string]string{}
	for _, f := range files {
		got[f.Path] = f.Provide
	}
	// The listing names a file the way this machine does, so the wanted paths
	// are built the same way - see TestListedPathsUseTheLocalSeparator.
	mounted := filepath.Join("r1", "etc", "app", "app.conf")
	copied := filepath.Join("r1", types.StagingDirName, "etc", "motd")
	if p, ok := got[mounted]; !ok || p != types.ProvideMount {
		t.Errorf("a mounted file belongs at its container path: %v", got)
	}
	if p, ok := got[copied]; !ok || p != types.ProvideCopy {
		t.Errorf("a copied file belongs under staging/: %v", got)
	}
}

// TestProvideDefaultsToMount: saying nothing has to keep meaning what it meant
// before the setting existed.
func TestProvideDefaultsToMount(t *testing.T) {
	fd := &types.FileDefinition{Name: "x", Path: "/etc/x"}
	if fd.GetProvide() != types.ProvideMount {
		t.Errorf("a file that says nothing is provided by %s, want mount", fd.GetProvide())
	}
}

// TestCopyIntoMountedDirectoryIsRejected covers the combination that cannot
// work: the mount is read only, so the copy has nowhere to land. On Kathara it
// fails without a word, which is why it is caught here instead.
func TestCopyIntoMountedDirectoryIsRejected(t *testing.T) {
	_, err := buildForProvideErr(t, `
name: copy_into_mount
global:
  path: local
file:
  - name: conf
    path: /etc/frr/frr.conf
  - name: extra
    path: /etc/frr/extra.conf
    provide: copy
nodeclass:
  - name: all
    config:
      - file: conf
        template: ["a"]
      - file: extra
        template: ["b"]
`, provideDot)
	if err == nil {
		t.Fatal("copying into a mounted directory must be rejected")
	}
	for _, want := range []string{"extra", "/etc/frr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message should name the file and the directory: %v", err)
		}
	}
}

// TestUnknownProvideIsRejected: a misspelled value must not fall back to the
// default, which would deliver the file in a way the author did not ask for.
func TestUnknownProvideIsRejected(t *testing.T) {
	cfgPath, _ := writeTempInput(t, `
name: bad_provide
file:
  - name: conf
    path: /etc/app/app.conf
    provide: mound
`, provideDot)
	if _, err := types.LoadConfig(cfgPath); err == nil {
		t.Fatal("an unknown provide value must be rejected")
	} else if !strings.Contains(err.Error(), "mound") {
		t.Errorf("the message should name the offending value: %v", err)
	}
}

func buildForProvide(t *testing.T, yaml, dot string) (*types.Config, *types.NetworkModel) {
	t.Helper()
	cfg, nm, err := buildProvideModel(t, yaml, dot)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return cfg, nm
}

func buildForProvideErr(t *testing.T, yaml, dot string) (*types.NetworkModel, error) {
	t.Helper()
	_, nm, err := buildProvideModel(t, yaml, dot)
	return nm, err
}

func buildProvideModel(t *testing.T, yaml, dot string) (*types.Config, *types.NetworkModel, error) {
	t.Helper()
	cfgPath, dotPath := writeTempInput(t, yaml, dot)
	cfg, err := types.LoadConfig(cfgPath)
	if err != nil {
		return nil, nil, err
	}
	d, err := DiagramFromDotFile(dotPath)
	if err != nil {
		t.Fatalf("DiagramFromDotFile: %v", err)
	}
	nm, err := BuildNetworkModelForFileList(cfg, d)
	if err != nil {
		return cfg, nil, err
	}
	return cfg, nm, nil
}
