package visual

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/awalterschulze/gographviz"

	"github.com/cpflat/dot2tinet/pkg/model"
)

const KEY_NODE_LABEL = "label"
const KEY_NODE_XLABEL = "xlabel"
const KEY_NODE_STYLE = "style"
const KEY_EDGE_LABEL = "label"
const KEY_EDGE_HEADLABEL = "headlabel"
const KEY_EDGE_TAILLABEL = "taillabel"

func abbreviateIPAddress(addr string, plen string) (string, error) {
	ip, err := netip.ParseAddr(addr)
	if err != nil {
		return addr, err
	}
	if ip.Is4() {
		pl, err := strconv.Atoi(plen)
		if err != nil {
			return addr, err
		}
		if pl >= 24 {
			return "." + strings.Split(addr, ".")[3], nil
		}
	}
	return addr, nil
}

func getNodeLoopback(node *model.Node, layer *model.IPSpaceDefinition) (string, error) {
	var addr string
	var err error
	addr, err = node.GetValue(layer.IPLoopbackReplacer())
	return addr, err
}

func getConnectionNetwork(conn *model.Connection, layer *model.IPSpaceDefinition) (string, string, error) {
	var net string
	var plen string
	var err error
	if conn.Src.IsAware(layer.Name) {
		net, err = conn.Src.GetValue(layer.IPNetworkReplacer())
		if err != nil {
			return "", "", err
		}
		plen, err = conn.Src.GetValue(layer.IPPrefixLengthReplacer())
		if err != nil {
			return "", "", err
		}
	} else if conn.Dst.IsAware(layer.Name) {
		net, err = conn.Dst.GetValue(layer.IPNetworkReplacer())
		if err != nil {
			return "", "", err
		}
		plen, err = conn.Dst.GetValue(layer.IPPrefixLengthReplacer())
		if err != nil {
			return "", "", err
		}
	} else {
		return "", "", fmt.Errorf("panic: Connection %s is not aware of layer %s", conn.String(), layer.Name)
	}
	return net, plen, err
}

func getInterfaceAddress(iface *model.Interface, layer *model.IPSpaceDefinition) (string, error) {
	addr, err := iface.GetValue(layer.IPAddressReplacer())
	if err != nil {
		err = fmt.Errorf(
			"panic: Interface %s of Node %s is aware of layer %s but does not have ip address",
			iface.Name, iface.Node.Name, layer.Name,
		)
	}
	return addr, err
}

