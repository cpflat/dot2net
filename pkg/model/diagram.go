package model

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/encoding/dot"

	"gonum.org/v1/gonum/graph/multi"
)

var SEPARATOR *regexp.Regexp

type graphAttribute struct {
	encoding.AttributeSetter
	Labels []string
}

func (ga *graphAttribute) SetAttribute(attr encoding.Attribute) error {
	switch attr.Key {
	default:
		// -> ignore
	}
	return nil
}

type NetworkDiagram struct {
	*multi.DirectedGraph
	graphAttribute

	// implemented interfaces
	dot.AttributeSetters
	dot.DOTIDSetter

	Name     string
	nodeAttr *nodeAttribute
	lineAttr *lineAttribute
}

func newNetworkDiagram() *NetworkDiagram {
	return &NetworkDiagram{
		DirectedGraph: multi.NewDirectedGraph(),
		nodeAttr:      &nodeAttribute{},
		lineAttr:      &lineAttribute{},
	}
}

func (nd *NetworkDiagram) DOTAttributeSetters() (graph, node, edge encoding.AttributeSetter) {
	return nd, nd.nodeAttr, nd.lineAttr
}

func (nd *NetworkDiagram) SetDOTID(id string) {
	nd.Name = id
}

func (nd *NetworkDiagram) NewNode() graph.Node {
	return &DiagramNode{Node: nd.DirectedGraph.NewNode()}
}

