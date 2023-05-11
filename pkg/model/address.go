package model

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
)

// A netSegment store information for address allocation.
// It corresponds to a network address block.
type netSegment struct {
	prefix  netip.Prefix
	uifaces []*Interface // ip-aware interfaces
	rifaces []*Interface // reserved interfaces (check consistency later)
	raddrs  []netip.Addr // reserved addresses for rifaces
	bound   bool         // network address is bound (determined by reservation) or not
	count   int          // number of unspecified interfaces for address assignment
	bits    int          // default (automatically assigned) prefix length
}

func (seg *netSegment) String() string {
	buf := fmt.Sprintf("prefix: %v, bits: %v, ", seg.prefix.String(), seg.bits)
	buf += fmt.Sprintf("%v free interfaces: [", len(seg.uifaces))
	ifaces := []string{}
	for _, iface := range seg.uifaces {
		ifaces = append(ifaces, iface.String())
	}
	buf += strings.Join(ifaces, ", ")
	buf += fmt.Sprintf("], %v reserved interfaces: [", len(seg.rifaces))
	ifaces = []string{}
	for _, iface := range seg.rifaces {
		ifaces = append(ifaces, iface.String())
	}
	buf += strings.Join(ifaces, ", ")
	buf += "]"
	return buf
}

func (seg *netSegment) checkConnection(conn *Connection, ipspace *IPSpaceDefinition) error {
	if val, ok := conn.GivenIPNetwork(ipspace); ok {
		prefix, err := netip.ParsePrefix(val)
		if err != nil {
			return fmt.Errorf("invalid given ipprefix (%v)", val)
		}
		if seg.bound {
			// check consistency with other reserved connections
			if prefix != seg.prefix {
				return fmt.Errorf("inconsistent specification of ip address (%+v) in a network segment", prefix)
			}
		} else {
			// set segment prefix
			seg.bound = true
			seg.prefix = prefix
		}
	}
	return nil
}

func (seg *netSegment) checkInterface(iface *Interface, ipspace *IPSpaceDefinition) (bool, error) {
	if val, ok := iface.GivenIPAddress(ipspace); ok {
		addr, err := netip.ParseAddr(val)
		if err != nil {
			return false, fmt.Errorf("invalid given ipaddr (%v)", val)
		}
		seg.rifaces = append(seg.rifaces, iface)
		seg.raddrs = append(seg.raddrs, addr)
		seg.bound = true
		return true, nil // ip aware (manually specified)
	} else if iface.ipAware.Contains(ipspace.Name) {
		seg.uifaces = append(seg.uifaces, iface)
		seg.count++
		return true, nil // ip aware (unspecified)
	}
	return false, nil // ip non aware
}

func (seg *netSegment) checkReservedInterfaces() error {
	if seg.bound {
		// check consistency with network prefix reserved by connections
		for _, addr := range seg.raddrs {
			if !seg.prefix.Contains(addr) {
				return fmt.Errorf("inconsistent specification of ip address (%+v) in a network segment", addr)
			}
		}
	} else {
		prev := netip.Prefix{}
		for _, addr := range seg.raddrs {
			prefix, err := addr.Prefix(seg.bits)
			if err != nil {
				return err
			}
			if prev != prefix {
				return fmt.Errorf("inconsistent specification of ip address (%+v) in a network segment", addr)
			}
			prev = prefix
		}
	}
	return nil
}

// A netSegment stores search results of network segments in the network model.
// It also manage address allocation considering reservation.
type netSegments struct {
	pool     *ipPool
	segments []*netSegment
	count    int // number of unbound segments
}

func (segs *netSegments) String() string {
	buf := fmt.Sprintf("pool: [%s]\n%d segments:\n", segs.pool.String(), len(segs.segments))
	tmp := []string{}
	for _, seg := range segs.segments {
		tmp = append(tmp, "- "+seg.String())
	}
	buf += strings.Join(tmp, "\n")
	return buf
}