func GraphToDot(cfg *model.Config, nm *model.NetworkModel, layer string) (string, error) {
	var layers []*model.IPSpaceDefinition
	if layer == "" {
		for i := range cfg.IPSpaceDefinitions {
			layers = append(layers, &cfg.IPSpaceDefinitions[i])
		}
	} else {
		l, ok := cfg.IPSpaceDefinitionByName(layer)
		if !ok {
			return "", fmt.Errorf("unknown layer %s", layer)
		}
		layers = append(layers, l)
	}

	g := gographviz.NewGraph()
	if err := g.SetName("G"); err != nil {
		return "", err
	}
	if err := g.SetDir(true); err != nil {
		return "", err
	}

	// set node name and loopback in node labels
	for _, node := range nm.Nodes {
		attrs := map[string]string{}

		// check the node is virtual
		if node.Virtual {
			attrs[KEY_NODE_STYLE] = "dashed"
		}

		// check the node is active (aware of any layer)
		flag := false
		var lo string
		for _, l := range layers {
			if node.IsAware(l.Name) {
				flag = true
				lo, _ = getNodeLoopback(node, l)
			}
		}
		if flag {
			attrs[KEY_NODE_LABEL] = node.Name
		} else {
			attrs[KEY_NODE_LABEL] = ""
		}

		if lo != "" {
			attrs[KEY_NODE_LABEL] = attrs[KEY_NODE_LABEL] + "\\n" + "lo: " + lo
		}

		if err := g.AddNode("G", node.Name, attrs); err != nil {
			return "", err
		}
	}

	// set interface information without connections
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			if iface.Opposite == nil {
				for _, l := range layers {
					if !iface.IsAware(l.Name) {
						continue
					}

					// sanity check
					addr, err := getInterfaceAddress(iface, l)
					if err != nil {
						return "", err
					}

					// add corresponding ip address information to node labels
					n := g.Nodes.Lookup[node.Name]
					n.Attrs[KEY_NODE_LABEL] = n.Attrs[KEY_NODE_LABEL] + ", " + iface.Name + ": " + addr
				}
			}
		}
	}

	for _, conn := range nm.Connections {
		flag := false
		for _, l := range layers {
			if conn.IPSpaces.Contains(l.Name) {
				flag = true
			}
		}
		if !flag {
			// skip the connection because not considered connected in all layers
			continue
		}

		attrs := map[string]string{"dir": "none"}
		for _, l := range layers {
			if !conn.IPSpaces.Contains(l.Name) {
				continue
			}

			if !conn.Src.IsAware(l.Name) && conn.Dst.IsAware(l.Name) {
				continue
			}

			net, plen, err := getConnectionNetwork(conn, l)
			if err != nil {
				return "", err
			}
			if _, ok := attrs[KEY_EDGE_LABEL]; ok {
				attrs[KEY_EDGE_LABEL] = attrs[KEY_EDGE_LABEL] + "\\n" + net
			} else {
				attrs[KEY_EDGE_LABEL] = net
			}

			if conn.Src.IsAware(l.Name) {
				src_addr, err := getInterfaceAddress(conn.Src, l)
				if err != nil {
					return "", err
				}
				src_addr, err = abbreviateIPAddress(src_addr, plen)
				if err != nil {
					return "", err
				}
				if _, ok := attrs[KEY_EDGE_TAILLABEL]; ok {
					attrs[KEY_EDGE_TAILLABEL] = attrs[KEY_EDGE_TAILLABEL] + "\\n" + src_addr
				} else {
					attrs[KEY_EDGE_TAILLABEL] = src_addr
				}
			}
			if conn.Dst.IsAware(l.Name) {
				dst_addr, err := getInterfaceAddress(conn.Dst, l)
				if err != nil {
					return "", err
				}
				dst_addr, err = abbreviateIPAddress(dst_addr, plen)
				if err != nil {
					return "", err
				}
				if _, ok := attrs[KEY_EDGE_HEADLABEL]; ok {
					attrs[KEY_EDGE_HEADLABEL] = attrs[KEY_EDGE_HEADLABEL] + "\\n" + dst_addr
				} else {
					attrs[KEY_EDGE_HEADLABEL] = dst_addr
				}
			}
		}
		if err := g.AddEdge(conn.Src.Node.Name, conn.Dst.Node.Name, true, attrs); err != nil {
			return "", err
		}
	}

	for _, node := range g.Nodes.Nodes {
		node.Attrs[KEY_NODE_LABEL] = "\"" + node.Attrs[KEY_NODE_LABEL] + "\""
	}
	for _, edge := range g.Edges.Edges {
		if _, ok := edge.Attrs[KEY_EDGE_LABEL]; ok {
			edge.Attrs[KEY_EDGE_LABEL] = "\"" + edge.Attrs[KEY_EDGE_LABEL] + "\""
		}
		if _, ok := edge.Attrs[KEY_EDGE_TAILLABEL]; ok {
			edge.Attrs[KEY_EDGE_TAILLABEL] = "\"" + edge.Attrs[KEY_EDGE_TAILLABEL] + "\""
		}
		if _, ok := edge.Attrs[KEY_EDGE_HEADLABEL]; ok {
			edge.Attrs[KEY_EDGE_HEADLABEL] = "\"" + edge.Attrs[KEY_EDGE_HEADLABEL] + "\""
		}
	}

	output := g.String()
	return output, nil
}