func (nd *NetworkDiagram) NewLine(from, to graph.Node) graph.Line {
	return &DiagramLine{Line: nd.DirectedGraph.NewLine(from, to)}
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

func (nd *NetworkDiagram) AllLines() []*DiagramLine {
	iterEdges := nd.Edges()
	lines := make([]*DiagramLine, 0, iterEdges.Len())
	for iterEdges.Next() {
		e := iterEdges.Edge()
		iterLines := nd.Lines(e.From().ID(), e.To().ID())
		for iterLines.Next() {
			lines = append(lines, iterLines.Line().(*DiagramLine))
		}
	}
	return lines
}

// Merge Diagram add components in newnd into nd.
// Labels in same components are merged.
// Lines are considered same only when the end nodes and "their ports" are completely same
// (Note that links without specified ports are always considered different).
func (nd *NetworkDiagram) MergeDiagram(newnd *NetworkDiagram) {

	lineKey := func(l *DiagramLine) [4]string {
		return [4]string{l.From().(*DiagramNode).Name, l.SrcName, l.To().(*DiagramNode).Name, l.DstName}
	}
	lineRevKey := func(l *DiagramLine) [4]string {
		return [4]string{l.To().(*DiagramNode).Name, l.DstName, l.From().(*DiagramNode).Name, l.SrcName}
	}

	nodeMap := map[string]*DiagramNode{}    // nodename
	lineMap := map[[4]string]*DiagramLine{} // [src_nodename, src_port, dst_nodename, dst_port]
	for _, n := range nd.AllNodes() {
		nodeMap[n.Name] = n
	}
	for _, l := range nd.AllLines() {
		lineMap[lineKey(l)] = l
		lineMap[lineRevKey(l)] = l
	}

	for _, n2 := range newnd.AllNodes() {
		if n, ok := nodeMap[n2.Name]; ok {
			n.Labels = append(n.Labels, n2.Labels...)
		} else {
			// add new node on nd
			newnode := nd.NewNode().(*DiagramNode)
			newnode.Name = n2.Name
			newnode.Labels = n2.Labels
			nd.AddNode(newnode)
		}
	}

	for _, l2 := range newnd.AllLines() {
		if l, ok := lineMap[lineKey(l2)]; ok {
			l.Labels = append(l.Labels, l2.Labels...)
			l.SrcLabels = append(l.SrcLabels, l2.SrcLabels...)
			l.DstLabels = append(l.DstLabels, l2.DstLabels...)
		} else if l, ok = lineMap[lineRevKey(l2)]; ok {
			l.Labels = append(l.Labels, l2.Labels...)
			// head or tail classes are reversed
			l.SrcLabels = append(l.SrcLabels, l2.DstLabels...)
			l.DstLabels = append(l.DstLabels, l2.SrcLabels...)
		} else {
			l2_srcNode := l2.From().(*DiagramNode)
			// add new line on nd (of course between nodes on nd)
			srcNode, ok := nodeMap[l2_srcNode.Name]
			if !ok {
				// add new node on nd
				srcNode := nd.NewNode().(*DiagramNode)
				srcNode.Name = l2_srcNode.Name
				srcNode.Labels = l2_srcNode.Labels
				nd.AddNode(srcNode)
			}
			l2_dstNode := l2.To().(*DiagramNode)
			dstNode, ok := nodeMap[l2.To().(*DiagramNode).Name]
			if !ok {
				// add new node on nd
				dstNode := nd.NewNode().(*DiagramNode)
				dstNode.Name = l2_dstNode.Name
				dstNode.Labels = l2_dstNode.Labels
				nd.AddNode(dstNode)
			}
			newLine := nd.NewLine(srcNode, dstNode).(*DiagramLine)
			newLine.SrcName = l2.SrcName
			newLine.DstName = l2.DstName
			newLine.Labels = l2.Labels
			newLine.SrcLabels = l2.SrcLabels
			newLine.DstLabels = l2.DstLabels
			nd.SetLine(newLine)
		}
	}
}

func NetworkDiagramFromDotFile(filepath string) (*NetworkDiagram, error) {
	src, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	nd := newNetworkDiagram()
	if err = dot.UnmarshalMulti([]byte(src), nd); err != nil {
		return nil, err
	}

	// attach global labels to nodes and lines
	if nd.nodeAttr != nil {
		for _, node := range nd.AllNodes() {
			node.Labels = append(node.Labels, nd.nodeAttr.Labels...)
		}
	}
	if nd.lineAttr != nil {
		for _, line := range nd.AllLines() {
			line.Labels = append(line.Labels, nd.lineAttr.Labels...)
			line.SrcLabels = append(line.SrcLabels, nd.lineAttr.SrcLabels...)
			line.DstLabels = append(line.DstLabels, nd.lineAttr.DstLabels...)
		}
	}

	return nd, nil
}

type nodeAttribute struct {
	encoding.AttributeSetter

	Labels []string
}

func (n *nodeAttribute) SetAttribute(attr encoding.Attribute) error {
	switch attr.Key {
	case
		"xlabel",       // visible on graphviz as external label
		"class",        // invisible on graphviz, but it may emit warnings on dot command
		"conf", "info": // meaningless attributes on dot
		// -> save as node label
		n.Labels = append(n.Labels, parseLabels(attr.Value)...)
	case "label": // visible as graphviz node label, but has conflict with record-shape node format
		// -> ignore label to avoid conflict with record-shape nodes
	default:
		// -> ignore
	}
	return nil
}

type DiagramNode struct {
	graph.Node
	nodeAttribute

	// implemented interfaces
	dot.DOTIDSetter

	Name string
}

func (n *DiagramNode) SetDOTID(id string) {
	n.Name = id
}

func (n *DiagramNode) String() string {
	return n.Name
}

type lineAttribute struct {
	encoding.AttributeSetter

	Labels    []string
	SrcLabels []string
	DstLabels []string
}

func (l *lineAttribute) SetAttribute(attr encoding.Attribute) error {
	switch attr.Key {
	case
		"label",        // visible on graphviz as central edge label
		"class",        // invisible on graphviz, but it may emit warnings on dot command
		"info", "conf": // meaningless attributes on dot
		// -> save as connection label
		l.Labels = parseLabels(attr.Value)
	case
		"headlabel",                         // visible on graphviz as edge arrowhead label
		"headclass", "headinfo", "headconf": // meaningless attributes on dot
		// -> save as interface label of src interface
		l.SrcLabels = parseLabels(attr.Value)
	case
		"taillabel",                         // visible on graphviz as edge arrowhead label
		"tailclass", "tailinfo", "tailconf": // meaningless attributes on dot
		// -> save as interface label of dst interface
		l.DstLabels = parseLabels(attr.Value)
	default:
		// -> ignore
	}
	return nil
}

type DiagramLine struct {
	graph.Line
	lineAttribute

	// implemented interfaces
	dot.PortSetter

	SrcName string
	DstName string
}

func (e *DiagramLine) SetFromPort(port, compass string) error {
	e.SrcName = port
	return nil
}

func (e *DiagramLine) SetToPort(port, compass string) error {
	e.DstName = port
	return nil
}

func parseLabels(value string) (classes []string) {
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