func searchNetworkSegments(nm *NetworkModel, pool *ipPool, ipspace *IPSpaceDefinition) (*netSegments, error) {
	segs := netSegments{pool: pool}

	checked := mapset.NewSet[*Connection]()
	for _, conn := range nm.Connections {
		// skip connections out of IPSpace
		if !conn.IPSpaces.Contains(ipspace.Name) {
			continue
		}

		// skip connections that is already checked
		if checked.Contains(conn) {
			continue
		}

		seg := netSegment{bits: pool.bits}

		// check connection
		checked.Add(conn)
		// reserve specified network address on connection
		if err := seg.checkConnection(conn, ipspace); err != nil {
			return nil, err
		}

		// search subnet
		todo := []*Interface{conn.Dst, conn.Src} // stack (Last In First Out)
		for len(todo) > 0 {
			// pop iface from todo
			iface := todo[len(todo)-1]
			todo = todo[:len(todo)-1]

			// check interface
			ipaware, err := seg.checkInterface(iface, ipspace)
			if err != nil {
				return nil, err
			} else if !ipaware {
				// ip unaware -> search adjacent interfaces
				for _, nextIf := range iface.Node.Interfaces {

					// pass iface itself
					if nextIf.Name == iface.Name {
						continue
					}

					// skip connections (and end interfaces) out of IPSpace
					if !nextIf.Connection.IPSpaces.Contains(ipspace.Name) {
						continue
					}

					if checked.Contains(nextIf.Connection) {
						// already checked connection, something wrong
						return nil, fmt.Errorf("network segment search algorithm panic")
					}

					// check connection
					checked.Add(nextIf.Connection)
					if err := seg.checkConnection(conn, ipspace); err != nil {
						return nil, err
					}

					// check interface
					ipaware, err := seg.checkInterface(nextIf, ipspace)
					if err != nil {
						return nil, err
					}
					if !ipaware {
						// ip unaware -> search opposite interface (and beyond)
						// add opposite interface to list for further search
						todo = append(todo, nextIf.Opposite)
					}
				}
			}
		}
		// note: reserved addresses are checked after all reserved connections
		// (reserved connections can change prefix length)
		err := seg.checkReservedInterfaces()
		if err != nil {
			return nil, err
		}
		// reserve address block in ipPool
		if seg.bound {
			pool.reservePrefix(seg.prefix)
		} else {
			segs.count++
		}
		segs.segments = append(segs.segments, &seg)

		// sanity check
		if len(seg.rifaces)+len(seg.uifaces) <= 0 {
			fmt.Printf("%+v, src: %s@%s, dst: %s@%s\n", conn, conn.Src.Name, conn.Src.Node.Name, conn.Dst.Name, conn.Dst.Node.Name)
			return nil, fmt.Errorf("searchNetworkSegment panic: no %v-aware interfaces in a segment", ipspace.Name)
		}
	}
	return &segs, nil
}

// An ipPool manage reservation of prefix range.
// It allocate address blocks considering the address reservation.
type ipPool struct {
	prefixRange netip.Prefix
	bits        int
	length      int
	boundIndex  map[int]struct{}
}

func initIPPool(prefixRange netip.Prefix, bits int) (*ipPool, error) {
	pbits := prefixRange.Bits()
	if pbits > bits { // pool range is smaller
		return nil, fmt.Errorf("prefix range %+v is too small for prefix length %+v", prefixRange, bits)
	}

	pool := ipPool{
		prefixRange: prefixRange,
		bits:        bits,
		length:      1 << (bits - pbits),
		boundIndex:  map[int]struct{}{},
	}
	return &pool, nil
}

func (pool *ipPool) String() string {
	return fmt.Sprintf("prefix: %s, bits: %d", pool.prefixRange.String(), pool.bits)
}

func (pool *ipPool) getitem(idx int) (netip.Prefix, error) {
	slice := pool.prefixRange.Addr().AsSlice()
	if idx < 0 {
		idx = pool.length - idx
	}

	// byte_idx to increase values: 1-8 -> 0, 9-16 -> 1, ...
	byte_idx := (pool.bits - 1) / 8
	if byte_idx == len(slice) {
		byte_idx -= 1
	}
	byte_increase := idx << (8 - 1 - (pool.bits-1)%8)
	//byte_increase := int(math.Pow(2, float64(8-bits%8))) * idx
	for byte_idx > 0 { // byte index to modify
		tmp_sum := int(slice[byte_idx]) + byte_increase
		byte_increase = tmp_sum >> 8
		if byte_increase > 0 { // tmp_sum > 256
			slice[byte_idx] = byte(tmp_sum - byte_increase<<8)
			// current_slice[byte_idx] = byte(tmp_sum % 256)
			byte_idx = byte_idx - 1
		} else {
			slice[byte_idx] = byte(tmp_sum)
			break
		}
	}

	new_addr, ok := netip.AddrFromSlice(slice)
	if ok {
		new_prefix := netip.PrefixFrom(new_addr, pool.bits)
		return new_prefix, nil
	} else {
		return netip.Prefix{}, fmt.Errorf("format error in address pool calculation")
	}
}

