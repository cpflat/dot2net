package model

import (
	"net/netip"
	"testing"
)

func TestGetIPAddrBlocks(t *testing.T) {
	t.Run("ipv4", func(t *testing.T) {
		poolprefix := netip.MustParsePrefix("10.252.0.0/14")
		bits := 18
		pool, err := getIPAddrBlocks(poolprefix, bits, 0, 0)
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

		pool, err = getIPAddrBlocks(poolprefix, bits, 5, 0)
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
		pool, err := getIPAddrBlocks(poolprefix, bits, 0, 0)
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
		pool, err := initIPPool(poolprefix, 16, 0)
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

		pool, err = initIPPool(poolprefix, 16, 0)
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
		pool, err = initIPPool(poolprefix, 12, 0)
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
		addrs, err := getIPAddr(prefix, 0, empty, 0)
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

		addrs, err = getIPAddr(prefix, 9, empty, 0)
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
		addrs, err := getIPAddr(prefix, 0, empty, 0)
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

		addrs, err = getIPAddr(prefix, 6, empty, 0)
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

func TestGetItem(t *testing.T) {
	tests := []struct {
		name    string
		pool    string // pool prefix range
		bits    int    // prefix length to allocate
		idx     int
		want    string // expected prefix string; ignored when wantErr
		wantErr bool
	}{
		{name: "/24 from /8: idx 0", pool: "10.0.0.0/8", bits: 24, idx: 0, want: "10.0.0.0/24"},
		{name: "/24 from /8: idx 1", pool: "10.0.0.0/8", bits: 24, idx: 1, want: "10.0.1.0/24"},
		{name: "/24 from /8: idx 256 (carry into 2nd octet)", pool: "10.0.0.0/8", bits: 24, idx: 256, want: "10.1.0.0/24"},
		// CR-008 regression: for bits<=8 the carry loop (byte_idx>0) never ran,
		// so idx was ignored and every index returned the pool base.
		{name: "/8 from /0: bits<=8 must use idx", pool: "0.0.0.0/0", bits: 8, idx: 5, want: "5.0.0.0/8"},
		// CR-008 regression: a carry reaching the most significant byte was dropped
		// because the loop stopped at byte_idx>0 without updating byte 0.
		{name: "/16 from /0: carry into most significant byte", pool: "0.0.0.0/0", bits: 16, idx: 256, want: "1.0.0.0/16"},
		// carry beyond the most significant byte -> out of range error
		{name: "/8 from /0: overflow beyond top byte errors", pool: "0.0.0.0/0", bits: 8, idx: 256, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := initIPPool(netip.MustParsePrefix(tt.pool), tt.bits, 0)
			if err != nil {
				t.Fatal(err)
			}
			got, err := pool.getitem(tt.idx)
			if tt.wantErr {
				if err == nil {
					t.Errorf("getitem(%d) expected error, got %v", tt.idx, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("getitem(%d) unexpected error: %v", tt.idx, err)
			}
			if got.String() != tt.want {
				t.Errorf("getitem(%d) = %v, want %v", tt.idx, got.String(), tt.want)
			}
		})
	}
}

func TestPrefixToIndex(t *testing.T) {
	// getitem and prefixToIndex must be inverse of each other (CR-017).
	pool, err := initIPPool(netip.MustParsePrefix("10.0.0.0/8"), 24, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, idx := range []int{0, 1, 5, 256, 1000, 65535} {
		p, err := pool.getitem(idx)
		if err != nil {
			t.Fatalf("getitem(%d): %v", idx, err)
		}
		got, err := pool.prefixToIndex(p)
		if err != nil {
			t.Fatalf("prefixToIndex(%v): %v", p, err)
		}
		if got != idx {
			t.Errorf("round-trip idx %d -> %v -> %d", idx, p, got)
		}
	}

	// A prefix below the pool range is an error, not a bogus index (CR-017).
	pool2, err := initIPPool(netip.MustParsePrefix("10.255.0.0/24"), 32, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool2.prefixToIndex(netip.MustParsePrefix("10.254.255.255/32")); err == nil {
		t.Error("prefixToIndex of a below-range prefix should error")
	}
}

func TestAddressEnumerationCap(t *testing.T) {
	// A large IPv6 pool expanded fully (cnt<=0) must be capped by maxCount rather
	// than trying to enumerate 2^64 addresses (CR-019). This must return promptly.
	addrs, err := getIPAddr(netip.MustParsePrefix("2001:db8::/64"), 0, []netip.Addr{}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 5 {
		t.Errorf("IPv6 getIPAddr cap: got %d addresses, want 5", len(addrs))
	}

	// Full block enumeration is likewise capped by maxCount.
	blocks, err := getIPAddrBlocks(netip.MustParsePrefix("2001:db8::/32"), 64, -1, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 7 {
		t.Errorf("getIPAddrBlocks cap: got %d blocks, want 7", len(blocks))
	}

	// maxCount<=0 falls back to the default.
	if got := resolveMaxAddressCount(0); got != DefaultMaxAddressCount {
		t.Errorf("resolveMaxAddressCount(0) = %d, want %d", got, DefaultMaxAddressCount)
	}
}
