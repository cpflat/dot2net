package kathara

import (
	"embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

const KatharaOutputFile = "lab.conf"

// InterfaceNamePrefix is not a default but a requirement. Kathara names a
// device's interfaces after the index written in lab.conf - r1[0] becomes eth0
// inside the container - and offers no way to change that, so a scenario asking
// for another prefix cannot be honoured. Rather than overwrite the request in
// silence, the module reports it.
const InterfaceNamePrefix = "eth"

// Parameters the module attaches to each interface. The collision domain is the
// shared medium an interface sits on; the index is the N of r1[N], which has to
// match the ethN the container ends up with.
const CollisionDomainParamName = "_kathara_cd"
const InterfaceIndexParamName = "_kathara_index"

const NetworkClassName = "_katharaNetwork"
const NodeClassName = "_katharaNode"
const InterfaceClassName = "_katharaInterface"

const KatharaLineFormatName = "_katharaLine"

// Kathara validates these itself and fails the whole lab, so the same rules are
// checked here where the message can name the scenario's own object.
var deviceNamePattern = regexp.MustCompile(`^[a-z0-9_]{1,30}$`)
var collisionDomainPattern = regexp.MustCompile(`^\w+$`)

//go:embed templates/*
var templates embed.FS

type KatharaModule struct {
	*types.StandardModule
}

// Capabilities provided by this module.
var (
	_ types.Module             = (*KatharaModule)(nil)
	_ types.ObjectClassifier   = (*KatharaModule)(nil)
	_ types.ParameterProvider  = (*KatharaModule)(nil)
	_ types.RequirementChecker = (*KatharaModule)(nil)
)

func NewModule() types.Module {
	return &KatharaModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *KatharaModule) UpdateConfig(cfg *types.Config) error {
	// Every line of lab.conf stands on its own: the file is a set of
	// declarations, not a sequence, and Kathara's parser reads one line at a
	// time without caring about their order.
	cfg.AddFormatStyle(&types.FormatStyle{
		Name:                KatharaLineFormatName,
		MergeBlockSeparator: "\n",
	})

	cfg.AddFileDefinition(&types.FileDefinition{
		Name:  KatharaOutputFile,
		Path:  "",
		Scope: types.ClassTypeNetwork,
	})

	ct, err := templateFrom("templates/lab.conf.network", &types.ConfigTemplate{
		File: KatharaOutputFile,
	})
	if err != nil {
		return err
	}
	cfg.AddNetworkClass(&types.NetworkClass{
		Name:            NetworkClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct},
	})

	ct, err = templateFrom("templates/lab.conf.node_kathara_device", &types.ConfigTemplate{
		Name:   "kathara_device",
		Format: KatharaLineFormatName,
	})
	if err != nil {
		return err
	}
	cfg.AddNodeClass(&types.NodeClass{
		Name:            NodeClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct},
	})

	// RequiredLink: the line declares a wire, so it must not be emitted for a
	// connection that models a shared segment without an actual link.
	ct, err = templateFrom("templates/lab.conf.interface_kathara_interface", &types.ConfigTemplate{
		Name:         "kathara_interface",
		Format:       KatharaLineFormatName,
		RequiredLink: true,
	})
	if err != nil {
		return err
	}
	cfg.AddInterfaceClass(&types.InterfaceClass{
		Name:            InterfaceClassName,
		ConfigTemplates: []*types.ConfigTemplate{ct},
	})

	return nil
}

func templateFrom(path string, ct *types.ConfigTemplate) (*types.ConfigTemplate, error) {
	bytes, err := templates.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ct.Template = []string{string(bytes)}
	return ct, nil
}

// ClassifyObjects gives the lab.conf lines to the devices and interfaces that
// end up in the file. A switch node is not a device: Kathara realizes a shared
// medium as a collision domain, which exists only as the name its members
// mention, so the node itself has no line of its own.
func (m *KatharaModule) ClassifyObjects(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			continue
		}
		node.AddModuleClassLabels(NodeClassName)
		for _, iface := range node.Interfaces {
			iface.AddModuleClassLabels(InterfaceClassName)
		}
	}
	return nil
}