func (pool *ipPool) prefixToIndex(prefix netip.Prefix) (int, error) {
	if prefix.Bits() != pool.bits {
		return -1, fmt.Errorf("invalid usage, give prefix of default bits")
	}

	topSlice := pool.prefixRange.Addr().AsSlice()
	givenSlice := prefix.Addr().AsSlice()
	cnt := 0
	for i := 0; i < len(topSlice); i++ {
		diff := int(givenSlice[i] - topSlice[i])
		if diff > 0 {
			if pool.bits >= (i+1)*8 {
				cnt = cnt + diff<<(pool.bits-(i+1)*8)
			} else {
				cnt = cnt + diff>>((i+1)*8-pool.bits)
			}
		}
	}
	return cnt, nil
}

func (pool *ipPool) reserveAddr(addr netip.Addr) error {
	prefix, err := addr.Prefix(pool.bits)
	if err != nil {
		// out of pool range
		return nil
	}
	idx, err := pool.prefixToIndex(prefix)
	if err != nil {
		return err
	}
	pool.boundIndex[idx] = struct{}{}
	return nil
}

func (pool *ipPool) reservePrefix(prefix netip.Prefix) error {
	if !pool.prefixRange.Contains(prefix.Addr()) {
		// out of prefixRange (no reservation)
	} else if prefix.Bits() < pool.bits {
		// bind all duplicated address blocks
		prefixes, err := getIPAddrBlocks(prefix, pool.bits, -1)
		if err != nil {
			return err
		}
		for _, subprefix := range prefixes {
			idx, err := pool.prefixToIndex(subprefix)
			if err != nil {
				return err
			}
			pool.boundIndex[idx] = struct{}{}
		}
	} else if prefix.Bits() == pool.bits {
		// bind the same address block
		idx, err := pool.prefixToIndex(prefix)
		if err != nil {
			return err
		}
		pool.boundIndex[idx] = struct{}{}
	} else { // prefix.Bits() > pool.bits
		// bind the address block including the reserved prefix
		newPrefix, err := prefix.Addr().Prefix(pool.bits)
		if err != nil {
			return err
		}
		idx, err := pool.prefixToIndex(newPrefix)
		if err != nil {
			return err
		}
		pool.boundIndex[idx] = struct{}{}
	}
	return nil
}

func (pool *ipPool) getAvailablePrefix(cnt int) ([]netip.Prefix, error) {
	if pool.length-len(pool.boundIndex) < cnt {
		return nil, fmt.Errorf("no enough network prefix in address pool")
	}
	if cnt < 0 {
		cnt = pool.length - len(pool.boundIndex)
	}

	var prefixes = make([]netip.Prefix, 0, cnt)
	for i := 0; i < pool.length; i++ {
		if _, exists := pool.boundIndex[i]; !exists {
			p, err := pool.getitem(i)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, p)
		}
		if len(prefixes) >= cnt {
			break
		}
	}
	return prefixes, nil
}

func getIPAddrBlocks(poolrange netip.Prefix, bits int, cnt int) ([]netip.Prefix, error) {
	pbits := poolrange.Bits()
	err_too_small := fmt.Errorf("poolrange is too small")

	if pbits > bits { // pool range is smaller
		return nil, err_too_small
	} else if pbits == bits {
		if cnt > 1 {
			return nil, err_too_small
		} else {
			return []netip.Prefix{poolrange}, nil
		}
	} else { // pbits < bits
		// calculate number of prefixes to generate
		potential := int(math.Pow(2, float64(bits-pbits)))
		if cnt <= 0 {
			cnt = potential
		} else if cnt > potential {
			return nil, err_too_small
		}
		var pool = make([]netip.Prefix, 0, cnt)

		// add first prefix
		new_prefix := netip.PrefixFrom(poolrange.Addr(), bits)
		pool = append(pool, new_prefix)

		// calculate following prefixes
		current_slice := poolrange.Addr().AsSlice()
		for i := 0; i < cnt-1; i++ { // pool addr index
			byte_idx := bits / 8
			byte_increase := int(math.Pow(2, float64(8-bits%8)))
			for byte_idx > 0 { // byte index to modify
				tmp_sum := int(current_slice[byte_idx]) + byte_increase
				if tmp_sum >= 256 {
					current_slice[byte_idx] = byte(tmp_sum - 256)
					byte_idx = byte_idx - 1
					byte_increase = 1
				} else {
					current_slice[byte_idx] = byte(tmp_sum)
					break
				}
			}
			new_addr, ok := netip.AddrFromSlice(current_slice)
			if ok {
				new_prefix = netip.PrefixFrom(new_addr, bits)
				pool = append(pool, new_prefix)
			} else {
				return pool, fmt.Errorf("format error in address pool calculation")
			}
		}
		return pool, nil
	}
}

