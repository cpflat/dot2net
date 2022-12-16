package model

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
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

func (seg *netSegment) checkConnection(conn *Connection) error {
	if val, ok := conn.GivenIPNetwork(); ok {
		prefix, err := netip.ParsePrefix(val)
		if err != nil {
			return err
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

func (seg *netSegment) checkInterface(iface *Interface, layer string) (bool, error) {
	if val, ok := iface.GivenIPAddress(); ok {
		addr, err := netip.ParseAddr(val)
		if err != nil {
			return false, err
		}
		seg.rifaces = append(seg.rifaces, iface)
		seg.raddrs = append(seg.raddrs, addr)
		seg.bound = true
		return true, nil // ip aware (manually specified)
	} else if iface.hasNumberKey(layer) {
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
	segments []netSegment
	count    int // number of unbound segments
}

func searchNetworkSegments(nm *NetworkModel, pool *ipPool, layer string) (*netSegments, error) {
	segs := netSegments{pool: pool}

	checked := map[*Connection]struct{}{} // set alternative
	for i, conn := range nm.Connections {
		// skip connections that is already checked
		if _, ok := checked[&nm.Connections[i]]; ok {
			break
		}

		seg := netSegment{bits: pool.bits}

		// check connection
		checked[&nm.Connections[i]] = struct{}{}
		// reserve specified network address on connection
		if err := seg.checkConnection(&conn); err != nil {
			return nil, err
		}

		// search subnet
		todo := []*Interface{conn.Dst, conn.Src} // stack (Last In First Out)
		for len(todo) > 0 {
			// pop iface from todo
			iface := todo[len(todo)-1]
			todo = todo[:len(todo)-1]

			// check interface
			ipaware, err := seg.checkInterface(iface, layer)
			if err != nil {
				return nil, err
			} else if !ipaware {
				// ip unaware -> search adjacent interfaces
				for _, nextIf := range iface.Node.Interfaces {

					if _, ok := checked[nextIf.Connection]; ok {
						// already checked connection, something wrong
						return nil, fmt.Errorf("network segment search algorithm panic")
					}

					// pass iface itself
					if nextIf.Name == iface.Name {
						continue
					}

					// check connection
					checked[nextIf.Connection] = struct{}{}
					if err := seg.checkConnection(&conn); err != nil {
						return nil, err
					}

					// check interface
					ipaware, err := seg.checkInterface(&nextIf, layer)
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
		segs.segments = append(segs.segments, seg)
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

func (pool *ipPool) getitem(idx int) (netip.Prefix, error) {
	slice := pool.prefixRange.Addr().AsSlice()

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

func searchIPLoopbacks(nm *NetworkModel, pool *ipPool, layer string) ([]*Node, int, error) {
	// search ip loopbacks
	allLoopbacks := []*Node{}
	cnt := 0
	for i, node := range nm.Nodes {
		// check specified (reserved) loopback address -> reserve
		if val, ok := node.GivenIPLoopback(); ok {
			addr, err := netip.ParseAddr(val)
			if err != nil {
				return nil, 0, err
			}
			pool.reserveAddr(addr)
		} else if node.HasIPLoopback() {
			// ip aware -> add the node to list
			// count as node with unspecified loopback
			allLoopbacks = append(allLoopbacks, &nm.Nodes[i])
			cnt++
		}
		// ip non-aware -> do nothing
	}
	return allLoopbacks, cnt, nil
}

func assignIPLoopbacks(cfg *Config, nm *NetworkModel, layer string) (*NetworkModel, error) {
	poolrange, err := netip.ParsePrefix(cfg.GlobalSettings.IPLoopbackRange)
	if err != nil {
		return nm, err
	}
	pool, err := initIPPool(poolrange, poolrange.Addr().BitLen())
	if err != nil {
		return nm, err
	}

	allLoopbacks, cnt, err := searchIPLoopbacks(nm, pool, layer)
	if err != nil {
		return nm, err
	}
	prefixes, err := pool.getAvailablePrefix(cnt)
	if err != nil {
		return nm, err
	}
	for i, node := range allLoopbacks {
		addr := prefixes[i].Addr()
		node.addNumber(NumberReplacerIPLoopback, addr.String())
	}

	return nm, nil
}

//func searchSubNetworks(nm *NetworkModel, pool *ipPool, layer string) ([][]*Interface, []bool, []int, error) {
//	allNetworkInterfaces := [][]*Interface{}
//	bounds := []bool{}
//	counts := []int{}
//	checked := map[*Connection]struct{}{} // set alternative
//	for i, conn := range nm.Connections {
//		// skip connections that is already checked
//		if _, ok := checked[&nm.Connections[i]]; ok {
//			break
//		}
//
//		bound := false
//		cnt := 0
//		checked[&nm.Connections[i]] = struct{}{}
//
//		// reserve specified network address on connection
//		if val, ok := conn.GivenIPNetwork(); ok {
//			prefix, err := netip.ParsePrefix(val)
//			if err != nil {
//				return nil, nil, err
//			}
//			err = pool.reservePrefix(prefix)
//			if err != nil {
//				return nil, nil, err
//			}
//		}
//
//		// search subnet
//		networkInterfaces := []*Interface{}
//		todo := []*Interface{conn.Dst, conn.Src} // stack (Last In First Out)
//		for len(todo) > 0 {
//			// pop iface from todo
//			iface := todo[len(todo)-1]
//			todo = todo[:len(todo)-1]
//
//			if val, ok := iface.GivenIPAddress(); ok {
//				// specified address (ip aware) -> network ends
//				networkInterfaces = append(networkInterfaces, iface)
//				// reserve specified IP address on Interface
//				addr, err := netip.ParseAddr(val)
//				if err != nil {
//					return nil, nil, err
//				}
//				err = pool.reserveAddr(addr)
//				if err != nil {
//					return nil, nil, err
//				}
//				bound = true
//			} else if iface.hasNumberKey(layer) {
//				// ip aware -> network ends
//				// count as interface without specified ip address
//				networkInterfaces = append(networkInterfaces, iface)
//				cnt ++
//			} else {
//				// ip unaware -> search adjacent interfaces
//				for _, nextIf := range iface.Node.Interfaces {
//					if _, ok := checked[nextIf.Connection]; ok {
//						// already checked network, something wrong
//						return nil, nil, fmt.Errorf("Subnetwork search algorithm panic")
//					}
//					checked[nextIf.Connection] = struct{}{}
//					if nextIf.Name == iface.Name {
//						// ignore iface itself
//					} else if val, ok := iface.GivenIPAddress(); ok {
//						// specified address (ip aware) -> network ends
//						networkInterfaces = append(networkInterfaces, iface)
//						// reserve specified IP address on Interface
//						addr, err := netip.ParseAddr(val)
//						if err != nil {
//							return nil, nil, err
//						}
//						err = pool.reserveAddr(addr)
//						if err != nil {
//							return nil, nil, err
//						}
//						bound = true
//					} else if nextIf.IsIPAware() {
//						// ip aware -> network ends
//						networkInterfaces = append(networkInterfaces, iface)
//						// count as interface without specified ip address
//						cnt ++
//					} else {
//						// ip unaware -> search adjacent connection
//						oppIf := nextIf.Opposite
//						todo = append(todo, oppIf)
//					}
//				}
//			}
//		}
//		allNetworkInterfaces = append(allNetworkInterfaces, networkInterfaces)
//		bounds = append(bounds, bound)
//		counts = append(counts, cnt)
//	}
//	return allNetworkInterfaces, bounds, counts, nil
//}

func assignIPAddresses(cfg *Config, nm *NetworkModel, layer string) (*NetworkModel, error) {
	poolrange, err := netip.ParsePrefix(cfg.GlobalSettings.IPAddrPool)
	if err != nil {
		return nm, err
	}
	pool, err := initIPPool(poolrange, cfg.GlobalSettings.IPNetPrefixLength)
	if err != nil {
		return nm, err
	}

	segs, err := searchNetworkSegments(nm, pool, layer)
	if err != nil {
		return nil, err
	}
	prefixes, err := segs.pool.getAvailablePrefix(segs.count)
	if err != nil {
		return nm, err
	}
	for _, seg := range segs.segments {
		if !seg.bound {
			if len(prefixes) <= 0 {
				return nil, fmt.Errorf("address reservation panic")
			}
			// pop prefixes
			seg.prefix = prefixes[0]
			prefixes = prefixes[1:]
		}
		addrs, err := getIPAddr(seg.prefix, len(seg.uifaces), seg.raddrs)
		// TODO avoid reserved addrs
		if err != nil {
			return nil, err
		}
		for i, iface := range seg.uifaces {
			iface.addNumber(NumberReplacerIPAddress, addrs[i].String())
			iface.addNumber(NumberReplacerIPNetwork, seg.prefix.String())
			iface.addNumber(NumberReplacerIPPrefixLength, strconv.Itoa(seg.prefix.Bits()))
		}
		for i, iface := range seg.rifaces {
			iface.addNumber(NumberReplacerIPAddress, seg.raddrs[i].String())
			iface.addNumber(NumberReplacerIPNetwork, seg.prefix.String())
			iface.addNumber(NumberReplacerIPPrefixLength, strconv.Itoa(seg.prefix.Bits()))
		}
	}

	if len(prefixes) > 0 {
		return nil, fmt.Errorf("address reservation panic")
	}
	return nm, nil
}

//func getIPAddrPool(poolrange netip.Prefix, bits int, cnt int) ([]netip.Prefix, error) {
//	pbits := poolrange.Bits()
//	err_too_small := fmt.Errorf("IPAddrPoolRange is too small")
//
//	if pbits > bits { // pool range is smaller
//		return nil, err_too_small
//	} else if pbits == bits {
//		if cnt > 1 {
//			return nil, err_too_small
//		} else {
//			return []netip.Prefix{poolrange}, nil
//		}
//	} else { // pbits < bits
//		// calculate number of prefixes to generate
//		potential := int(math.Pow(2, float64(bits-pbits)))
//		if cnt <= 0 {
//			cnt = potential
//		} else if cnt > potential {
//			return nil, err_too_small
//		}
//		var pool = make([]netip.Prefix, 0, cnt)
//
//		// add first prefix
//		new_prefix := netip.PrefixFrom(poolrange.Addr(), bits)
//		pool = append(pool, new_prefix)
//
//		// calculate following prefixes
//		current_slice := poolrange.Addr().AsSlice()
//		for i := 0; i < cnt-1; i++ { // pool addr index
//			byte_idx := bits / 8
//			byte_increase := int(math.Pow(2, float64(8-bits%8)))
//			for byte_idx > 0 { // byte index to modify
//				tmp_sum := int(current_slice[byte_idx]) + byte_increase
//				if tmp_sum >= 256 {
//					current_slice[byte_idx] = byte(tmp_sum - 256)
//					byte_idx = byte_idx - 1
//					byte_increase = 1
//				} else {
//					current_slice[byte_idx] = byte(tmp_sum)
//					break
//				}
//			}
//			new_addr, ok := netip.AddrFromSlice(current_slice)
//			if ok {
//				new_prefix = netip.PrefixFrom(new_addr, bits)
//				pool = append(pool, new_prefix)
//			} else {
//				return pool, fmt.Errorf("format error in address pool calculation")
//			}
//		}
//		return pool, nil
//	}
//
//}

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

func getASNumber(cnt int) ([]int, error) {
	var asnumbers = make([]int, 0, cnt)
	if cnt <= 535 {
		for i := 0; i < cnt; i++ {
			asnumbers = append(asnumbers, 65001+i)
		}
	} else if cnt <= 1024 {
		for i := 0; i < cnt; i++ {
			asnumbers = append(asnumbers, 64512+i)
		}
	} else { // cnt > 1024
		// currently returns error
		return nil, fmt.Errorf("requested more than 1024 private AS numbers")
	}
	return asnumbers, nil
}
