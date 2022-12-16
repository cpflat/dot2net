package model

import (
	"net/netip"
	"testing"
)

func TestGetIPAddrBlocks(t *testing.T) {
	t.Run("ipv4", func(t *testing.T) {
		poolprefix := netip.MustParsePrefix("10.252.0.0/14")
		bits := 18
		pool, err := getIPAddrBlocks(poolprefix, bits, 0)
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

		pool, err = getIPAddrBlocks(poolprefix, bits, 5)
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
		pool, err := getIPAddrBlocks(poolprefix, bits, 0)
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

func TestIPAddrPool(t *testing.T) {
	t.Run("ipv4", func(t *testing.T) {
		poolprefix := netip.MustParsePrefix("10.252.0.0/14")
		pool, err := initIPPool(poolprefix, 16)
		if err != nil {
			t.Fatal(err)
		}
		prefixes, err := pool.getAvailablePrefix(-1)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("A: %+v\n", prefixes)
		last := prefixes[len(prefixes)-1].String()
		target := "10.255.0.0/16"
		if last != target {
			t.Errorf("last prefix mismatch %v, %v", last, target)
		}

		pool, err = initIPPool(poolprefix, 16)
		if err != nil {
			t.Fatal(err)
		}
		reserveAddr := netip.MustParseAddr("10.252.127.14")
		pool.reserveAddr(reserveAddr)
		prefixes, err = pool.getAvailablePrefix(-1)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("B: %+v\n", prefixes)
		first := prefixes[0].String()
		target = "10.253.0.0/16"
		if first != target {
			t.Errorf("first prefix mismatch %v, %v", first, target)
		}

		poolprefix = netip.MustParsePrefix("10.0.0.0/8")
		pool, err = initIPPool(poolprefix, 12)
		if err != nil {
			t.Fatal(err)
		}
		pool.reservePrefix(netip.MustParsePrefix("10.0.0.0/10"))
		pool.reservePrefix(netip.MustParsePrefix("10.64.0.0/10"))
		pool.reservePrefix(netip.MustParsePrefix("10.16.0.0/12"))
		pool.reservePrefix(netip.MustParsePrefix("10.129.254.0/24"))
		prefixes, err = pool.getAvailablePrefix(-1)
		if err != nil {
			t.Fatal(err)
		}
		// fmt.Printf("C: %+v\n", prefixes)
		first = prefixes[0].String()
		target = "10.144.0.0/12"
		if first != target {
			t.Errorf("first prefix mismatch %v, %v", first, target)
		}
	})
}

func TestGetIPAddr(t *testing.T) {
	empty := []netip.Addr{}
	t.Run("ipv4", func(t *testing.T) {
		prefix := netip.MustParsePrefix("192.0.2.16/28")
		addrs, err := getIPAddr(prefix, 0, empty)
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

		addrs, err = getIPAddr(prefix, 9, empty)
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
		addrs, err := getIPAddr(prefix, 0, empty)
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

		addrs, err = getIPAddr(prefix, 6, empty)
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
