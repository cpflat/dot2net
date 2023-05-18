package model

import (
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/awalterschulze/gographviz"
	mapset "github.com/deckarep/golang-set/v2"
)

var SEPARATOR *regexp.Regexp

type Diagram struct {
	graph      *gographviz.Graph
	nodeGroups map[string][]string
}

func DiagramFromDotFile(filepath string) (*Diagram, error) {
	src, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	graphAst, _ := gographviz.Parse(src)
	graph := gographviz.NewGraph()
	if err := gographviz.Analyse(graphAst, graph); err != nil {
		panic(err)
	}

	diagram := &Diagram{graph: graph, nodeGroups: map[string][]string{}}
	diagram.searchGroupMembers(graph.Name)
	return diagram, nil
}

func (d *Diagram) Nodes() []*gographviz.Node {
	return d.graph.Nodes.Nodes
}

func (d *Diagram) SortedNodes() []*gographviz.Node {
	ret := make([]*gographviz.Node, len(d.graph.Nodes.Nodes))
	copy(ret, d.graph.Nodes.Nodes)
	sort.Slice(ret, func(i, j int) bool { return ret[i].Name < ret[j].Name })
	return ret
}

func (d *Diagram) Links() []*gographviz.Edge {
	return d.graph.Edges.Edges
}

func (d *Diagram) SortedLinks() []*gographviz.Edge {
	ret := make([]*gographviz.Edge, len(d.graph.Edges.Edges))
	copy(ret, d.graph.Edges.Edges)
	sort.SliceStable(ret, func(i, j int) bool {
		var vimin, vimax string
		var vjmin, vjmax string
		if ret[i].Src < ret[i].Dst {
			vimin = ret[i].Src
			vimax = ret[i].Dst
		} else {
			vimin = ret[i].Dst
			vimax = ret[i].Src
		}
		if ret[j].Src < ret[j].Dst {
			vjmin = ret[j].Src
			vjmax = ret[j].Dst
		} else {
			vjmin = ret[j].Dst
			vjmax = ret[i].Src
		}
		if vimin == vjmin {
			return vimax < vjmax
		} else {
			return vimin < vjmin
		}
	})
	return ret
}

func (d *Diagram) Groups() map[string]*gographviz.SubGraph {
	return d.graph.SubGraphs.SubGraphs
}

func (d *Diagram) NodeGroups(name string) (groups []*gographviz.SubGraph) {
	for _, gname := range d.nodeGroups[name] {
		group := d.graph.SubGraphs.SubGraphs[gname]
		groups = append(groups, group)
	}
	return groups
}

func (d *Diagram) searchGroupMembers(parent string) []string {
	var nodes []string
	for child := range d.graph.Relations.ParentToChildren[parent] {
		if _, ok := d.graph.SubGraphs.SubGraphs[child]; ok {
			//if _, ok := d.graph.Relations.ParentToChildren[child]; ok {
			// child is subgraph
			// recursively search member nodes of subgraph child.Name
			nodes = append(nodes, d.searchGroupMembers(child)...)
		} else {
			// child is node
			nodes = append(nodes, child)
		}
	}
	if parent != d.graph.Name {
		// parent corresponds to a subgraph (i.e., group)
		for _, name := range nodes {
			d.nodeGroups[name] = append(d.nodeGroups[name], parent)
		}
	}
	return nodes
}

