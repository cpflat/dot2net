package example_test

import (
	"strings"
	"testing"
)

// The assert module exists because the golden tests cannot see a class that is
// declared but never applied: it contributes nothing to the output, so the
// expected files simply record its absence. These cases pin the module's own
// behaviour, in both directions.
func TestAssertModule(t *testing.T) {
	const dot = `digraph {
		r1 [xlabel="router"]; r2 [xlabel="router"];
		hub1 [xlabel="hub"];
		r1 -> hub1 [dir="none", label="segment#used_seg"];
		r2 -> hub1 [dir="none", label="segment#used_seg"];
	}`

	// head holds everything the cases share; each case appends its own classes.
	const head = `
module: [assert]

layer:
  - name: ip
    default_connect: true
    policy:
      - name: ip
        range: 10.0.0.0/16
        prefix: 24
`

	// The segment class the topology really attaches, asserted. Cases that need
	// a satisfied assertion include this so that the "nothing is asserted"
	// error does not mask what they are testing.
	const usedSeg = `
segmentclass:
  - name: used_seg
    layer: ip
    values:
      assert_used: "true"
`

	const plainNodes = `
nodeclass:
  - name: router
    interface_policy: [ip]
  - name: hub
`

	tests := []struct {
		name   string
		yaml   string
		errMsg string // empty means the build must succeed
	}{
		{
			name: "asserted class is applied",
			yaml: head + plainNodes + usedSeg,
		},
		{
			// This is example/vlan_multihost's failure: a segment class that no
			// relational label attaches. Without the module it passes silently.
			name: "asserted segment class is never attached",
			yaml: head + plainNodes + usedSeg + `
  - name: orphan_seg
    layer: ip
    values:
      assert_used: "true"
`,
			errMsg: "declared with assert_used but never applied to any object: segmentclass orphan_seg",
		},
		{
			name: "asserted node class is never used in the topology",
			yaml: head + usedSeg + `
nodeclass:
  - name: router
    interface_policy: [ip]
  - name: hub
  - name: unused_node
    values:
      assert_used: "true"
`,
			errMsg: "never applied to any object: nodeclass unused_node",
		},
		{
			name: "an unapplied class without the flag is fine",
			yaml: head + plainNodes + usedSeg + `
  - name: orphan_seg
    layer: ip
`,
		},
		{
			name: "the flag must be a boolean",
			yaml: head + plainNodes + `
segmentclass:
  - name: used_seg
    layer: ip
    values:
      assert_used: "yes please"
`,
			errMsg: "assert_used must be a boolean",
		},
		{
			name: "loading the module without asserting anything is an error",
			yaml: head + plainNodes + `
segmentclass:
  - name: used_seg
    layer: ip
`,
			errMsg: "no class declares assert_used",
		},
		{
			// Every network class is applied to the network model, so the
			// assertion could only ever pass. It warns rather than failing:
			// the declaration is harmless, it just must not read as coverage.
			name: "asserting a network class only warns",
			yaml: head + plainNodes + usedSeg + `
networkclass:
  - name: _default
    values:
      assert_used: "true"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildInDir(t, dot, tt.yaml)

			if tt.errMsg == "" {
				if err != nil {
					t.Fatalf("expected success, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tt.errMsg)
			}
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("unexpected error:\n  got:      %v\n  expected to contain: %s", err, tt.errMsg)
			}
		})
	}
}