// GenerateParameters names the collision domain each interface sits on, and the
// index it takes in its device's line.
//
// A connection to a switch names the switch: everything meeting there shares one
// medium, which is exactly what a collision domain is. A connection between two
// ordinary devices names the connection, giving it a medium of its own.
func (m *KatharaModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		if cfg.IsSwitchNode(node) {
			continue
		}
		for _, iface := range node.Interfaces {
			if iface.Connection == nil {
				continue
			}
			cd := iface.Connection.Name
			if iface.Opposite != nil && cfg.IsSwitchNode(iface.Opposite.Node) {
				cd = iface.Opposite.Node.Name
			}
			if !collisionDomainPattern.MatchString(cd) {
				return fmt.Errorf(
					"collision domain name %q is not accepted by Kathara (it allows only word characters)", cd)
			}
			iface.AddParam(CollisionDomainParamName, cd)

			index, err := interfaceIndex(iface)
			if err != nil {
				return err
			}
			iface.AddParam(InterfaceIndexParamName, strconv.Itoa(index))
		}
	}
	return nil
}

// interfaceIndex recovers the N of ethN. The name is where the number lives:
// Kathara derives the interface name inside the container from the index in
// lab.conf, so the two cannot be chosen independently.
func interfaceIndex(iface *types.Interface) (int, error) {
	rest, ok := strings.CutPrefix(iface.Name, InterfaceNamePrefix)
	if !ok {
		return 0, fmt.Errorf(
			"interface %s of node %s is not named %sN, so its index in lab.conf cannot be told",
			iface.Name, iface.Node.Name, InterfaceNamePrefix)
	}
	index, err := strconv.Atoi(rest)
	if err != nil {
		return 0, fmt.Errorf(
			"interface %s of node %s is not named %sN, so its index in lab.conf cannot be told",
			iface.Name, iface.Node.Name, InterfaceNamePrefix)
	}
	return index, nil
}

func (m *KatharaModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	// The interface prefix is dictated by Kathara, so a scenario that asks for
	// another one is telling the module to do something it cannot. Say so
	// rather than overwrite the request without a word.
	for _, ic := range cfg.InterfaceClasses {
		if ic.Prefix != "" && ic.Prefix != InterfaceNamePrefix {
			return fmt.Errorf(
				"interfaceclass %s sets prefix %q, but Kathara names a device's interfaces %sN "+
					"after their index in lab.conf and offers no way to change that",
				ic.Name, ic.Prefix, InterfaceNamePrefix)
		}
	}

	for _, node := range nm.Nodes {
		if node.IsVirtual() || cfg.IsSwitchNode(node) {
			continue
		}
		if !deviceNamePattern.MatchString(node.Name) {
			return fmt.Errorf(
				"node name %q is not accepted by Kathara (lowercase letters, digits and "+
					"underscores, at most 30 characters)", node.Name)
		}
		// Kathara rejects a device whose interface indexes have a hole, and the
		// indexes come from the names, which are handed out before anything is
		// dropped from the output. A virtual interface therefore leaves a gap.
		seen := map[int]bool{}
		count := 0
		for _, iface := range node.Interfaces {
			if iface.IsVirtual() || iface.Connection == nil {
				continue
			}
			index, err := interfaceIndex(iface)
			if err != nil {
				return err
			}
			seen[index] = true
			count++
		}
		for i := 0; i < count; i++ {
			if !seen[i] {
				return fmt.Errorf(
					"node %s has a hole in its interface indexes (%s%d is missing), which Kathara "+
						"rejects; renumbering after dropping interfaces is not implemented yet",
					node.Name, InterfaceNamePrefix, i)
			}
		}
	}
	return nil
}