// Merge Diagram merge components in two Diagram objects.
// Labels in same components are merged.
// Lines are considered same only when the end nodes and "their ports" are completely same
// (Note that links without specified ports are always considered different).
func (d *Diagram) MergeDiagram(d2 *Diagram) {

	// add nodes and their attributes
	for _, node2 := range d2.graph.Nodes.Nodes {
		if node, ok := d.graph.Nodes.Lookup[node2.Name]; ok {
			// node exists, merge attributes
			node.Attrs = mergeAttrs(node.Attrs, node2.Attrs)
		} else {
			// node not exists
			d.graph.Nodes.Add(node2)
		}
	}

	// add links and their attributes
	for _, edge2 := range d2.graph.Edges.Edges {
		match := []*gographviz.Edge{}
		for _, edge := range d.graph.Edges.SrcToDsts[edge2.Src][edge2.Dst] {
			if edge.SrcPort == edge2.SrcPort && edge.DstPort == edge2.DstPort &&
				edge.SrcPort != "" && edge.DstPort != "" {
				match = append(match, edge)
			}
		}
		for _, edge := range d.graph.Edges.DstToSrcs[edge2.Src][edge2.Dst] {
			if edge.SrcPort == edge2.DstPort && edge.DstPort == edge2.SrcPort &&
				edge.SrcPort != "" && edge.DstPort != "" {
				match = append(match, edge)
			}
		}
		if len(match) > 1 {
			panic("multiple corresponding edges found in MergeDiagram process")
		} else if len(match) == 1 {
			// link exists, merge attributes
			edge := match[0]
			edge.Attrs = mergeAttrs(edge.Attrs, edge2.Attrs)
		} else {
			// link not exists
			d.graph.Edges.Add(edge2)
		}
	}

	// add graph attributes
	newAttrs := mergeAttrs(d.graph.Attrs, d2.graph.Attrs)
	d.graph.Attrs = newAttrs

	// add subgraphs and their attributes
	for group, subgraph2 := range d2.graph.SubGraphs.SubGraphs {
		if subgraph, ok := d.graph.SubGraphs.SubGraphs[group]; ok {
			subgraph.Attrs = mergeAttrs(subgraph.Attrs, subgraph2.Attrs)
		} else {
			d.graph.SubGraphs.SubGraphs[group] = subgraph2
		}
	}

	// merge nodeGroups
	for name, groups2 := range d2.nodeGroups {
		if groups, ok := d.nodeGroups[name]; ok {
			set := mapset.NewSet[string]()
			set.Append(groups...)
			set.Append(groups2...)
			d.nodeGroups[name] = set.ToSlice()
		}
	}
}

func mergeAttrs(attrs1 gographviz.Attrs, attrs2 gographviz.Attrs) gographviz.Attrs {
	ret := attrs1.Copy()

	for k, v2 := range attrs2 {
		if v, ok := attrs1[k]; ok {
			// attribute exists, merge attribute description
			ret[k] = v + ";" + v2
		} else {
			ret[k] = v2
		}
	}

	return ret
}

func getNodeLabels(n *gographviz.Node) (labels []string) {
	for k, v := range n.Attrs {
		switch k {
		case
			"xlabel",       // visible on graphviz as external label
			"class",        // invisible on graphviz, but it may emit warnings on dot command
			"conf", "info": // meaningless attributes on dot
			// -> save as node label
			labels = append(labels, ParseLabels(v)...)
		case "label": // visible as graphviz node label, but has conflict with record-shape node format
			// -> ignore label to avoid conflict with record-shape nodes
		default:
			// -> ignore
		}
	}
	return labels
}

func getEdgeLabels(e *gographviz.Edge) (labels []string, srcLabels []string, dstLabels []string) {
	for k, v := range e.Attrs {
		switch k {
		case
			"label",        // visible on graphviz as central edge label
			"class",        // invisible on graphviz, but it may emit warnings on dot command
			"info", "conf": // meaningless attributes on dot
			// -> save as connection label
			labels = append(labels, ParseLabels(v)...)
		case
			"headlabel",                         // visible on graphviz as edge arrowhead label
			"headclass", "headinfo", "headconf": // meaningless attributes on dot
			// -> save as interface label of src interface
			dstLabels = append(dstLabels, ParseLabels(v)...)
		case
			"taillabel",                         // visible on graphviz as edge arrowhead label
			"tailclass", "tailinfo", "tailconf": // meaningless attributes on dot
			// -> save as interface label of dst interface
			srcLabels = append(srcLabels, ParseLabels(v)...)
		default:
			// -> ignore
		}
	}
	return labels, srcLabels, dstLabels
}

func getSubGraphLabels(s *gographviz.SubGraph) (labels []string) {
	for k, v := range s.Attrs {
		switch k {
		case
			"label",        // visible on graphviz as central edge label
			"class",        // invisible on graphviz, but it may emit warnings on dot command
			"info", "conf": // meaningless attributes on dot
			labels = append(labels, ParseLabels(v)...)
		default:
			// -> ignore
		}
	}
	return labels
}

func ParseLabels(value string) (classes []string) {
	if value == "" {
		return classes
	}
	if SEPARATOR == nil {
		SEPARATOR = regexp.MustCompile("[,;]")
	}
	for _, s := range SEPARATOR.Split(strings.Trim(value, "\""), -1) {
		classes = append(classes, strings.TrimSpace(s))
	}
	return classes
}
