package model

import (
	"fmt"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/cpflat/dot2net/pkg/types"
)

// DependencyNode represents a node in dependency graph
type DependencyNode[T any] interface {
	GetID() string
	GetDependencies() ([]string, error)
	GetItem() T
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
		// Find the cycle path
		cycleStartIndex := -1
		for i, pathNode := range dg.visitPath {
			if pathNode == nodeID {
				cycleStartIndex = i
				break
			}
		}
		
		var cyclePath []string
		if cycleStartIndex >= 0 {
			cyclePath = append(cyclePath, dg.visitPath[cycleStartIndex:]...)
			cyclePath = append(cyclePath, nodeID) // complete the cycle
		} else {
			cyclePath = []string{nodeID}
		}
		
		return fmt.Errorf("cyclic dependency detected: %s", fmt.Sprintf("%s", cyclePath))
	}

	dg.temporary.Add(nodeID)
	dg.visitPath = append(dg.visitPath, nodeID)
	node := dg.nodes[nodeID]

	dependencies, err := node.GetDependencies()
	if err != nil {
		dg.visitPath = dg.visitPath[:len(dg.visitPath)-1] // remove from path on error
		return err
	}

	// Sort dependencies to ensure stable processing order
	sortedDeps := make([]string, len(dependencies))
	copy(sortedDeps, dependencies)
	sort.Strings(sortedDeps)

	for _, depID := range sortedDeps {
		if _, exists := dg.nodes[depID]; !exists {
			dg.visitPath = dg.visitPath[:len(dg.visitPath)-1] // remove from path on error
			return fmt.Errorf("dependency %s not found for node %s", depID, nodeID)
		}
		if err := dg.visit(depID, sorted); err != nil {
			return err
		}
	}

	dg.temporary.Remove(nodeID)
	dg.visitPath = dg.visitPath[:len(dg.visitPath)-1] // remove from path when done
	dg.permanent.Add(nodeID)
	*sorted = append(*sorted, node.GetItem())
	return nil
}

// reorderConfigTemplates sorts ConfigTemplates based on their dependency relationships
func reorderConfigTemplates(cts []*types.ConfigTemplate, verbose bool) ([]*types.ConfigTemplate, error) {
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
	ctmap    map[string][]int  // name -> indices
	grouped  map[string][]int  // group -> indices
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

// reorderNameSpacers sorts NameSpacers based on their dependency relationships using DependClasses and Depends methods
func reorderNameSpacers(namespacers []types.NameSpacer, verbose bool) ([]types.NameSpacer, error) {
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