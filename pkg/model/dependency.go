package model

import (
	"fmt"
	"sort"

	"github.com/cpflat/dot2net/pkg/types"
	mapset "github.com/deckarep/golang-set/v2"
)

// DependencyNode represents a node in dependency graph
type DependencyNode[T any] interface {
	GetID() string
	GetDependencies() ([]string, error)
	GetItem() T
	// GetLabel returns a human-readable identifier used in diagnostics such as
	// cyclic-dependency messages. Unlike GetID (an internal synthetic key like
	// "template_3"), it should name the underlying object meaningfully.
	GetLabel() string
}

// DependencyGraph handles topological sorting of dependency nodes
type DependencyGraph[T any] struct {
	nodes     map[string]DependencyNode[T]
	permanent mapset.Set[string]
	temporary mapset.Set[string]
	visitPath []string // track current visit path for cycle detection
}

func NewDependencyGraph[T any]() *DependencyGraph[T] {
	return &DependencyGraph[T]{
		nodes:     make(map[string]DependencyNode[T]),
		permanent: mapset.NewSet[string](),
		temporary: mapset.NewSet[string](),
	}
}

func (dg *DependencyGraph[T]) AddNode(node DependencyNode[T]) {
	dg.nodes[node.GetID()] = node
}

func (dg *DependencyGraph[T]) TopologicalSort() ([]T, error) {
	dg.permanent = mapset.NewSet[string]()
	dg.temporary = mapset.NewSet[string]()
	dg.visitPath = make([]string, 0)
	var sorted []T

	// Collect node IDs and sort them to ensure stable iteration order
	var nodeIDs []string
	for id := range dg.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	for _, id := range nodeIDs {
		if !dg.permanent.Contains(id) {
			if err := dg.visit(id, &sorted); err != nil {
				return nil, err
			}
		}
	}

	if len(sorted) != len(dg.nodes) {
		return nil, fmt.Errorf("some nodes are not included in the sorted list")
	}

	return sorted, nil
}

func (dg *DependencyGraph[T]) visit(nodeID string, sorted *[]T) error {
	if dg.permanent.Contains(nodeID) {
		return nil
	}
	if dg.temporary.Contains(nodeID) {
		// Build a human-readable cycle path from the current visit stack.
		cycleStartIndex := -1
		for i, pathNode := range dg.visitPath {
			if pathNode == nodeID {
				cycleStartIndex = i
				break
			}
		}

		var cyclePath []string
		if cycleStartIndex >= 0 {
			for _, id := range dg.visitPath[cycleStartIndex:] {
				cyclePath = append(cyclePath, dg.labelFor(id))
			}
			cyclePath = append(cyclePath, dg.labelFor(nodeID)) // complete the cycle
		} else {
			cyclePath = []string{dg.labelFor(nodeID)}
		}

		return fmt.Errorf("cyclic dependency detected: %s", cyclePath)
	}

	// Mark as being visited and ensure the temporary mark and visit-path entry
	// are always cleaned up together on every return path (error or success),
	// keeping dg state consistent so the graph can be reused safely.
	dg.temporary.Add(nodeID)
	dg.visitPath = append(dg.visitPath, nodeID)
	defer func() {
		dg.temporary.Remove(nodeID)
		dg.visitPath = dg.visitPath[:len(dg.visitPath)-1]
	}()

	node := dg.nodes[nodeID]

	dependencies, err := node.GetDependencies()
	if err != nil {
		return err
	}

	// Sort dependencies to ensure stable processing order
	sortedDeps := make([]string, len(dependencies))
	copy(sortedDeps, dependencies)
	sort.Strings(sortedDeps)

	for _, depID := range sortedDeps {
		if _, exists := dg.nodes[depID]; !exists {
			return fmt.Errorf("dependency %s not found for node %s", dg.labelFor(depID), dg.labelFor(nodeID))
		}
		if err := dg.visit(depID, sorted); err != nil {
			return err
		}
	}

	dg.permanent.Add(nodeID)
	*sorted = append(*sorted, node.GetItem())
	return nil
}

// labelFor returns a human-readable label for a node ID (falling back to the
// raw ID if the node is unknown), used in diagnostic messages.
func (dg *DependencyGraph[T]) labelFor(nodeID string) string {
	if node, ok := dg.nodes[nodeID]; ok {
		return node.GetLabel()
	}
	return nodeID
}

