// Package assert provides a module that checks the expectations a scenario
// declares about itself. It generates no output.
//
// It exists because the golden tests cannot notice a class that is declared but
// never applied: such a class contributes nothing to the output, so regenerating
// the expected files simply freezes its absence. This actually happened -
// example/vlan_multihost declared segment classes that no relational label ever
// attached, and the breakage was released and preserved by the golden files.
//
// Requiring every class to be applied is not workable: scenarios legitimately
// define classes they do not use in every topology. So the check is opt-in, and
// the scenario states the expectation itself.
package assert

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

// UsedValueKey is the values key that marks a class as one the scenario expects
// to be applied to at least one object. It lives in values rather than in a
// dedicated field so that no core vocabulary is spent on a checking concern.
const UsedValueKey = "assert_used"

type AssertModule struct {
	*types.StandardModule
}

// Capabilities provided by this module.
var (
	_ types.Module             = (*AssertModule)(nil)
	_ types.RequirementChecker = (*AssertModule)(nil)
)

func NewModule() types.Module {
	return &AssertModule{
		StandardModule: types.NewStandardModule(),
	}
}

// UpdateConfig is a no-op: the module contributes no classes, files or formats.
func (m *AssertModule) UpdateConfig(_ *types.Config) error {
	return nil
}

// classRef identifies a class definition for reporting.
type classRef struct {
	classType string
	name      string
}

func (c classRef) String() string {
	return c.classType + "class " + c.name
}

// isAsserted reports whether values asks for the used-check. Any value other
// than a boolean literal is rejected, so that a typo does not silently disable
// the assertion it was meant to enable.
func isAsserted(values map[string]string, ref classRef) (bool, error) {
	raw, ok := values[UsedValueKey]
	if !ok {
		return false, nil
	}
	asserted, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s: %s must be a boolean, got %q", ref, UsedValueKey, raw)
	}
	return asserted, nil
}

// assertedClasses collects the classes whose definition asks to be checked.
func assertedClasses(cfg *types.Config) ([]classRef, error) {
	var refs []classRef

	add := func(classType, name string, values map[string]string) error {
		ref := classRef{classType: classType, name: name}
		asserted, err := isAsserted(values, ref)
		if err != nil {
			return err
		}
		if asserted {
			refs = append(refs, ref)
		}
		return nil
	}

	// Network classes are applied to the network model unconditionally, so the
	// assertion could never fail and would give false confidence.
	for _, c := range cfg.NetworkClasses {
		ref := classRef{classType: types.ClassTypeNetwork, name: c.Name}
		asserted, err := isAsserted(c.Values, ref)
		if err != nil {
			return nil, err
		}
		if asserted {
			return nil, fmt.Errorf("%s: %s is meaningless here because every network class is always applied", ref, UsedValueKey)
		}
	}

	for _, c := range cfg.NodeClasses {
		if err := add(types.ClassTypeNode, c.Name, c.Values); err != nil {
			return nil, err
		}
	}
	for _, c := range cfg.InterfaceClasses {
		if err := add(types.ClassTypeInterface, c.Name, c.Values); err != nil {
			return nil, err
		}
	}
	for _, c := range cfg.ConnectionClasses {
		if err := add(types.ClassTypeConnection, c.Name, c.Values); err != nil {
			return nil, err
		}
	}
	for _, c := range cfg.GroupClasses {
		if err := add(types.ClassTypeGroup, c.Name, c.Values); err != nil {
			return nil, err
		}
	}
	for _, c := range cfg.SegmentClasses {
		if err := add(types.ClassTypeSegment, c.Name, c.Values); err != nil {
			return nil, err
		}
	}

	return refs, nil
}

// appliedClasses collects the class labels that actually reached an object.
// Segments are walked separately: NetworkModel.LabelOwners does not list them
// even though NetworkSegment implements LabelOwner.
func appliedClasses(nm *types.NetworkModel) map[classRef]bool {
	applied := map[classRef]bool{}

	mark := func(classType string, lo types.LabelOwner) {
		for _, name := range lo.ClassLabels() {
			applied[classRef{classType: classType, name: name}] = true
		}
	}

	for _, n := range nm.Nodes {
		mark(types.ClassTypeNode, n)
		for _, iface := range n.Interfaces {
			mark(types.ClassTypeInterface, iface)
		}
	}
	for _, conn := range nm.Connections {
		mark(types.ClassTypeConnection, conn)
	}
	for _, g := range nm.Groups {
		mark(types.ClassTypeGroup, g)
	}
	for _, segs := range nm.NetworkSegments {
		for _, seg := range segs {
			mark(types.ClassTypeSegment, seg)
		}
	}

	return applied
}

func (m *AssertModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	refs, err := assertedClasses(cfg)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		return fmt.Errorf("the assert module is loaded but no class declares %s, so nothing is checked", UsedValueKey)
	}

	applied := appliedClasses(nm)

	var unused []string
	for _, ref := range refs {
		if !applied[ref] {
			unused = append(unused, ref.String())
		}
	}
	if len(unused) == 0 {
		return nil
	}

	sort.Strings(unused)
	return fmt.Errorf(
		"declared with %s but never applied to any object: %s",
		UsedValueKey, strings.Join(unused, ", "),
	)
}
