package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// virtualWiringYAML has one interface class that withholds its configuration
// while staying an end of wiring the platform lays. r1's end of the r1-r2 link
// carries it; everything else is ordinary.
const virtualWiringYAML = `
name: virtual_wiring
global:
  path: local
class_policy:
  interface:
    default: [default]
module_config:
  kathara:
    mount_dirs: [/etc/frr]
module:
  - kathara
  - tinet
  - containerlab
file:
  - name: frr.conf
    path: /etc/frr/frr.conf
layer:
  - name: ip
    default_connect: true
    policy:
      - name: ip
        range: 10.0.0.0/16
        prefix: 24
nodeclass:
  - name: router
    values:
      image: quay.io/frrouting/frr:8.5.4
      kind: linux
    interface_policy: [ip]
    config:
      - name: startup
        template: []
      - file: frr.conf
        template:
          - "{{ .interfaces_addr }}"
interfaceclass:
  - name: quiet
    virtual: true
    deploy: link
  - name: default
    config:
      - name: addr
        template:
          - "interface {{ .name }}"
`

const virtualWiringDot = `digraph {
  r1 [xlabel="router"];
  r2 [xlabel="router"];
  r3 [xlabel="router"];
  r1 -> r2 [dir="none", taillabel="quiet"];
  r2 -> r3 [dir="none"];
}`

// TestVirtualDoesNotUnwire pins the line between the two axes on the one
// template that sits closest to it. virtual withholds an object's
// configuration; whether the platform lays a link is deploy's answer. A wiring
// template is therefore written even for an interface marked virtual.
//
// Getting this wrong is not visible in one platform's output, which is why it is
// worth a test of its own: containerlab writes its wiring per connection and
// would have kept the link, while TiNET and Kathara write theirs per interface
// and would have lost it. Kathara would then have had a gap in lab.conf, which
// it refuses to start on and which nothing here would have reported.
func TestVirtualDoesNotUnwire(t *testing.T) {
	dir := generateInto(t, virtualWiringYAML, virtualWiringDot)

	for _, c := range []struct {
		file, want, platform string
	}{
		{"topo.yaml", "[r1:eth0, r2:eth0]", "containerlab"},
		{"spec.yaml", "{name: eth0, type: direct, args: r2#eth0}", "TiNET"},
		{filepath.Join("kathara", "lab.conf"), `r1[0]=`, "Kathara"},
	} {
		got := readGenerated(t, dir, c.file)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: %s does not wire the virtual interface (looking for %q in:\n%s)",
				c.platform, c.file, c.want, got)
		}
	}

	// The other half of the same rule: what virtual does withhold is the
	// interface's own configuration.
	frr := readGenerated(t, dir, filepath.Join("r1", "etc", "frr", "frr.conf"))
	if strings.Contains(frr, "interface eth0") {
		t.Errorf("the configuration of a virtual interface must not be written, got:\n%s", frr)
	}
}

// TestVirtualNodeIsStillDeployed pins the node half of the same rule, in the
// generated files rather than in the model. Two things went wrong here in turn:
// nothing applied a node class's virtual to the node at all, and then, once it
// did, virtual withheld the platform's own record that the node exists — so the
// node vanished from topo.yaml while the links to it stayed, which containerlab
// refuses to deploy. Neither is visible from the model's flags, which is why
// this reads the output.
func TestVirtualNodeIsStillDeployed(t *testing.T) {
	dir := generateInto(t, strings.Replace(virtualWiringYAML,
		"nodeclass:\n", "nodeclass:\n  - name: unwritten\n    deploy: container\n    virtual: true\n", 1),
		strings.Replace(virtualWiringDot,
			`r2 [xlabel="router"];`, `r2 [xlabel="router;unwritten"];`, 1))

	for _, c := range []struct {
		file, want, platform string
	}{
		{"topo.yaml", "r2:", "containerlab"},
		{"spec.yaml", "name: r2", "TiNET"},
		{filepath.Join("kathara", "lab.conf"), "r2[image]=", "Kathara"},
	} {
		got := readGenerated(t, dir, c.file)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: %s drops the node itself, leaving the links to it dangling "+
				"(looking for %q in:\n%s)", c.platform, c.file, c.want, got)
		}
	}

	// What virtual does withhold: the node's own configuration, and with it the
	// bind that would have delivered a file that was never written.
	if _, err := os.Stat(filepath.Join(dir, "r2", "etc", "frr", "frr.conf")); err == nil {
		t.Error("the configuration of a virtual node must not be written")
	}
	// Every platform delivers a file its own way, and each of them has to leave
	// the delivery out: a mount naming a file nobody wrote points at a path that
	// does not exist. TiNET got this wrong while the other two were right, which
	// is why all three are named here.
	for _, c := range []struct{ file, mount, platform string }{
		{"topo.yaml", "r2/etc/frr", "containerlab"},
		{"spec.yaml", "r2/etc/frr", "TiNET"},
		{filepath.Join("kathara", "lab.conf"), "r2[volume]", "Kathara"},
	} {
		if got := readGenerated(t, dir, c.file); strings.Contains(got, c.mount) {
			t.Errorf("%s: a virtual node must not be given %s - no file was written for it:\n%s",
				c.platform, c.mount, got)
		}
	}
}

// generateInto builds the model and writes every generated file into a
// directory of the test's own, which it returns.
func generateInto(t *testing.T, yaml, dot string) string {
	t.Helper()
	cfg, nm := buildFullModel(t, yaml, dot)

	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(wd)

	if err := BuildConfigFiles(cfg, nm, false); err != nil {
		t.Fatalf("BuildConfigFiles: %v", err)
	}
	return dir
}

func readGenerated(t *testing.T, dir, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("reading the generated %s: %v", name, err)
	}
	return string(content)
}
