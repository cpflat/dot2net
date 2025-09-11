package model

import (
	"fmt"
	"math"
	"net/netip"
	"strconv"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"

	"github.com/cpflat/dot2net/pkg/types"
)

// An ipPool manage reservation of prefix range.
// It allocate address blocks considering the address reservation.
type ipPool struct {
	prefixRange   netip.Prefix
	bits          int
	availableBits int
	// length      int
	boundIndex   map[int]struct{}
	segments     []*netSegment
	n_unassigned int
}

func initIPPool(prefixRange netip.Prefix, bits int) (*ipPool, error) {
	pbits := prefixRange.Bits()
	if pbits > bits { // pool range is smaller
		return nil, fmt.Errorf("prefix range %+v is too small for prefixes of length %+v", prefixRange, bits)
	}

	pool := ipPool{
		prefixRange:   prefixRange,
		bits:          bits,
		availableBits: bits - pbits,
		//length:      1 << (bits - pbits),
		boundIndex: map[int]struct{}{},
	}
	return &pool, nil
}

func (pool *ipPool) String() string {
	return fmt.Sprintf("prefix: %s, bits: %d", pool.prefixRange.String(), pool.bits)
}

func (pool *ipPool) isEnough(cnt int) bool {
	return (cnt >> pool.availableBits) == 0
}