// reorderConfigTemplates sorts ConfigTemplates based on their dependency relationships
func reorderConfigTemplates(cts []*types.ConfigTemplate) ([]*types.ConfigTemplate, error) {
	// Build name and group mappings
	ctmap := make(map[string][]int)
	grouped := make(map[string][]int)

	for ind, ct := range cts {
		if ct.Name != "" {
			ctmap[ct.Name] = append(ctmap[ct.Name], ind)
		}
		if ct.Group != "" {
			grouped[ct.Group] = append(grouped[ct.Group], ind)
		}
	}

	// Create dependency graph
	dg := NewDependencyGraph[*types.ConfigTemplate]()

	for i, ct := range cts {
		node := &ConfigTemplateDependencyNode{
			template: ct,
			index:    i,
			ctmap:    ctmap,
			grouped:  grouped,
		}
		dg.AddNode(node)
	}

	return dg.TopologicalSort()
}

// ConfigTemplateDependencyNode adapts ConfigTemplate to DependencyNode interface
type ConfigTemplateDependencyNode struct {
	template *types.ConfigTemplate
	index    int
	ctmap    map[string][]int // name -> indices
	grouped  map[string][]int // group -> indices
}

func (ctdn *ConfigTemplateDependencyNode) GetID() string {
	return fmt.Sprintf("template_%d", ctdn.index)
}

func (ctdn *ConfigTemplateDependencyNode) GetDependencies() ([]string, error) {
	var deps []string
	ct := ctdn.template

	// sorter depends on grouped templates
	if ct.Style == types.ConfigTemplateStyleSort {
		if indices, exists := ctdn.grouped[ct.SortGroup]; exists {
			for _, idx := range indices {
				deps = append(deps, fmt.Sprintf("template_%d", idx))
			}
		}
	}

	// explicit dependencies
	for _, depName := range ct.Depends {
		if indices, exists := ctdn.ctmap[depName]; exists {
			for _, idx := range indices {
				deps = append(deps, fmt.Sprintf("template_%d", idx))
			}
		} else {
			return nil, fmt.Errorf("dependency %s not found for template %v", depName, ct)
		}
	}

	return deps, nil
}

func (ctdn *ConfigTemplateDependencyNode) GetItem() *types.ConfigTemplate {
	return ctdn.template
}

func (ctdn *ConfigTemplateDependencyNode) GetLabel() string {
	ct := ctdn.template
	switch {
	case ct.Name != "":
		return fmt.Sprintf("template:%s", ct.Name)
	case ct.File != "":
		return fmt.Sprintf("template(file:%s)", ct.File)
	default:
		return ctdn.GetID()
	}
}

// reorderNameSpacers sorts NameSpacers based on their dependency relationships using DependClasses and Depends methods
func reorderNameSpacers(namespacers []types.NameSpacer) ([]types.NameSpacer, error) {
	// Create dependency graph
	dg := NewDependencyGraph[types.NameSpacer]()

	for i, ns := range namespacers {
		node := &NameSpacerDependencyNode{
			namespacer:  ns,
			index:       i,
			namespacers: namespacers,
		}
		dg.AddNode(node)
	}

	return dg.TopologicalSort()
}

// NameSpacerDependencyNode adapts NameSpacer to DependencyNode interface
type NameSpacerDependencyNode struct {
	namespacer  types.NameSpacer
	index       int
	namespacers []types.NameSpacer
}

func (nsdn *NameSpacerDependencyNode) GetID() string {
	return fmt.Sprintf("namespacer_%d", nsdn.index)
}

func (nsdn *NameSpacerDependencyNode) GetLabel() string {
	return nsdn.namespacer.StringForMessage()
}

func (nsdn *NameSpacerDependencyNode) GetItem() types.NameSpacer {
	return nsdn.namespacer
}

func (nsdn *NameSpacerDependencyNode) GetDependencies() ([]string, error) {
	var deps []string
	ns := nsdn.namespacer

	// Get dependency classes
	dependClasses, err := ns.DependClasses()
	if err != nil {
		return nil, err
	}

	// For each dependency class, find the corresponding NameSpacers
	for _, depClass := range dependClasses {
		dependNameSpacers, err := ns.Depends(depClass)
		if err != nil {
			return nil, err
		}

		// Find the indices of dependent NameSpacers in the original slice
		for _, depNS := range dependNameSpacers {
			for j, originalNS := range nsdn.namespacers {
				if depNS == originalNS {
					deps = append(deps, fmt.Sprintf("namespacer_%d", j))
				}
			}
		}
	}

	return deps, nil
}
