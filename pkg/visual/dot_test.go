package visual

import (
	"strings"
	"testing"

	"github.com/cpflat/dot2net/pkg/types"
)

// TestAbbreviateIPAddress documents the current behavior: the feature is
// disabled by the compile-time constant ABBREVIATE_IPADDRESS, so the function
// is a pass-through that never errors, regardless of address/prefix validity.
// If the constant is ever enabled this test will fail and must be revisited.
func TestAbbreviateIPAddress(t *testing.T) {
	if ABBREVIATE_IPADDRESS {
		t.Skip("ABBREVIATE_IPADDRESS enabled; this pass-through test no longer applies")
	}
	tests := []struct {
		name string
		addr string
		plen string
	}{
		{"ipv4 /24", "10.0.0.1", "24"},
		{"ipv4 /16", "10.0.0.1", "16"},
		{"invalid address", "not-an-ip", "24"},
		{"invalid prefix length", "10.0.0.1", "xx"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := abbreviateIPAddress(tt.addr, tt.plen)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.addr {
				t.Errorf("abbreviateIPAddress(%q,%q) = %q, want pass-through %q", tt.addr, tt.plen, got, tt.addr)
			}
		})
	}
}

// TestGetInterfaceAddress covers the success path and the error-message
// construction flagged by CR-027 (wrap the underlying error, include the node
// name) and CR-013 (guard a nil iface.Node with "<unknown>").
func TestGetInterfaceAddress(t *testing.T) {
	layer := &types.Layer{Name: "ip"}

	t.Run("success returns the layer address param", func(t *testing.T) {
		nm := types.NewNetworkModel()
		node := nm.NewNode("r1")
		iface := node.NewInterface("eth0")
		iface.SetRelativeParam(layer.IPAddressReplacer(), "10.0.0.1")

		got, err := getInterfaceAddress(iface, layer)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "10.0.0.1" {
			t.Errorf("got %q, want 10.0.0.1", got)
		}
	})

	t.Run("missing address wraps error with node name", func(t *testing.T) {
		nm := types.NewNetworkModel()
		node := nm.NewNode("r1")
		iface := node.NewInterface("eth0")

		_, err := getInterfaceAddress(iface, layer)
		if err == nil {
			t.Fatalf("expected error for missing address")
		}
		msg := err.Error()
		for _, want := range []string{"node r1", "layer ip", "eth0"} {
			if !strings.Contains(msg, want) {
				t.Errorf("error %q does not contain %q", msg, want)
			}
		}
	})

	t.Run("nil node reports <unknown>", func(t *testing.T) {
		nm := types.NewNetworkModel()
		node := nm.NewNode("r1")
		iface := node.NewInterface("eth0")
		iface.Node = nil // simulate a detached interface

		_, err := getInterfaceAddress(iface, layer)
		if err == nil {
			t.Fatalf("expected error for missing address")
		}
		if !strings.Contains(err.Error(), "<unknown>") {
			t.Errorf("error %q should report <unknown> node", err.Error())
		}
	})
}
