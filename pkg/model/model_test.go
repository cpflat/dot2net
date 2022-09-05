package model

import (
	"fmt"
	"net/netip"
	"os"
	"testing"

	"gonum.org/v1/gonum/graph"
)

func TestLoadConfig(t *testing.T) {
	// dir, _ := os.Getwd()
	// filepath := dir + "/test.yaml"
	filepath := "test.yaml"
	cfg, err := LoadConfig(filepath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", cfg)
}

func TestLoadDot(t *testing.T) {
	dir, _ := os.Getwd()
	filepath := dir + "/test.dot"
	nd, err := NetworkDiagramFromDotFile(filepath)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", nd)
	for _, e := range graph.EdgesOf(nd.Edges()) {
		fmt.Printf("%+v\n", e)
	}
}

func TestGetIPAddrPool(t *testing.T) {
	t.Run("ipv4", func(t *testing.T) {
		poolprefix := netip.MustParsePrefix("10.252.0.0/14")
		bits := 18
		pool, err := getIPAddrPool(poolprefix, bits, 0)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("%v\n", pool)
		if len(pool) != 16 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(pool))
		}
		last := pool[len(pool)-1].String()
		if last != "10.255.192.0/18" {
			t.Errorf("last prefix mismatch %v", last)
		}

		pool, err = getIPAddrPool(poolprefix, bits, 5)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("%v\n", pool)
		if len(pool) != 5 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(pool))
		}
		last = pool[len(pool)-1].String()
		if last != "10.253.0.0/18" {
			t.Errorf("last prefix mismatch %v", last)
		}
	})

	t.Run("ipv6", func(t *testing.T) {
		poolprefix := netip.MustParsePrefix("2001:db8:1232::/45")
		bits := 50
		pool, err := getIPAddrPool(poolprefix, bits, 0)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("%v\n", pool)
		if len(pool) != 32 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(pool))
		}
		last := pool[len(pool)-1].String()
		if last != "2001:db8:1239:c000::/50" {
			t.Errorf("last prefix mismatch %v", last)
		}
	})

}

func TestGetIPAddr(t *testing.T) {
	t.Run("ipv4", func(t *testing.T) {
		prefix := netip.MustParsePrefix("192.0.2.16/28")
		addrs, err := getIPAddr(prefix, 0)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("%v\n", addrs)
		if len(addrs) != 14 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(addrs))
		}
		last := addrs[len(addrs)-1].String()
		if last != "192.0.2.30" {
			t.Errorf("last addr mismatch %v", last)
		}

		addrs, err = getIPAddr(prefix, 9)
		if err != nil {
			t.Fatal(err)
		}
		if len(addrs) != 9 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(addrs))
		}
		last = addrs[len(addrs)-1].String()
		if last != "192.0.2.25" {
			t.Errorf("last addr mismatch %v", last)
		}
	})

	t.Run("ipv6", func(t *testing.T) {
		prefix := netip.MustParsePrefix("2001:db8:1234:abcd:5678:fedc:1111:1120/123")
		addrs, err := getIPAddr(prefix, 0)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("%v\n", addrs)
		if len(addrs) != 31 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(addrs))
		}
		last := addrs[len(addrs)-1].String()
		if last != "2001:db8:1234:abcd:5678:fedc:1111:113f" {
			t.Errorf("last addr mismatch %v", last)
		}

		addrs, err = getIPAddr(prefix, 6)
		if err != nil {
			t.Fatal(err)
		}
		if len(addrs) != 6 {
			t.Errorf("number of generated prefixes mismatch (%v)", len(addrs))
		}
		last = addrs[len(addrs)-1].String()
		if last != "2001:db8:1234:abcd:5678:fedc:1111:1126" {
			t.Errorf("last addr mismatch %v", last)
		}
	})
}
