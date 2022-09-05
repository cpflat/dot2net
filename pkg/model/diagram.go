package model

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

var SEPARATOR *regexp.Regexp

type NetworkDiagram struct {
	*simple.DirectedGraph
}

func newNetworkDiagram() *NetworkDiagram {
	return &NetworkDiagram{DirectedGraph: simple.NewDirectedGraph()}
}

func (nd *NetworkDiagram) NewNode() graph.Node {
	return &DiagramNode{Node: nd.DirectedGraph.NewNode()}
}

func (nd *NetworkDiagram) NewEdge(from, to graph.Node) graph.Edge {
	return &DiagramEdge{Edge: nd.DirectedGraph.NewEdge(from, to)}
}

func (nd *NetworkDiagram) AllNodes() []*DiagramNode {
	iterNodes := nd.Nodes()
	nodes := make([]*DiagramNode, 0, iterNodes.Len())
	for iterNodes.Next() {
		n := iterNodes.Node().(*DiagramNode)
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Name < nodes[j].Name })
	return nodes
}

func (nd *NetworkDiagram) AllEdges() []*DiagramEdge {
	iterEdges := nd.Edges()
	edges := make([]*DiagramEdge, 0, iterEdges.Len())
	for iterEdges.Next() {
		e := iterEdges.Edge().(*DiagramEdge)
		edges = append(edges, e)
	}
	return edges
}

func NetworkDiagramFromDotFile(filepath string) (*NetworkDiagram, error) {
	src, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	dst := newNetworkDiagram()
	if err = dot.Unmarshal([]byte(src), dst); err != nil {
		return nil, err
	}
	return dst, nil
}

type DiagramNode struct {
	graph.Node
	dot.DOTIDSetter
	Name    string
	Classes []string
}

func (n *DiagramNode) SetAttribute(attr encoding.Attribute) error {
	switch attr.Key {
	case "label":
		n.Classes = append(n.Classes, parseClasses(attr.Value)...)
	case "class": // any of label and class will be used as node class identifier.
		n.Classes = append(n.Classes, parseClasses(attr.Value)...)
	default:
		// ignore
	}
	return nil
}

func (n *DiagramNode) SetDOTID(id string) {
	n.Name = id
}

func (n *DiagramNode) String() string {
	return n.Name
}

type DiagramEdge struct {
	graph.Edge
	SrcName    string
	DstName    string
	Classes    []string
	SrcClasses []string
	DstClasses []string
}

func (e *DiagramEdge) SetAttribute(attr encoding.Attribute) error {
	switch attr.Key {
	case "label":
		e.Classes = parseClasses(attr.Value)
	case "class":
		e.Classes = parseClasses(attr.Value)
	case "headlabel":
		e.SrcClasses = parseClasses(attr.Value)
	case "taillabel":
		e.DstClasses = parseClasses(attr.Value)
	default:
		// ignore
	}
	return nil
}

func (e *DiagramEdge) SetFromPort(port, compass string) error {
	e.SrcName = port
	return nil
}

func (e *DiagramEdge) SetToPort(port, compass string) error {
	e.DstName = port
	return nil
}

func parseClasses(value string) (classes []string) {
	if value == "" {
		return classes
	}
	if SEPARATOR == nil {
		SEPARATOR = regexp.MustCompile("[,;]")
	}
	for _, s := range SEPARATOR.Split(value, -1) {
		classes = append(classes, strings.TrimSpace(s))
	}
	return classes
}