func searchIPLoopbacks(nm *NetworkModel, pool *ipPool, ipspace *IPSpaceDefinition) ([]*Node, int, error) {
	// search ip loopbacks
	allLoopbacks := []*Node{}
	cnt := 0
	for _, node := range nm.Nodes {
		// check specified (reserved) loopback address -> reserve
		if val, ok := node.GivenIPLoopback(ipspace); ok {
			addr, err := netip.ParseAddr(val)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid given iploopback (%v)", val)
			}
			pool.reserveAddr(addr)
		} else if node.ipAware.Contains(ipspace.Name) {
			// ip aware -> add the node to list
			// count as node with unspecified loopback
			allLoopbacks = append(allLoopbacks, node)
			cnt++
		}
		// ip non-aware -> do nothing
	}
	return allLoopbacks, cnt, nil
}

func assignIPLoopbacks(cfg *Config, nm *NetworkModel, ipspace *IPSpaceDefinition) error {
	poolrange, err := netip.ParsePrefix(ipspace.LoopbackRange)
	if err != nil {
		return fmt.Errorf("invalid ipspace loopback_range (%v)", ipspace.LoopbackRange)
	}
	pool, err := initIPPool(poolrange, poolrange.Addr().BitLen())
	if err != nil {
		return err
	}
	err = pool.reserveAddr(poolrange.Addr()) // avoid network address
	if err != nil {
		return err
	}
	if poolrange.Addr().Is4() {
		// avoid broadcast address on IPv4
		baddr, err := pool.getitem(-1)
		if err != nil {
			return err
		}
		err = pool.reserveAddr(baddr.Addr())
		if err != nil {
			return err
		}
	}

	allLoopbacks, cnt, err := searchIPLoopbacks(nm, pool, ipspace)
	if err != nil {
		return err
	}
	prefixes, err := pool.getAvailablePrefix(cnt)
	if err != nil {
		return err
	}
	for i, node := range allLoopbacks {
		addr := prefixes[i].Addr()
		node.addNumber(ipspace.IPLoopbackReplacer(), addr.String())
	}

	return nil
}

func searchManagementInterfaces(nm *NetworkModel, pool *ipPool, ipspace *IPSpaceDefinition) ([]*Interface, int, error) {
	cnt := 0
	allInterfaces := []*Interface{}
	for _, node := range nm.Nodes {
		if iface := node.mgmtInterface; iface != nil {
			if val, ok := iface.GivenIPAddress(ipspace); ok {
				addr, err := netip.ParseAddr(val)
				if err != nil {
					return nil, 0, fmt.Errorf("invalid given ipaddress (%v)", val)
				}
				pool.reserveAddr(addr)
			} else if iface.ipAware.Contains(ipspace.Name) {
				allInterfaces = append(allInterfaces, iface)
				cnt++
			}
		}
	}
	return allInterfaces, cnt, nil
}

func assignManagementIPAddresses(cfg *Config, nm *NetworkModel, ipspace *IPSpaceDefinition) error {
	poolrange, err := netip.ParsePrefix(ipspace.AddrRange)
	if err != nil {
		return fmt.Errorf("invalid ipspace range (%v)", ipspace.AddrRange)
	}
	pool, err := initIPPool(poolrange, poolrange.Addr().BitLen())
	if err != nil {
		return err
	}
	err = pool.reserveAddr(poolrange.Addr()) // avoid network address
	if err != nil {
		return err
	}
	if poolrange.Addr().Is4() {
		// avoid broadcast address on IPv4
		baddr, err := pool.getitem(-1)
		if err != nil {
			return err
		}
		err = pool.reserveAddr(baddr.Addr())
		if err != nil {
			return err
		}
	}
	// avoid external gateway address
	// if external gateway address is not given, use first address as default (same as containerlab defaults)
	var gaddr netip.Addr
	if ipspace.ExternalGateway == "" {
		gaddr = poolrange.Addr().Next()
		ipspace.ExternalGateway = gaddr.String()
	} else {
		gaddr, err = netip.ParseAddr(ipspace.ExternalGateway)
		if err != nil {
			return err
		}
	}
	err = pool.reserveAddr(gaddr)
	if err != nil {
		return err
	}

	allInterfaces, cnt, err := searchManagementInterfaces(nm, pool, ipspace)
	if err != nil {
		return err
	}
	prefixes, err := pool.getAvailablePrefix(cnt)
	if err != nil {
		return err
	}
	for i, iface := range allInterfaces {
		addr := prefixes[i].Addr()
		iface.addNumber(ipspace.IPAddressReplacer(), addr.String())
		iface.addNumber(ipspace.IPNetworkReplacer(), poolrange.String())
		iface.addNumber(ipspace.IPPrefixLengthReplacer(), strconv.Itoa(poolrange.Bits()))
	}

	return nil
}