func (pool *ipPool) getitem(idx int) (netip.Prefix, error) {
	slice := pool.prefixRange.Addr().AsSlice()
	// if idx < 0 {
	// 	idx = pool.length - idx
	// }

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
	required := cnt + len(pool.boundIndex)
	if !pool.isEnough(required) {
		return nil, fmt.Errorf("no enough network prefix in address pool (%d required)", required)
	}

	var prefixes = make([]netip.Prefix, 0, cnt)
	for i := 0; i < required; i++ {
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

// A netSegment store information for address allocation.
// It corresponds to a network address block.
type netSegment struct {
	prefix  netip.Prefix
	uifaces []*types.Interface // ip-aware interfaces
	rifaces []*types.Interface // reserved interfaces (check consistency later)
	raddrs  []netip.Addr       // reserved addresses for rifaces
	bound   bool               // network address is bound (determined by reservation) or not
	count   int                // number of unspecified interfaces for address assignment
	bits    int                // default (automatically assigned) prefix length
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

func (seg *netSegment) Interfaces() []*types.Interface {
	return append(seg.uifaces, seg.rifaces...)
}

func (seg *netSegment) checkConnection(conn *types.Connection, layer *types.Layer) error {
	if val, ok := conn.GivenIPNetwork(layer); ok {
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

func (seg *netSegment) checkInterface(iface *types.Interface, layer *types.Layer) error {
	if val, ok := iface.GivenIPAddress(layer); ok {
		addr, err := netip.ParseAddr(val)
		if err != nil {
			return fmt.Errorf("invalid given ipaddr (%v)", val)
		}
		seg.rifaces = append(seg.rifaces, iface)
		seg.raddrs = append(seg.raddrs, addr)
		seg.bound = true
	} else {
		seg.uifaces = append(seg.uifaces, iface)
		seg.count++
	}
	return nil
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

func searchSegments(nm *types.NetworkModel, layer *types.Layer, verbose bool) ([]*types.NetworkSegment, error) {
	if verbose {
		fmt.Printf("search segments on layer %+v\n", layer.Name)
	}
	segs := []*types.NetworkSegment{}

	checked := mapset.NewSet[*types.Connection]()
	for _, conn := range nm.Connections {
		// skip connections out of layer
		if !conn.Layers.Contains(layer.Name) {
			continue
		}

		// skip connections that is already checked
		if checked.Contains(conn) {
			continue
		}

		// init segment
		//seg := &types.NetworkSegment{}
		seg := types.NewNetworkSegment()

		if verbose {
			fmt.Printf("search start with connection %s\n", conn)
		}
		checked.Add(conn)
		seg.Connections = append(seg.Connections, conn)

		// search subnet
		todo := []*types.Interface{conn.Dst, conn.Src} // stack (Last In First Out)
		for len(todo) > 0 {
			// pop iface from todo
			iface := todo[len(todo)-1]
			todo = todo[:len(todo)-1]

			if iface.AwareLayer(layer.Name) {
				// aware -> search stop, add the interface to segment
				seg.Interfaces = append(seg.Interfaces, iface)
			} else {
				// unaware -> search adjacent interfaces
				for _, nextIf := range iface.Node.Interfaces {
					// pass iface itself
					if nextIf.Name == iface.Name {
						continue
					}

					tmpconn := nextIf.Connection

					// skip connections (and end interfaces) out of layer
					if !tmpconn.Layers.Contains(layer.Name) {
						continue
					}

					if checked.Contains(tmpconn) {
						// already checked connection, may be caused on networks with closed paths
						continue
					}

					checked.Add(tmpconn)
					seg.Connections = append(seg.Connections, tmpconn)
					if verbose {
						fmt.Printf("check next connection %s\n", tmpconn)
					}

					// check interface
					if nextIf.AwareLayer(layer.Name) {
						// aware -> check opposite interface, but end searching
						seg.Interfaces = append(seg.Interfaces, nextIf)
						seg.Interfaces = append(seg.Interfaces, nextIf.Opposite)
					} else {
						// unaware -> search opposite interface (and beyond)
						// add opposite interface to list for further search
						todo = append(todo, nextIf.Opposite)
					}
				}
			}
		}
		if verbose {
			fmt.Printf("determine segment: %+v\n", seg)
		}
		segs = append(segs, seg)
	}
	return segs, nil
}

func setSegmentLabels(cfg *types.Config, segs []*types.NetworkSegment, layer *types.Layer) error {
	for _, seg := range segs {
		scNames := mapset.NewSet[string]()
		for _, conn := range seg.Connections {
			for _, rlabel := range conn.RelationalClassLabels() {
				if rlabel.ClassType == types.ClassTypeSegment {
					sc, ok := cfg.SegmentClassByName(rlabel.Name)
					if !ok {
						return fmt.Errorf("unknown segment class (%v)", rlabel.Name)
					}
					if sc.Layer == layer.Name {
						if !scNames.Contains(rlabel.Name) {
							scNames.Add(rlabel.Name)
						}
					}
				}
			}
		}
		for _, name := range scNames.ToSlice() {
			seg.AddClassLabels(name)
		}
	}
	return nil
}

func setNeighbors(segs []*types.NetworkSegment, layer *types.Layer) {
	for _, seg := range segs {
		for _, iface := range seg.Interfaces {
			iface.Neighbors[layer.Name] = []*types.Neighbor{}
			for _, n := range seg.Interfaces {
				if iface != n {
					iface.AddNeighbor(n, layer.Name)
				}
			}
		}
	}
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

func searchIPLoopbacks(nm *types.NetworkModel, pool *ipPool, layer *types.Layer) ([]*types.Node, int, error) {
	// search ip loopbacks
	allLoopbacks := []*types.Node{}
	cnt := 0
	for _, node := range nm.Nodes {
		// check specified (reserved) loopback address -> reserve
		if val, ok := node.GivenIPLoopback(layer); ok {
			addr, err := netip.ParseAddr(val)
			if err != nil {
				return nil, 0, fmt.Errorf("invalid given iploopback (%v)", val)
			}
			pool.reserveAddr(addr)
		} else if node.AwareLayer(layer.Name) {
			// ip aware -> add the node to list
			// count as node with unspecified loopback
			allLoopbacks = append(allLoopbacks, node)
			cnt++
		}
		// ip non-aware -> do nothing
	}
	return allLoopbacks, cnt, nil
}

func assignIPLoopbacks(nm *types.NetworkModel, layer *types.Layer) error {
	poolmap := map[string]*ipPool{}
	for _, policy := range layer.LoopbackPolicy {
		poolrange, err := netip.ParsePrefix(policy.AddrRange)
		if err != nil {
			return fmt.Errorf("invalid range (%v) for policy (%v)", policy.AddrRange, policy.Name)
		}
		bits := poolrange.Addr().BitLen() // always 32 or 128
		pool, err := initIPPool(poolrange, bits)
		if err != nil {
			return err
		}
		poolmap[policy.Name] = pool
	}
	if len(poolmap) == 0 {
		return nil
	}

	for _, pool := range poolmap {
		// avoid network address
		err := pool.reserveAddr(pool.prefixRange.Addr())
		if err != nil {
			return err
		}
		// avoid broadcast address on IPv4
		if pool.prefixRange.Addr().Is4() {
			baddr, err := pool.getitem(-1)
			if err != nil {
				return err
			}
			err = pool.reserveAddr(baddr.Addr())
			if err != nil {
				return err
			}
		}

		allLoopbacks, cnt, err := searchIPLoopbacks(nm, pool, layer)
		if err != nil {
			return err
		}
		prefixes, err := pool.getAvailablePrefix(cnt)
		if err != nil {
			return err
		}
		for i, node := range allLoopbacks {
			addr := prefixes[i].Addr()
			node.AddParam(layer.IPLoopbackReplacer(), addr.String())
		}
	}

	return nil
}

func searchManagementInterfaces(nm *types.NetworkModel, pool *ipPool, layer *types.ManagementLayer) ([]*types.Interface, int, error) {
	cnt := 0
	allInterfaces := []*types.Interface{}
	for _, node := range nm.Nodes {
		if iface := node.GetManagementInterface(); iface != nil {
			if val, ok := iface.GivenIPAddress(layer); ok {
				addr, err := netip.ParseAddr(val)
				if err != nil {
					return nil, 0, fmt.Errorf("invalid given ipaddress (%v)", val)
				}
				pool.reserveAddr(addr)
			} else {
				allInterfaces = append(allInterfaces, iface)
				cnt++
			}
		}
	}
	return allInterfaces, cnt, nil
}

func assignManagementIPAddresses(cfg *types.Config, nm *types.NetworkModel) error {
	mlayer := &cfg.ManagementLayer
	poolrange, err := netip.ParsePrefix(mlayer.AddrRange)
	if err != nil {
		return fmt.Errorf("invalid range (%v) for management layer", mlayer.AddrRange)
	}
	bits := poolrange.Addr().BitLen()
	pool, err := initIPPool(poolrange, bits)
	if err != nil {
		return err
	}

	// avoid network address
	err = pool.reserveAddr(pool.prefixRange.Addr())
	if err != nil {
		return err
	}
	// avoid broadcast address on IPv4
	if pool.prefixRange.Addr().Is4() {
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
	if mlayer.ExternalGateway == "" {
		gaddr = poolrange.Addr().Next()
		mlayer.ExternalGateway = gaddr.String()
	} else {
		gaddr, err = netip.ParseAddr(mlayer.ExternalGateway)
		if err != nil {
			return err
		}
	}
	err = pool.reserveAddr(gaddr)
	if err != nil {
		return err
	}

	allInterfaces, cnt, err := searchManagementInterfaces(nm, pool, mlayer)
	if err != nil {
		return err
	}
	prefixes, err := pool.getAvailablePrefix(cnt)
	if err != nil {
		return err
	}
	for i, iface := range allInterfaces {
		addr := prefixes[i].Addr()
		iface.AddParam(mlayer.IPAddressReplacer(), addr.String())
		iface.AddParam(mlayer.IPNetworkReplacer(), poolrange.String())
		iface.AddParam(mlayer.IPPrefixLengthReplacer(), strconv.Itoa(poolrange.Bits()))
	}

	return nil
}

func assignIPAddresses(nm *types.NetworkModel, layer *types.Layer) error {
	poolmap := map[string]*ipPool{}
	for _, policy := range layer.IPPolicy {
		poolrange, err := netip.ParsePrefix(policy.AddrRange)
		if err != nil {
			return fmt.Errorf("invalid range (%v) for policy (%v)", policy.AddrRange, policy.Name)
		}
		bits := policy.DefaultPrefixLength
		pool, err := initIPPool(poolrange, bits)
		if err != nil {
			return err
		}
		poolmap[policy.Name] = pool
	}
	if len(poolmap) == 0 {
		return nil
	}

	segs := nm.NetworkSegments[layer.Name]
	for _, seg := range segs {
		// check policy consistency
		segmentPolicy := ""
		for _, iface := range seg.Interfaces {
			if iface.AwareLayer(layer.Name) {
				p := iface.GetLayerPolicy(layer.Name)
				if p == nil {
					return fmt.Errorf("no policy defined for interface %s in layer %s", iface, layer.Name)
				}
				if _, ok := poolmap[p.Name]; !ok {
					return fmt.Errorf("undefined IP policy %s for interface %s", p.Name, iface)
				}
				if segmentPolicy == "" {
					segmentPolicy = p.Name
				} else if segmentPolicy != p.Name {
					return fmt.Errorf("inconsistent IP policy (%s, %s) for segment %+v", segmentPolicy, p.Name, seg)
				}
			} else {
				return fmt.Errorf("panic: layer-non-aware interface %s included in segment %+v", iface.String(), seg)
			}
		}
		if segmentPolicy == "" {
			// no segments for the layer
			return fmt.Errorf("no segment for layer %s", layer.Name)
		}
		pool, ok := poolmap[segmentPolicy]
		if !ok {
			return fmt.Errorf("no address pool for policy %s", segmentPolicy)
		}

		// check segment members
		netSegment := &netSegment{bits: pool.bits}
		for _, conn := range seg.Connections {
			netSegment.checkConnection(conn, layer)
		}
		for _, iface := range seg.Interfaces {
			netSegment.checkInterface(iface, layer)
		}
		pool.segments = append(pool.segments, netSegment)
		if !netSegment.bound {
			pool.n_unassigned += 1
		}

		// check address reservation consistency
		err := netSegment.checkReservedInterfaces()
		if err != nil {
			return err
		}
	}

	for policy, pool := range poolmap {
		prefixes, err := pool.getAvailablePrefix(pool.n_unassigned)
		if err != nil {
			return err
		}
		for _, seg := range pool.segments {
			if !seg.bound {
				if len(prefixes) <= 0 {
					return fmt.Errorf("address reservation panic in policy %v", policy)
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
				iface.AddParam(layer.IPAddressReplacer(), addrs[i].String())
				iface.AddParam(layer.IPNetworkReplacer(), seg.prefix.String())
				iface.AddParam(layer.IPPrefixLengthReplacer(), strconv.Itoa(seg.prefix.Bits()))
			}
			for i, iface := range seg.rifaces {
				iface.AddParam(layer.IPAddressReplacer(), seg.raddrs[i].String())
				iface.AddParam(layer.IPNetworkReplacer(), seg.prefix.String())
				iface.AddParam(layer.IPPrefixLengthReplacer(), strconv.Itoa(seg.prefix.Bits()))
			}
		}
		if len(prefixes) > 0 {
			return fmt.Errorf("address reservation panic: %d prefixes unassigned", len(prefixes))
		}
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

// func getASNumber(cfg *Config, cnt int) ([]int, error) {
// 	var asmin int
// 	var asmax int
// 	var asnumbers = make([]int, 0, cnt)
// 	if cfg.GlobalSettings.ASNumberMin > 0 {
// 		asmin = cfg.GlobalSettings.ASNumberMin
// 		if cfg.GlobalSettings.ASNumberMax > 0 {
// 			asmax = cfg.GlobalSettings.ASNumberMax
// 		} else {
// 			asmax = 65535
// 		}
// 		if asmax <= asmin {
// 			return nil, fmt.Errorf("invalid AS range (%d - %d) specified in configuration", asmin, asmax)
// 		}
// 		if (asmax - asmin + 1) < cnt {
// 			return nil, fmt.Errorf("requested %d AS numbers, but specified AS range has only %d numbers", cnt, asmax-asmin+1)
// 		}
// 	} else {
// 		// if ASNumberMin/Max not given, automatically use Private AS numbers
// 		if cnt <= 535 {
// 			asmin = 65001
// 			asmax = 65535
// 		} else if cnt <= 1024 {
// 			asmin = 64512
// 			asmax = 65535
// 		} else {
// 			// currently returns error
// 			return nil, fmt.Errorf("requested more than 1024 private AS numbers")
// 		}
// 	}
// 	for i := 0; i < cnt; i++ {
// 		asnumbers = append(asnumbers, asmin+i)
// 	}
// 	return asnumbers, nil
// }
