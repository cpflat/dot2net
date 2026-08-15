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

// TestVirtualNodeIsMarkedAndStillDeployed guards a flag that was silently
// dropped once: nothing applied a node class's virtual to the node, so writing
// it had no effect at all. It also pins that the flag stops at configuration -
// a node nobody configures is still a node the platform deploys.
func TestVirtualNodeIsMarkedAndStillDeployed(t *testing.T) {
	_, nm := buildFullModel(t, `
name: virtual_node
global:
  path: local
nodeclass:
  - name: unwritten
    deploy: container
    virtual: true
  - name: ordinary
`, `graph { r1 [class="unwritten"]; r2 [class="ordinary"]; r1 -- r2; }`)

	for _, n := range nm.Nodes {
		switch n.Name {
		case "r1":
			if !n.IsVirtual() {
				t.Error("a node class saying virtual: true must mark the node")
			}
			if !n.IsMaterialised() {
				t.Error("virtual must not decide deployment; deploy: container says it is deployed")
			}
		case "r2":
			if n.IsVirtual() {
				t.Error("a class that says nothing must not mark the node virtual")
			}
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