func assignIPAddresses(cfg *Config, nm *NetworkModel, ipspace *IPSpaceDefinition) error {
	poolrange, err := netip.ParsePrefix(ipspace.AddrRange)
	if err != nil {
		return fmt.Errorf("invalid ipspace range (%v)", ipspace.AddrRange)
	}
	pool, err := initIPPool(poolrange, ipspace.DefaultPrefixLength)
	if err != nil {
		return err
	}

	segs, err := searchNetworkSegments(nm, pool, ipspace)
	if err != nil {
		return err
	}
	prefixes, err := segs.pool.getAvailablePrefix(segs.count)
	if err != nil {
		return err
	}
	for _, seg := range segs.segments {
		if !seg.bound {
			if len(prefixes) <= 0 {
				return fmt.Errorf("address reservation panic")
			}
			// pop prefixes
			seg.prefix = prefixes[0]
			prefixes = prefixes[1:]
		}
		addrs, err := getIPAddr(seg.prefix, len(seg.uifaces), seg.raddrs)
		if err != nil {
			return err
		}
		for i, iface := range seg.uifaces {
			iface.addNumber(ipspace.IPAddressReplacer(), addrs[i].String())
			iface.addNumber(ipspace.IPNetworkReplacer(), seg.prefix.String())
			iface.addNumber(ipspace.IPPrefixLengthReplacer(), strconv.Itoa(seg.prefix.Bits()))
		}
		for i, iface := range seg.rifaces {
			iface.addNumber(ipspace.IPAddressReplacer(), seg.raddrs[i].String())
			iface.addNumber(ipspace.IPNetworkReplacer(), seg.prefix.String())
			iface.addNumber(ipspace.IPPrefixLengthReplacer(), strconv.Itoa(seg.prefix.Bits()))
		}
	}

	if len(prefixes) > 0 {
		return fmt.Errorf("address reservation panic: %d prefixes unassigned", len(prefixes))
	}
	return nil
}

func getIPAddr(pool netip.Prefix, cnt int, reserved []netip.Addr) ([]netip.Addr, error) {
	var potential int
	err_too_small := fmt.Errorf("addr pool is too small")

	// calculate number of addresses to generate
	if pool.Addr().Is4() {
		// IPv4: skip network address and broadcast address
		potential = int(math.Pow(2, float64(32-pool.Bits()))) - 2 - len(reserved)
	} else {
		// IPv6: skip network address
		potential = int(math.Pow(2, float64(128-pool.Bits()))) - 1 - len(reserved)
	}
	if cnt <= 0 {
		cnt = potential
	} else if cnt > potential {
		return nil, err_too_small
	}

	reservedMap := map[string]struct{}{}
	for _, addr := range reserved {
		reservedMap[addr.String()] = struct{}{}
	}

	// generate addresses
	var addrs = make([]netip.Addr, 0, cnt)
	current_addr := pool.Addr()
	for len(addrs) < cnt {
		current_addr = current_addr.Next()
		if !pool.Contains(current_addr) {
			return nil, err_too_small
		} else if _, exists := reservedMap[current_addr.String()]; !exists {
			addrs = append(addrs, current_addr)
		}
	}
	return addrs, nil
}

func getASNumber(cfg *Config, cnt int) ([]int, error) {
	var asmin int
	var asmax int
	var asnumbers = make([]int, 0, cnt)
	if cfg.GlobalSettings.ASNumberMin > 0 {
		asmin = cfg.GlobalSettings.ASNumberMin
		if cfg.GlobalSettings.ASNumberMax > 0 {
			asmax = cfg.GlobalSettings.ASNumberMax
		} else {
			asmax = 65535
		}
		if asmax <= asmin {
			return nil, fmt.Errorf("invalid AS range (%d - %d) specified in configuration", asmin, asmax)
		}
		if (asmax - asmin + 1) < cnt {
			return nil, fmt.Errorf("requested %d AS numbers, but specified AS range has only %d numbers", cnt, asmax-asmin+1)
		}
	} else {
		// if ASNumberMin/Max not given, automatically use Private AS numbers
		if cnt <= 535 {
			asmin = 65001
			asmax = 65535
		} else if cnt <= 1024 {
			asmin = 64512
			asmax = 65535
		} else {
			// currently returns error
			return nil, fmt.Errorf("requested more than 1024 private AS numbers")
		}
	}
	for i := 0; i < cnt; i++ {
		asnumbers = append(asnumbers, asmin+i)
	}
	return asnumbers, nil
}
