package model

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cpflat/dot2net/pkg/types"
)

// const IPPolicyTypeDefault string = "ip"
// const IPPolicyTypeLoopback string = "loopback"

// const DefaultNodePrefix string = "node"
// const DefaultInterfacePrefix string = "net"

const ManagementInterfaceName string = "mgmt"
const ManagementLayerReplacer string = "mgmt"

const NumberAS string = "as"
const NumberNumber string = "number"

// const DummyIPSpace string = "none"

// BuildNetworkModelForFileList builds a lightweight NetworkModel sufficient for file listing.
// This function only processes the minimum required for FilesToGenerate() to work:
// - Module loading (for FileDefinitions)
// - Topology skeleton (nodes, interfaces, class labels)
// - Classification, both the core pass and the modules'
// - Class validation
// It skips expensive operations like IP address assignment and parameter generation.
//
// Every step that can give an object a class has to run here, because a class is
// what carries the config templates a file comes from. Classifying is cheap - it
// only attaches labels - and leaving it out silently shortens the list: a
// topology file scoped to a worker group is handed out by the module classifier,
// so before this ran, `dot2net files` omitted the per-machine topo.yaml and
// `dot2net clean` left it behind.
func BuildNetworkModelForFileList(cfg *types.Config, d *Diagram) (nm *types.NetworkModel, err error) {
	err = LoadModules(cfg)
	if err != nil {
		return nil, err
	}

	// Before the classes are checked over: a hook written the old way is a
	// config template name until this has run, and two classes writing into one
	// hook would be read as two classes claiming one name.
	types.NormalizeHookNames(cfg)

	// build topology skeleton with class labels
	nm, err = buildSkeleton(cfg, d)
	if err != nil {
		return nil, err
	}

	err = checkWorkerGroupsDisjoint(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = aggregateCrossingLinks(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = classifyBoundaryConnections(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = classifyModuleObjects(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = checkClasses(cfg, nm)
	if err != nil {
		return nil, err
	}

	// Before anything asks whether an object is there. Which files are written
	// depends on it, so the file list has to settle it as well.
	err = resolveDeployForms(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = checkOutputFilesUnique(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = checkCopyTargets(cfg, nm)
	if err != nil {
		return nil, err
	}

	return nm, nil
}

func BuildNetworkModel(cfg *types.Config, d *Diagram, verbose bool) (nm *types.NetworkModel, err error) {

	err = LoadModules(cfg)
	if err != nil {
		return nil, err
	}

	// Before the classes are checked over: a hook written the old way is a
	// config template name until this has run, and two classes writing into one
	// hook would be read as two classes claiming one name.
	types.NormalizeHookNames(cfg)

	// build topology
	nm, err = buildSkeleton(cfg, d)
	if err != nil {
		return nil, err
	}

	// Before anything reads the worker groups: a node in two of them makes both
	// the boundary classification and the per-machine output files inconsistent.
	err = checkWorkerGroupsDisjoint(cfg, nm)
	if err != nil {
		return nil, err
	}

	// Reshape before anything reads the topology: the links this adds have to be
	// seen as leaving a machine like any other.
	err = aggregateCrossingLinks(cfg, nm)
	if err != nil {
		return nil, err
	}

	// Classify before the class labels are resolved: a label added after
	// checkClasses would never become a class. The core pass runs first so that
	// a module classifying objects can already see which connections leave a
	// group.
	err = classifyBoundaryConnections(cfg, nm)
	if err != nil {
		return nil, err
	}

	// Let modules classify or reshape the topology.
	err = classifyModuleObjects(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = checkClasses(cfg, nm)
	if err != nil {
		return nil, err
	}

	// Before anything asks whether an object is there. Which files are written
	// depends on it, so the file list has to settle it as well.
	err = resolveDeployForms(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = checkOutputFilesUnique(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = checkCopyTargets(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = addSpecialInterfaces(cfg, nm)
	if err != nil {
		return nil, err
	}

	// assign names for unnamed objects in topology
	if cfg.GlobalSettings.NodeAutoRename {
		err = assignNodeNames(nm)
		if err != nil {
			return nil, err
		}
	}
	err = assignInterfaceNames(nm)
	if err != nil {
		return nil, err
	}
	err = assignConnectionNames(nm)
	if err != nil {
		return nil, err
	}

	// assign numbers, interface names and addresses
	err = setGivenParameters(nm)
	if err != nil {
		return nil, err
	}

	err = generateModuleParameters(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = assignIPParameters(cfg, nm, verbose)
	if err != nil {
		return nil, err
	}

	err = assignSegmentNames(nm)
	if err != nil {
		return nil, err
	}

	err = assignParameters(cfg, nm)
	if err != nil {
		return nil, err
	}

	err = makeRelativeNamespace(nm)
	if err != nil {
		return nil, err
	}

	// Note: generateValueReferenceParams is called in BuildConfigFiles after LoadTemplates

	// // build config commands from config templates
	// cfg, err = loadTemplates(cfg)
	// if err != nil {
	// 	return nil, err
	// }

	// err = generateConfigFiles(cfg, nm, output)
	// if err != nil {
	// 	return nil, err
	// }

	return nm, err
}

func BuildConfigFiles(cfg *types.Config, nm *types.NetworkModel, verbose bool) error {
	// build config commands from config templates

	err := checkModuleRequirements(cfg, nm)
	if err != nil {
		return err
	}

	cfg, err = types.LoadTemplates(cfg)
	if err != nil {
		return err
	}

	// Generate values_xxx params after templates are parsed
	err = generateValueReferenceParams(cfg, nm)
	if err != nil {
		return err
	}

	err = generateConfigFiles(cfg, nm, verbose)
	if err != nil {
		return err
	}

	return nil
}

func buildSkeleton(cfg *types.Config, d *Diagram) (*types.NetworkModel, error) {
	nm := types.NewNetworkModel()
	nm.Name = cfg.Name
	nm.Classes = cfg.NetworkClasses
	// nm := &NetworkModel{
	// 	NetworkSegments:          map[string][]*SegmentMembers{},
	// 	nodeMap:                  map[string]*Node{},
	// 	groupMap:                 map[string]*Group{},
	// 	nodeClassMemberMap:       classMemberMap{mapper: map[string][]NameSpacer{}},
	// 	interfaceClassMemberMap:  classMemberMap{mapper: map[string][]NameSpacer{}},
	// 	connectionClassMemberMap: classMemberMap{mapper: map[string][]NameSpacer{}},
	// }

	ifaceCounter := map[string]int{}
	for _, e := range d.graph.Edges.Edges {
		ifaceCounter[e.Src]++
		ifaceCounter[e.Dst]++
	}

	nm.Groups = make([]*types.Group, 0, len(d.graph.SubGraphs.SubGraphs))
	for _, s := range d.SortedSubGraphs() {
		group := nm.NewGroup(s.Name)
		if err := group.SetLabels(cfg, getSubGraphLabels(s), getModuleGroupClassLabels(cfg)); err != nil {
			return nil, err
		}
	}

	nm.Nodes = make([]*types.Node, 0, len(d.graph.Nodes.Nodes))
	for _, n := range d.SortedNodes() {
		node := nm.NewNode(n.Name)
		// Note: node.Name can be overwritten later if nodeautoname = true
		// but the name must be DOTID in this function to keep consistency with other graph objects
		err := node.SetLabels(cfg, getNodeLabels(n), getModuleNodeClassLabels(cfg))
		if err != nil {
			return nil, err
		}
		if groups, ok := d.nodeGroups[n.Name]; ok {
			for _, name := range groups {
				group, ok := nm.GroupByName(name)
				if !ok {
					return nil, fmt.Errorf("invalid group name %s", name)
				}
				node.Groups = append(node.Groups, group)
				group.Nodes = append(group.Nodes, node)
			}
		}
	}

	nm.Connections = make([]*types.Connection, 0, len(d.graph.Edges.Edges))
	for _, e := range d.SortedLinks() {
		labels, srcLabels, dstLabels := getEdgeLabels(e)

		srcNode, ok := nm.NodeByName(e.Src)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := srcNode.InterfaceByName(e.SrcPort); ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.SrcPort)
		}
		// new interface
		// interface name can be blank (automatically named later)
		srcIf := srcNode.NewInterface(strings.TrimLeft(e.SrcPort, ":"))
		err := srcIf.SetLabels(cfg, srcLabels, getModuleInterfaceClassLabels(cfg))
		if err != nil {
			return nil, err
		}

		dstNode, ok := nm.NodeByName(e.Dst)
		if !ok {
			return nil, fmt.Errorf("buildSkeleton panic: inconsistent Edge information")
		}
		if _, ok := dstNode.InterfaceByName(e.DstPort); ok {
			// existing named interface
			return nil, fmt.Errorf("duplicated interface name %v", e.DstPort)
		}
		dstIf := dstNode.NewInterface(strings.TrimLeft(e.DstPort, ":"))
		err = dstIf.SetLabels(cfg, dstLabels, getModuleInterfaceClassLabels(cfg))
		if err != nil {
			return nil, err
		}

		srcIf.Opposite = dstIf
		dstIf.Opposite = srcIf

		conn := nm.NewConnection(srcIf, dstIf)
		err = conn.SetLabels(cfg, labels, getModuleConnectionClassLabels(cfg))
		if err != nil {
			return nil, err
		}
		// relational class label for interfaces
		for _, rlabel := range conn.RelationalClassLabels() {
			if rlabel.ClassType == types.ClassTypeInterface {
				srcIf.AddClassLabels(rlabel.Name)
				dstIf.AddClassLabels(rlabel.Name)
			}
		}

		if len(conn.PlaceLabels()) > 0 {
			return nil, fmt.Errorf("connection cannot have placeLabels")
		}
		// Unlabeled links are intentionally permitted: an interface/connection
		// with no explicit class may still receive the "all"/"default" class or
		// a module-provided class later, and a link that legitimately needs no
		// configuration should not be rejected. Do not reinstate a hard error
		// here without a corresponding "default" class requirement.
	}

	return nm, nil
}

// resolveDeployForms settles what every object is materialised as, once all the
// classes have been applied. It is a pass of its own because the answer for one
// object depends on the answers around it: a wire needs something at both of its
// ends, and an end of a wire is there because the wire is.
//
// That propagation used to be written into the virtual flag, where it could not
// be told apart from an author asking for a configuration to be withheld. The
// two questions are separate now - see doc/ROADMAP.md TODO 85.
func resolveDeployForms(cfg *types.Config, nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		form, err := cfg.ResolveDeploy(node)
		if err != nil {
			return err
		}
		node.SetDeployForm(form)
	}

	for _, conn := range nm.Connections {
		form, err := cfg.ResolveConnectionDeploy(conn)
		if err != nil {
			return err
		}
		// Nothing reaches between ends that are not there, whoever would have
		// built it. This is what keeps the far end of a link to a node nobody
		// deploys from describing an interface that never appears.
		if form != types.DeployNone && !connectionEndsMaterialised(conn) {
			form = types.DeployNone
		}
		conn.SetDeployForm(form)
	}

	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			form, err := interfaceDeployForm(cfg, iface)
			if err != nil {
				return err
			}
			iface.SetDeployForm(form)
		}
	}

	return nil
}

func connectionEndsMaterialised(conn *types.Connection) bool {
	for _, end := range []*types.Interface{conn.Src, conn.Dst} {
		if end == nil || end.Node == nil {
			continue
		}
		if !end.Node.IsMaterialised() {
			return false
		}
	}
	return true
}

// interfaceDeployForm works out what one interface is materialised as. An
// interface on a connection takes the connection's form, so the usual case needs
// nothing written; a class may only say so for an interface that has no
// connection to take it from.
func interfaceDeployForm(cfg *types.Config, iface *types.Interface) (string, error) {
	claim, err := cfg.ResolveInterfaceDeployClaim(iface)
	if err != nil {
		return "", err
	}

	// A node that is not there has no interfaces. This overrides whatever the
	// interface's own classes ask for rather than reporting a conflict: the
	// author's answer is already recorded one level up, on the node.
	if !iface.Node.IsMaterialised() {
		return types.DeployNone, nil
	}

	if iface.Connection == nil {
		if claim == "" {
			// A management interface: the platform supplies it without an edge
			// ever being written for it.
			return types.DeployLink, nil
		}
		return claim, nil
	}

	form := iface.Connection.DeployForm()
	if claim != "" && claim != form {
		return "", fmt.Errorf(
			"interface %s is an end of connection %s, which is %s, but its interface classes ask "+
				"for %s. An interface takes the form of its connection; write the form on the "+
				"connection class instead, or leave it off the interface class",
			iface.StringForMessage(), iface.Connection.Name, form, claim)
	}
	return form, nil
}

func checkClasses(cfg *types.Config, nm *types.NetworkModel) error {
	var err error

	// check nodes
	for _, node := range nm.Nodes {
		err = node.SetClasses(cfg, nm)
		if err != nil {
			return err
		}
	}

	// check connections
	for _, conn := range nm.Connections {
		err = conn.SetClasses(cfg, nm)
		if err != nil {
			return err
		}
	}

	// check interfaces
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			err = iface.SetClasses(cfg, nm)
			if err != nil {
				return err
			}
		}
	}

	for _, group := range nm.Groups {
		err = group.SetClasses(cfg, nm)
		if err != nil {
			return err
		}
	}

	// add class members to member referrers
	for _, mr := range nm.MemberReferrers() {
		for _, mc := range mr.GetMemberClasses() {
			classtype, classes, err := mc.GetSpecifiedClasses()
			if err != nil {
				return err
			}

			for _, cls := range classes {
				var members []types.NameSpacer
				switch classtype {
				case types.ClassTypeNode:
					members = nm.NodeClassMembers(cls)
				case types.ClassTypeInterface:
					members = nm.InterfaceClassMembers(cls)
				case types.ClassTypeConnection:
					members = nm.ConnectionClassMembers(cls)
				}
				if len(members) == 0 {
					fmt.Fprintf(os.Stderr, "warning: class %s has no members\n", cls)
					// return fmt.Errorf("class %v has no members", cls)
				}
				for _, memberObject := range members {
					// The object doing the referring is a member of the class
					// it names, so without this it appears among its own
					// members: a node writing a line per peer writes one
					// naming itself. include_self asks for it back where that
					// is what was meant.
					//
					// The check is the one that was written in 0.2.3 and lost
					// when this loop moved here; the blank it left is why
					// bgp_evpn_vxlan_topo1 carried a BGP neighbour statement
					// pointing at its own loopback.
					if !mc.IncludeSelf && memberObject == types.NameSpacer(mr) {
						continue
					}
					member := types.NewMember(cls, classtype, memberObject, mr)
					mr.AddMember(member)
				}
			}
		}
	}

	return nil
}

func addSpecialInterfaces(cfg *types.Config, nm *types.NetworkModel) error {
	if cfg.HasManagementLayer() {
		// set mgmt interfaces on nodes
		name := cfg.ManagementLayer.InterfaceName
		if name == "" {
			name = ManagementInterfaceName
		}
		for _, node := range nm.Nodes {
			_, err := node.CreateManagementInterface(cfg, name)
			if err != nil {
				return err
			}

			// 			iface.SetLabels(cfg, []string{ic.Name})
			// 			if ic := node.GetManagementInterfaceClass(); ic != nil {
			//
			// 				// check that mgmtInterfaceClass is not used in topology
			// 				for _, iface := range node.Interfaces {
			// 					for _, cls := range iface.ClassLabels() {
			// 						if cls == ic.Name {
			// 							return fmt.Errorf("mgmt InterfaceClass should not be specified in topology graph (automatically added)")
			// 						}
			// 					}
			// 				}
			//
			// 				// add management interface
			// 				iface := node.NewInterface(name)
			// 				iface.SetLabels(cfg, []string{ic.Name})
			// 				// iface.parsedLabels = newParsedLabels()
			// 				// iface.parsedLabels.classLabels = append(iface.parsedLabels.classLabels, ic.Name)
			// 				node.mgmtInterface = iface
			// 			}
		}
	}
	return nil
}

// assignNodeNames assign names for unnamed nodes with given name prefix automatically
func assignNodeNames(nm *types.NetworkModel) error {
	prefixMap := map[string][]*types.Node{}
	for _, node := range nm.Nodes {
		prefixMap[node.NamePrefix] = append(prefixMap[node.NamePrefix], node)
	}

	for prefix, nodes := range prefixMap {
		for i, node := range nodes {
			oldName := node.Name
			node.Name = prefix + strconv.Itoa(i+1) // starts with 1
			nm.RenameNode(node, oldName, node.Name)
		}
	}

	return nil
}

// wiredFirst puts the interfaces the platform lays ahead of the rest, keeping
// the order within each group. The numbers a prefix hands out then run without a
// gap over the interfaces that appear in the platform's own files, which is what
// Kathara needs: it names an interface after its position in lab.conf, so a
// number spent on an interface that never gets a line there leaves a hole it
// refuses to start.
//
// The others still get names from the same prefix, continuing after the wired
// ones. A topology that wants a name of its own for a device it builds itself
// says so with a prefix of its own, which is the usual way to write it.
func wiredFirst(interfaces []*types.Interface) []*types.Interface {
	ordered := make([]*types.Interface, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.DeployForm() == types.DeployLink {
			ordered = append(ordered, iface)
		}
	}
	for _, iface := range interfaces {
		if iface.DeployForm() != types.DeployLink {
			ordered = append(ordered, iface)
		}
	}
	return ordered
}

// assignInterfaceNames assign names for unnamed interfaces with given name prefix automatically
func assignInterfaceNames(nm *types.NetworkModel) error {
	for _, node := range nm.Nodes {
		existingNames := map[string]struct{}{}
		prefixMap := map[string][]*types.Interface{} // Interfaces to be named automatically
		for _, iface := range node.Interfaces {
			if iface.Name == "" {
				prefixMap[iface.NamePrefix] = append(prefixMap[iface.NamePrefix], iface)
			} else {
				existingNames[iface.Name] = struct{}{}
			}
		}
		for prefix, interfaces := range prefixMap {
			i := 0
			for _, iface := range wiredFirst(interfaces) {
				oldName := iface.Name
				var name string
				for { // avoid existing names
					name = prefix + strconv.Itoa(i)
					_, exists := existingNames[name]
					if !exists {
						break
					}
					i++ // starts with 0, increment by loop
				}
				iface.Name = name
				iface.Node.RenameInterface(iface, oldName, iface.Name)
				existingNames[iface.Name] = struct{}{}
				i++
			}
		}
	}

	// confirm all interfaces are named
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			if iface.Name == "" {
				return fmt.Errorf("there still exists unnamed interfaces after assignInterfaceNames")
			}
		}
	}

	return nil
}

func assignConnectionNames(nm *types.NetworkModel) error {
	existingNames := map[string]struct{}{}
	prefixMap := map[string][]*types.Connection{} // Connections to be named automatically

	for _, conn := range nm.Connections {
		if conn.Name == "" {
			prefixMap[conn.NamePrefix] = append(prefixMap[conn.NamePrefix], conn)
		} else {
			existingNames[conn.Name] = struct{}{}
		}
	}

	for prefix, connections := range prefixMap {
		i := 0
		for _, conn := range connections {
			var name string
			for { // avoid existing names
				name = prefix + strconv.Itoa(i)
				_, exists := existingNames[name]
				if !exists {
					break
				}
				i++ // starts with 0, increment by loop
			}
			conn.Name = name
			existingNames[conn.Name] = struct{}{}
			i++
		}
	}

	// confirm all connections are named
	for _, conn := range nm.Connections {
		if conn.Name == "" {
			return fmt.Errorf("there still exists unnamed connections after assignConnectionNames")
		}
	}

	return nil
}

func assignSegmentNames(nm *types.NetworkModel) error {
	existingNames := map[string]struct{}{}
	prefixMap := map[string][]*types.NetworkSegment{} // Segments to be named automatically

	// Collect all segments from all layers
	var allSegments []*types.NetworkSegment
	for _, segments := range nm.NetworkSegments {
		allSegments = append(allSegments, segments...)
	}

	for _, segment := range allSegments {
		if segment.Name == "" {
			// SetClasses has already resolved the prefix by tier; redoing it here
			// from GetClasses would put the classes back in label order, which is
			// not the order of precedence (see tieredValues).
			prefixMap[segment.NamePrefix] = append(prefixMap[segment.NamePrefix], segment)
		} else {
			existingNames[segment.Name] = struct{}{}
		}
	}

	for prefix, segments := range prefixMap {
		i := 0
		for _, segment := range segments {
			var name string
			for { // avoid existing names
				name = prefix + strconv.Itoa(i)
				_, exists := existingNames[name]
				if !exists {
					break
				}
				i++ // starts with 0, increment by loop
			}
			segment.Name = name
			existingNames[segment.Name] = struct{}{}
			i++
		}
	}

	// confirm all segments are named
	for _, segment := range allSegments {
		if segment.Name == "" {
			return fmt.Errorf("there still exists unnamed segments after assignSegmentNames")
		}
	}

	return nil
}

func setGivenParameters(nm *types.NetworkModel) error {
	// add parameters only when no same key in namespace
	addParam := func(lo types.LabelOwner, k string, v string) error {
		// IMPORTANT: *types.Connection also satisfies types.NameSpacer, so its
		// case MUST precede the NameSpacer case below. A type switch matches the
		// first applicable case in order; reordering these two would silently
		// stop propagating connection values to the endpoint interfaces.
		switch obj := lo.(type) {
		case *types.Connection:
			// Connection requires special handling: the value is propagated to
			// both endpoint interfaces as well as the Connection itself.
			if !obj.Src.HasParam(k) {
				obj.Src.AddParam(k, v)
			}
			if !obj.Dst.HasParam(k) {
				obj.Dst.AddParam(k, v)
			}
			// Also add to Connection itself (now NameSpacer)
			if !obj.HasParam(k) {
				obj.AddParam(k, v)
			}
		case types.NameSpacer: // *Node, *Interface, *Group (NOT *Connection, matched above)
			if !obj.HasParam(k) {
				obj.AddParam(k, v)
			}
		default:
			return fmt.Errorf("unexpected type %T for setGivenParameters", lo)
		}
		return nil
	}

	for _, lo := range nm.LabelOwners() {
		// set values in ValueLabels
		for k, v := range lo.ValueLabels() {
			err := addParam(lo, k, v)
			if err != nil {
				return err
			}
		}

		// Set values from the classes. The strongest class to give a key wins,
		// which has to be resolved before anything is written: addParam keeps the
		// first value it is handed, and GetClasses is in label order, where a
		// class reached through use: comes after the base class regardless of
		// where it was defined. Two classes of the same tier giving different
		// values were already rejected by SetClasses, so ties keep either one.
		type givenValue struct {
			value string
			tier  int
		}
		given := map[string]givenValue{}
		for _, cls := range lo.GetClasses() {
			loClass, ok := cls.(types.LabelOwnerClass)
			if !ok {
				return fmt.Errorf("unexpected class type %T for setGivenParameters", cls)
			}
			tier := lo.ClassTier(loClass.ClassName())
			for k, v := range loClass.GetGivenValues() {
				if prev, seen := given[k]; seen && prev.tier >= tier {
					continue
				}
				given[k] = givenValue{value: v, tier: tier}
			}
		}
		for k, gv := range given {
			if err := addParam(lo, k, gv.value); err != nil {
				return err
			}
		}
	}

	return nil

	// // set values in ValueLabels
	// for _, node := range nm.Nodes {
	// 	for k, v := range node.valueLabels {
	// 		// check existance (ip numbers may already added)
	// 		if !node.hasParam(k) {
	// 			node.addParam(k, v)
	// 		}
	// 	}
	// 	for _, iface := range node.Interfaces {
	// 		for k, v := range iface.valueLabels {
	// 			// check existance (ip numbers may already added)
	// 			if !iface.hasParam(k) {
	// 				iface.addParam(k, v)
	// 			}
	// 		}
	// 	}
	// }
	// for _, conn := range nm.Connections {
	// 	for k, v := range conn.valueLabels {
	// 		if !conn.Src.hasParam(k) {
	// 			conn.Src.addParam(k, v)
	// 		}
	// 		if !conn.Dst.hasParam(k) {
	// 			conn.Dst.addParam(k, v)
	// 		}
	// 	}
	// }
	// for _, group := range nm.Groups {
	// 	for k, v := range group.valueLabels {
	// 		if !group.hasParam(k) {
	// 			group.addParam(k, v)
	// 		}
	// 	}
	// }
	// return nil
}

func assignIPParameters(cfg *types.Config, nm *types.NetworkModel, verbose bool) error {
	if cfg.HasManagementLayer() {
		err := assignManagementIPAddresses(cfg, nm)
		if err != nil {
			return err
		}
	}

	for _, layer := range cfg.Layers {
		// loopback
		err := assignIPLoopbacks(nm, layer, cfg.GlobalSettings.MaxAddressCount)
		if err != nil {
			return err
		}

		// determine network segment
		segs, err := searchSegments(nm, layer, verbose)
		if err != nil {
			return err
		}
		if len(segs) > 0 {
			nm.NetworkSegments[layer.Name] = segs
		}
		for _, seg := range segs {
			err := seg.SetSegmentLabelsFromRelationalLabels(cfg, layer)
			if err != nil {
				return err
			}
			err = seg.SetClasses(cfg, nm)
			if err != nil {
				return err
			}
		}
		setNeighbors(segs, layer)

		// assign ip addresses
		err = assignIPAddresses(nm, layer, cfg.GlobalSettings.MaxAddressCount)
		if err != nil {
			return err
		}
	}
	return nil
}

func assignParameters(cfg *types.Config, nm *types.NetworkModel) error {

	err := assignNetworkParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignNodeParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignConnectionParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignInterfaceParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignSegmentParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignGroupParameters(cfg, nm)
	if err != nil {
		return err
	}
	err = assignAttachModeParameters(cfg, nm)

	return err
}

// makeRelativeNamespace builds relative namespace for all NameSpacers in the NetworkModel.
// This must be called after all parameters are assigned.
func makeRelativeNamespace(nm *types.NetworkModel) error {
	globals, err := types.InitGloballNameSpace(nm)
	if err != nil {
		return err
	}

	for _, ns := range nm.NameSpacers() {
		if err := ns.BuildRelativeNameSpace(globals); err != nil {
			return err
		}
	}

	return nil
}
