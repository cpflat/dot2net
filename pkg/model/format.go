package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/cpflat/dot2net/pkg/types"
)

const EmptyOutput string = "#EMPTY#"

// EmptySeparator moved to pkg/types so that modules can use it too.
const EmptySeparator = types.EmptySeparator
const NChars int = 32

type ConfigAggregator struct {
	belong map[belongKey][]sorterKey
	groups map[sorterKey][]*ConfigBlock

	// Parent-child config block management
	childConfigs map[childConfigKey][]*ChildConfig // child's config blocks
	parentChild  map[parentChildKey][]string       // parent -> children mapping
}

type childConfigKey struct {
	child types.NameSpacer // child NameSpacer object reference
	name  string           // config template name
}

type parentChildKey struct {
	parent    string // parent NameSpacer's StringForMessage()
	childType string // "interface", "node", etc.
	name      string // config template name
}

type ChildConfig struct {
	config  string
	formats []string
}

func initConfigAggregator() *ConfigAggregator {
	return &ConfigAggregator{
		belong:       map[belongKey][]sorterKey{},
		groups:       map[sorterKey][]*ConfigBlock{},
		childConfigs: map[childConfigKey][]*ChildConfig{},
		parentChild:  map[parentChildKey][]string{},
	}
}

func (ca *ConfigAggregator) addSorterChildren(sorter types.NameSpacer, ns types.NameSpacer, group string) error {
	// list up candidate children objects that generate grouped configs for the sorter
	k := belongKey{namespacer: ns, group: group}
	sk := sorterKey{sorter: sorter, group: group}
	// The object graph is not a tree: a node is a child of both the network
	// model and of every group it belongs to, so the same object can be reached
	// several times from one sorter. Registering it twice would make the sorter
	// emit its config blocks twice, so stop at the first arrival.
	for _, registered := range ca.belong[k] {
		if registered == sk {
			return nil
		}
	}
	ca.belong[k] = append(ca.belong[k], sk)

	classes, err := ns.ChildClasses()
	if err != nil {
		return err
	}
	for _, cls := range classes {
		objs, err := ns.Childs(cls)
		if err != nil {
			return err
		}
		for _, child := range objs {
			ca.addSorterChildren(sorter, child, group)
		}
	}
	return nil
}

func (ca *ConfigAggregator) addSorter(sorter types.NameSpacer, group string) {
	ca.addSorterChildren(sorter, sorter, group)
}

// addChildConfig adds a child config block for parent retrieval during integration
func (ca *ConfigAggregator) addChildConfig(child types.NameSpacer, name string, config string, formats []string) {
	key := childConfigKey{
		child: child,
		name:  name,
	}
	ca.childConfigs[key] = append(ca.childConfigs[key], &ChildConfig{
		config:  config,
		formats: formats,
	})
}

func (ca *ConfigAggregator) addConfigBlock(ns types.NameSpacer, group string, block *ConfigBlock, top bool) {
	// add config blocks for sorter objects corresponding to parent objects
	bk := belongKey{namespacer: ns, group: group}
	for _, sk := range ca.belong[bk] {
		if top {
			ca.groups[sk] = append([]*ConfigBlock{block}, ca.groups[sk]...)

		} else {
			ca.groups[sk] = append(ca.groups[sk], block)
		}
	}
}

func (ca *ConfigAggregator) getConfigBlocks(ns types.NameSpacer, group string, verbose bool) []string {
	sk := sorterKey{sorter: ns, group: group}
	blocks := ca.groups[sk]

	if verbose && len(blocks) > 0 {
		fmt.Fprintf(os.Stderr, " sorting %d config blocks for group %s:\n", len(blocks), group)
		for i, cb := range blocks {
			fmt.Fprintf(os.Stderr, "  [%d] Priority=%d: %q\n", i, cb.Priority, headN(cb.Block, NChars))
		}
	}

	// sort considering Priority
	sort.SliceStable(blocks, func(i, j int) bool { return blocks[i].Priority < blocks[j].Priority })

	if verbose && len(blocks) > 0 {
		fmt.Fprintf(os.Stderr, " after sorting by Priority:\n")
		for i, cb := range blocks {
			fmt.Fprintf(os.Stderr, "  [%d] Priority=%d: %q\n", i, cb.Priority, headN(cb.Block, NChars))
		}
	}

	ret := make([]string, 0, len(blocks))
	for _, cb := range blocks {
		ret = append(ret, cb.Block)
	}
	return ret
}

// Parent-child config block management methods

// Register parent-child relationships during Phase 0
func (ca *ConfigAggregator) registerParentChild(parent types.NameSpacer, cfg *types.Config) {
	// Check all possible config templates of parent
	parentTemplates := parent.GetPossibleConfigTemplates(cfg)

	// For each template with a name, register potential children
	for _, ct := range parentTemplates {
		if ct.Name != "" {
			// Register all child types that might contribute config blocks
			classes, err := parent.ChildClasses()
			if err != nil {
				continue
			}

			for _, cls := range classes {
				children, err := parent.Childs(cls)
				if err != nil {
					continue
				}

				// Determine child type string
				var childType string
				if len(children) > 0 {
					switch children[0].(type) {
					case *types.Interface:
						childType = "interface"
					case *types.Node:
						childType = "node"
					case *types.Group:
						childType = "group"
					default:
						continue
					}

					// Register parent-child mapping
					key := parentChildKey{
						parent:    parent.StringForMessage(),
						childType: childType,
						name:      ct.Name,
					}

					for _, child := range children {
						ca.parentChild[key] = append(ca.parentChild[key], child.StringForMessage())
					}
				}
			}
		}
	}
}

type sorterKey struct {
	sorter types.NameSpacer
	group  string
}

type belongKey struct {
	namespacer types.NameSpacer
	group      string
}

type ConfigBlock struct {
	Block    string
	Priority int
}

// style
// const StyleDefault string = "default" // merge with line feed
// const StyleComma string = "comma"     // merge with comma

// format
// const FormatShell string = "shell"
// const FormatFile string = "file"

// style
// const StyleLocal string = "local"
// const StyleVtysh string = "vtysh"
// const StyleFRRVtysh string = "frr-vtysh"

// type ConfigData struct {
// 	Data           string
// 	ConfigTemplate *ConfigTemplate
// }

// Legacy
// type ConfigFiles struct {
// 	mapper map[string]*ConfigFile
// }
//
// func newConfigFiles() *ConfigFiles {
// 	return &ConfigFiles{mapper: map[string]*ConfigFile{}}
// }
//
// func (files *ConfigFiles) newConfigBlock(cfg *Config, ct *ConfigTemplate) (*configBlock, error) {
// 	filedef, ok := cfg.FileDefinitionByName(ct.File)
// 	if !ok {
// 		return nil, fmt.Errorf("undefined file %s", ct.File)
// 	}
// 	file := files.GetFile(filedef.Name)
// 	if file == nil {
// 		file = &ConfigFile{
// 			FileDefinition: filedef,
// 		}
// 		files.addFile(file)
// 	}
//
// 	block := &configBlock{
// 		priority: ct.Priority,
// 		style:    ct.Style,
// 	}
// 	file.blocks = append(file.blocks, block)
// 	return block, nil
// }
//
// func (files *ConfigFiles) addFile(file *ConfigFile) {
// 	files.mapper[file.FileDefinition.Name] = file
// }
//
// func (files *ConfigFiles) GetFile(filename string) *ConfigFile {
// 	if file, ok := files.mapper[filename]; ok {
// 		return file
// 	} else {
// 		return nil
// 	}
// }
//
// func (files *ConfigFiles) FileNames() []string {
// 	filenames := []string{}
// 	for filename := range files.mapper {
// 		if filename != "" {
// 			filenames = append(filenames, filename)
// 		}
// 	}
// 	sort.SliceStable(filenames, func(i, j int) bool {
// 		return filenames[i] > filenames[j]
// 	})
// 	return filenames
// }
//
// func (files *ConfigFiles) GetFiles() []*ConfigFile {
// 	ret := []*ConfigFile{}
// 	for _, filename := range files.FileNames() {
// 		ret = append(ret, files.GetFile(filename))
// 	}
// 	return ret
// }
//
// func (files *ConfigFiles) GetEmbeddedConfig() *ConfigFile {
// 	return files.mapper[""]
// }
//
// type ConfigFile struct {
// 	Content        []string
// 	FileDefinition *FileDefinition // nil if config is described in platform configs (e.g., tinet spec file)
//
// 	blocks []*configBlock
// }
//
// type configBlock struct {
// 	config   string
// 	priority int
// 	style    string
// }

// for verbose output
func headN(s string, n int) string {
	runes := []rune(s)
	if len(runes) < n {
		return s
	}
	return string(runes[:n])
}

func getConfig(tpl *template.Template, namespace map[string]string) (string, error) {
	if tpl == nil {
		return "", fmt.Errorf("template is nil")
	}
	tpl = tpl.Option("missingkey=error")

	writer := new(strings.Builder)
	err := tpl.Execute(writer, namespace)
	if err != nil {
		return "", fmt.Errorf("missing variables in parameters: %w", err)
	}
	return writer.String(), nil
}

// func getTargetFiles(cfg *Config, nm *NetworkModel, localFiles *ConfigFiles, ct *ConfigTemplate) (*ConfigFiles, error) {
// 	// check target file is local or global
// 	filedef, ok := cfg.FileDefinitionByName(ct.File)
// 	if !ok {
// 		return nil, fmt.Errorf("invalid file %s specified in a template", ct.File)
// 	}
// 	if filedef.Shared {
// 		return nm.Files, nil
// 	} else {
// 		return localFiles, nil
// 	}
// }

func generateConfigFiles(cfg *types.Config, nm *types.NetworkModel, verbose bool) error {
	if verbose {
		fmt.Printf("Object Classes: \n")
		for _, ns := range nm.NameSpacers() {
			fmt.Printf(" %s\n", ns.StringForMessage())
		}
	}

	// Phase 0: Pre-Analysis - register sorter candidates and parent-child relationships
	ca := initConfigAggregator()
	if verbose {
		fmt.Printf("Phase 0: Pre-Analysis (Sorter and Parent-Child relationships)\n")
	}
	for _, ns := range nm.NameSpacers() {
		checkSorterObjects(cfg, ca, ns)
		ca.registerParentChild(ns, cfg)
	}

	// Dependency-Ordered Individual Config Generation
	if verbose {
		fmt.Printf("Dependency-Ordered Individual Config Generation\n")
	}
	reorderedNameSpacers, err2 := reorderNameSpacers(nm.NameSpacers())
	if err2 != nil {
		return fmt.Errorf("failure in reordering NameSpacers: %w", err2)
	}

	if verbose {
		fmt.Printf("Processing order: ")
		for i, ns := range reorderedNameSpacers {
			if i > 0 {
				fmt.Printf(" -> ")
			}
			fmt.Printf("%s", ns.StringForMessage())
		}
		fmt.Printf("\n")
	}

	// Process individual configs in dependency order
	for _, ns := range reorderedNameSpacers {
		err3 := generateIndividualConfigs(cfg, ca, ns, verbose)
		if err3 != nil {
			return fmt.Errorf("failure in generating individual configs for %s: %w", ns.StringForMessage(), err3)
		}
	}

	return nil
}

// parseBlockReference parses a block reference string and returns its components
// Supported formats:
//   - self_configname
//   - interfaces_configname
//   - nodes_configname
//   - segments_layer_configname
//   - neighbors_layer_configname
//   - members_classtype_classname_configname
func parseBlockReference(blockRef string) (objectType, layer, classType, className, configName string, err error) {
	switch {
	case strings.HasPrefix(blockRef, types.SelfConfigHeader):
		return "self", "", "", "", strings.TrimPrefix(blockRef, types.SelfConfigHeader), nil

	case strings.HasPrefix(blockRef, types.ChildInterfacesConfigHeader):
		return "interface", "", "", "", strings.TrimPrefix(blockRef, types.ChildInterfacesConfigHeader), nil

	case strings.HasPrefix(blockRef, types.ChildNodesConfigHeader):
		return "node", "", "", "", strings.TrimPrefix(blockRef, types.ChildNodesConfigHeader), nil

	case strings.HasPrefix(blockRef, types.ChildConnectionsConfigHeader):
		return "connection", "", "", "", strings.TrimPrefix(blockRef, types.ChildConnectionsConfigHeader), nil

	case strings.HasPrefix(blockRef, types.ChildGroupsConfigHeader):
		return "group", "", "", "", strings.TrimPrefix(blockRef, types.ChildGroupsConfigHeader), nil

	case strings.HasPrefix(blockRef, types.ChildSegmentsConfigHeader):
		// segments_layer_configname format
		rest := strings.TrimPrefix(blockRef, types.ChildSegmentsConfigHeader)
		parts := strings.SplitN(rest, types.NumberSeparator, 2)
		if len(parts) != 2 {
			return "", "", "", "", "", fmt.Errorf("invalid segment reference: %s (expected format: segments_layer_configname)", blockRef)
		}
		return "segment", parts[0], "", "", parts[1], nil

	case strings.HasPrefix(blockRef, types.ChildNeighborsConfigHeader):
		// neighbors_layer_configname format
		rest := strings.TrimPrefix(blockRef, types.ChildNeighborsConfigHeader)
		parts := strings.SplitN(rest, types.NumberSeparator, 2)
		if len(parts) != 2 {
			return "", "", "", "", "", fmt.Errorf("invalid neighbor reference: %s (expected format: neighbors_layer_configname)", blockRef)
		}
		return "neighbor", parts[0], "", "", parts[1], nil

	case strings.HasPrefix(blockRef, types.ChildMembersConfigHeader):
		// members_classtype_classname_configname format
		rest := strings.TrimPrefix(blockRef, types.ChildMembersConfigHeader)
		parts := strings.SplitN(rest, types.NumberSeparator, 3)
		if len(parts) != 3 {
			return "", "", "", "", "", fmt.Errorf("invalid member reference: %s (expected format: members_classtype_classname_configname)", blockRef)
		}
		return "member", "", parts[0], parts[1], parts[2], nil

	default:
		return "", "", "", "", "", fmt.Errorf("unsupported block reference: %s", blockRef)
	}
}

// buildRelativeParamName constructs the parameter name from parsed components
func buildRelativeParamName(objectType, layer, classType, className, configName string) string {
	switch objectType {
	case "self":
		return types.SelfConfigHeader + configName
	case "interface":
		return types.ChildInterfacesConfigHeader + configName
	case "node":
		return types.ChildNodesConfigHeader + configName
	case "connection":
		return types.ChildConnectionsConfigHeader + configName
	case "segment":
		return types.ChildSegmentsConfigHeader + layer + types.NumberSeparator + configName
	case "group":
		return types.ChildGroupsConfigHeader + configName
	case "neighbor":
		return types.ChildNeighborsConfigHeader + layer + types.NumberSeparator + configName
	case "member":
		return types.ChildMembersConfigHeader + classType + types.NumberSeparator + className + types.NumberSeparator + configName
	default:
		return ""
	}
}

// collectConfigBlocks collects config blocks from namespace based on block references
func collectConfigBlocks(ns types.NameSpacer, blockRefs []string) ([]string, error) {
	var blocks []string
	relativeParams := ns.GetRelativeParams()

	for _, ref := range blockRefs {
		// Parse the block reference
		objectType, layer, classType, className, configName, err := parseBlockReference(ref)
		if err != nil {
			return nil, err
		}

		// Build parameter name
		paramName := buildRelativeParamName(objectType, layer, classType, className, configName)
		if paramName == "" {
			return nil, fmt.Errorf("failed to build parameter name for: %s", ref)
		}

		// Get the config block from namespace
		block, exists := relativeParams[paramName]
		if !exists {
			return nil, fmt.Errorf("config block not found: %s (parameter name: %s)", ref, paramName)
		}

		blocks = append(blocks, block)
	}

	return blocks, nil
}

// aggregationParamName returns the parameter under which the config named
// configName of dep is aggregated into its parent's namespace. It is the
// producing counterpart of buildRelativeParamName, which resolves the same
// names when a template refers to them, so the two must stay in step.
func aggregationParamName(dep types.NameSpacer, configName string) (string, error) {
	switch obj := dep.(type) {
	case *types.Node:
		return types.ChildNodesConfigHeader + configName, nil
	case *types.Interface:
		return types.ChildInterfacesConfigHeader + configName, nil
	case *types.Connection:
		return types.ChildConnectionsConfigHeader + configName, nil
	case *types.NetworkSegment:
		// A segment belongs to exactly one layer, and a parent sees the segments
		// of every layer at once, so the layer is part of the name.
		return types.ChildSegmentsConfigHeader + obj.Layer + types.NumberSeparator + configName, nil
	case *types.Group:
		return types.ChildGroupsConfigHeader + configName, nil
	case *types.Neighbor:
		return types.ChildNeighborsConfigHeader + obj.Layer + types.NumberSeparator + configName, nil
	case *types.Member:
		return types.ChildMembersConfigHeader + obj.ClassType + types.NumberSeparator + obj.ClassName + types.NumberSeparator + configName, nil
	default:
		return "", fmt.Errorf("unsupported dependency type: %T", obj)
	}
}

// setEmptyAggregationParams pre-declares the aggregation parameters that
// objects of depClass could contribute to ns, leaving them empty. Real configs
// overwrite them later: setConfigParamForNameSpace treats an empty previous
// value as absent.
//
// Neighbors and members are not covered: their parameter names carry the layer
// or the member class, and neither can be enumerated from the configuration
// alone. Segments can, because a segment class declares the layer it belongs to.
func setEmptyAggregationParams(cfg *types.Config, ns types.NameSpacer, depClass string, verbose bool) error {
	// names collects the parameter names that objects of depClass could set.
	var names []string
	addNames := func(header string, cts []*types.ConfigTemplate) {
		for _, ct := range cts {
			if ct.Name != "" {
				names = append(names, header+ct.Name)
			}
		}
	}

	switch depClass {
	case types.ClassTypeNode:
		for _, c := range cfg.NodeClasses {
			addNames(types.ChildNodesConfigHeader, c.ConfigTemplates)
		}
	case types.ClassTypeInterface:
		for _, c := range cfg.InterfaceClasses {
			addNames(types.ChildInterfacesConfigHeader, c.ConfigTemplates)
		}
	case types.ClassTypeConnection:
		for _, c := range cfg.ConnectionClasses {
			addNames(types.ChildConnectionsConfigHeader, c.ConfigTemplates)
		}
	case types.ClassTypeGroup:
		for _, c := range cfg.GroupClasses {
			addNames(types.ChildGroupsConfigHeader, c.ConfigTemplates)
		}
	case types.ClassTypeSegment:
		for _, c := range cfg.SegmentClasses {
			addNames(types.ChildSegmentsConfigHeader+c.Layer+types.NumberSeparator, c.ConfigTemplates)
		}
	default:
		return nil
	}

	for _, name := range names {
		if ns.HasRelativeParam(name) {
			continue
		}
		if err := setConfigParamForNameSpace(ns, name, EmptyOutput, nil, verbose); err != nil {
			return err
		}
	}
	return nil
}

// integrateConfigsFromDependencies integrates config blocks from dependent objects
func integrateConfigsFromDependencies(cfg *types.Config, ca *ConfigAggregator, ns types.NameSpacer, verbose bool) error {
	// Process each dependency class
	depClasses, err := ns.DependClasses()
	if err != nil {
		return err
	}

	for _, depClass := range depClasses {
		deps, err := ns.Depends(depClass)
		if err != nil {
			continue
		}

		// A named template of the dependency class contributes an aggregation
		// parameter even when no object produced anything for it, so that a
		// template referring to it still renders. Without this, a group with a
		// single node has no {{ .connections_... }} at all and fails to
		// template, while its neighbour group with two nodes succeeds.
		// Undefined names are left absent so that a typo is still an error.
		if err := setEmptyAggregationParams(cfg, ns, depClass, verbose); err != nil {
			return err
		}
		if len(deps) == 0 {
			continue
		}

		// Collect the configs of every dependency, grouped by the aggregation
		// parameter they contribute to. The parameter name is derived per
		// dependency rather than from deps[0]: segments of different layers
		// reach the network model in a single call, and merging them into one
		// parameter would silently mix the layers.
		configsByParam := make(map[string][]string)
		formatsByParam := make(map[string][]string)

		for _, dep := range deps {
			// Find all stored configs for this dependency
			for childKey, childConfigs := range ca.childConfigs {
				if childKey.child != dep {
					continue
				}
				paramName, err := aggregationParamName(dep, childKey.name)
				if err != nil {
					return err
				}
				for _, cc := range childConfigs {
					configsByParam[paramName] = append(configsByParam[paramName], cc.config)
					if len(formatsByParam[paramName]) == 0 {
						formatsByParam[paramName] = cc.formats
					}
				}
			}
		}

		for relativeName, configs := range configsByParam {
			if len(configs) == 0 {
				continue
			}

			// Merge and add to namespace
			mergedConfig, err := mergeConfigBlocks(cfg, configs, formatsByParam[relativeName])
			if err != nil {
				return fmt.Errorf("error merging configs from %s: %w", depClass, err)
			}

			err = setConfigParamForNameSpace(ns, relativeName, mergedConfig, nil, verbose)
			if err != nil {
				return fmt.Errorf("error adding configs to namespace: %w", err)
			}

			if verbose {
				fmt.Fprintf(os.Stderr, " integrated %d configs from %s as %s\n", len(configs), depClass, relativeName)
			}
			// DEBUG: Show details for segment integration
		}
	}

	return nil
}

func generateIndividualConfigs(cfg *types.Config, ca *ConfigAggregator, ns types.NameSpacer, verbose bool) error {
	// First, integrate dependent config blocks into this namespace
	// This handles both hierarchical (child) and non-hierarchical dependencies
	err := integrateConfigsFromDependencies(cfg, ca, ns, verbose)
	if err != nil {
		return fmt.Errorf("error integrating configs from dependencies: %w", err)
	}

	// Then proceed with normal config generation
	configTemplates := ns.GetPossibleConfigTemplates(cfg)
	if verbose {
		fmt.Fprintf(os.Stderr, "processing individual configs for %s (%d possible templates)\n", ns.StringForMessage(), len(configTemplates))
	}

	// Reorder ConfigTemplates based on their dependencies (Level 2 dependencies)
	reordered, err := reorderConfigTemplates(configTemplates)
	if err != nil {
		return fmt.Errorf("failure in reordering config templates for %s: %w", ns.StringForMessage(), err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "processing order: %v\n", reordered)
	}

	for _, ct := range reordered {
		// Generate config block if conditions are met
		var conf string
		reason, met := checkConfigTemplateConditions(ns, ct, verbose)
		if met {
			if verbose {
				fmt.Fprintf(os.Stderr, "templating individual config for %s with %s\n", ns.StringForMessage(), ct.String())
			}

			// Check if blocks functionality is used
			hasBlocks := len(ct.Blocks.Before) > 0 || len(ct.Blocks.After) > 0

			if hasBlocks || ct.Style == types.ConfigTemplateStyleSort {
				// Use new blocks processing engine for:
				// 1. Templates with blocks.before/after
				// 2. Sort style templates (to integrate with blocks)
				conf, err = processConfigTemplateWithBlocks(cfg, ca, ns, ct, verbose)
				if err != nil {
					return err
				}
			} else {
				// Use traditional processing for templates without blocks
				conf, err = generateConfigBlock(ns, ct)
				if err != nil {
					return err
				}
			}
		} else {
			if verbose {
				fmt.Fprintf(os.Stderr, " skip templating for %s with %s because %s\n", ns.StringForMessage(), ct.String(), reason)
			}
			conf = EmptyOutput
		}

		// Store config block for grouping (Group accumulation)
		// Note: For sort style, this is already handled in processConfigTemplateWithBlocks
		if met && ct.Group != "" && ct.Style != types.ConfigTemplateStyleSort {
			ca.addConfigBlock(ns, ct.Group, &ConfigBlock{Block: conf, Priority: ct.Priority}, false)
			if verbose {
				fmt.Fprintf(os.Stderr, " store config to group %s (%q)\n", ct.Group, headN(conf, NChars))
			}
		}

		// Note: Sort processing is now handled in processConfigTemplateWithBlocks

		// Add to self's namespace if ct.Name is specified
		if ct.Name != "" {
			// addSelfConfigToNameSpace formats the config and stores it to namespace
			// It returns the formatted config for use in childConfigs
			formattedConf, err := addSelfConfigToNameSpace(cfg, ns, conf, ct, verbose)
			if err != nil {
				return err
			}

			// Store the FORMATTED config in ConfigBlockManager for parent to retrieve later
			// This ensures parents get the properly formatted config blocks
			ca.addChildConfig(ns, ct.Name, formattedConf, ct.GetNamespaceFormats())
			if verbose {
				fmt.Fprintf(os.Stderr, " stored config for parent retrieval: %s\n", ct.Name)
			}
		}

		// Output file if ct.File is specified
		if ct.File != "" {
			err = outputConfigFile(cfg, ns, conf, ct, verbose)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func checkSorterObjects(cfg *types.Config, ca *ConfigAggregator, ns types.NameSpacer) {
	cts := ns.GetPossibleConfigTemplates(cfg)
	for _, ct := range cts {
		// Check if the config template is valid and sorter
		_, met := checkConfigTemplateConditions(ns, ct, false)
		if met && ct.Style == types.ConfigTemplateStyleSort {
			ca.addSorter(ns, ct.SortGroup)
		}
	}
}

// func generateConfigBlock(ct *types.ConfigTemplate, ns types.NameSpacer) (string, error) {
func generateConfigBlock(ns types.NameSpacer, configTemplate *types.ConfigTemplate) (string, error) {
	if content, ok := configTemplate.RawContent(); ok {
		return content, nil
	}
	conf, err := getConfig(configTemplate.ParsedTemplate, ns.GetRelativeParams())
	if err != nil {
		return EmptyOutput, fmt.Errorf("templating failure for %s, %w", ns.StringForMessage(), err)
	}
	return conf, nil
}

// processConfigTemplateWithBlocks processes a config template with blocks.before and blocks.after
func processConfigTemplateWithBlocks(cfg *types.Config, ca *ConfigAggregator, ns types.NameSpacer, ct *types.ConfigTemplate, verbose bool) (string, error) {
	var allBlocks []string

	// 1. Collect blocks.before if specified
	if len(ct.Blocks.Before) > 0 {
		beforeBlocks, err := collectConfigBlocks(ns, ct.Blocks.Before)
		if err != nil {
			return "", fmt.Errorf("error collecting blocks.before for %s: %w", ns.StringForMessage(), err)
		}
		allBlocks = append(allBlocks, beforeBlocks...)
		if verbose {
			fmt.Fprintf(os.Stderr, "  collected %d blocks.before\n", len(beforeBlocks))
		}
	}

	// 2. Process main template or sort results
	if ct.Style == types.ConfigTemplateStyleSort {
		// For sort style, collect grouped blocks directly without merging
		// (merged later in step 4 with before/after blocks for optimization)
		selfConf, err := generateConfigBlock(ns, ct)
		if err != nil {
			return "", err
		}
		ca.addConfigBlock(ns, ct.SortGroup, &ConfigBlock{Block: selfConf, Priority: ct.Priority}, true)
		sortedBlocks := ca.getConfigBlocks(ns, ct.SortGroup, verbose)

		// Append sorted blocks directly to allBlocks (not merging here)
		// This avoids double merge: previously merged here and again at step 4
		allBlocks = append(allBlocks, sortedBlocks...)
		if verbose {
			fmt.Fprintf(os.Stderr, " collected %d config blocks in group %s\n", len(sortedBlocks)-1, ct.SortGroup)
		}
	} else if len(ct.Template) > 0 || ct.SourceFile != "" {
		// Normal template processing
		mainBlock, err := generateConfigBlock(ns, ct)
		if err != nil {
			return "", err
		}
		if mainBlock != "" && mainBlock != EmptyOutput {
			allBlocks = append(allBlocks, mainBlock)
		}
	}

	// 3. Collect blocks.after if specified
	if len(ct.Blocks.After) > 0 {
		afterBlocks, err := collectConfigBlocks(ns, ct.Blocks.After)
		if err != nil {
			return "", fmt.Errorf("error collecting blocks.after for %s: %w", ns.StringForMessage(), err)
		}
		allBlocks = append(allBlocks, afterBlocks...)
		if verbose {
			fmt.Fprintf(os.Stderr, "  collected %d blocks.after\n", len(afterBlocks))
		}
	}

	// 4. Merge all blocks if we have multiple, otherwise return the single block
	if len(allBlocks) == 0 {
		return EmptyOutput, nil
	} else if len(allBlocks) == 1 {
		return allBlocks[0], nil
	} else {
		// Merge all blocks using the config template's assembly formats
		mergedConf, err := mergeConfigBlocks(cfg, allBlocks, ct.GetAssemblyFormats())
		if err != nil {
			return "", fmt.Errorf("error merging blocks for %s: %w", ns.StringForMessage(), err)
		}
		return mergedConf, nil
	}
}

// addSelfConfigToNameSpace formats and stores config block to namespace
// Returns the formatted config for use by other components (e.g., childConfigs)
func addSelfConfigToNameSpace(cfg *types.Config, ns types.NameSpacer, conf string, ct *types.ConfigTemplate, verbose bool) (string, error) {
	formats := ct.GetNamespaceFormats()

	// format config block in the same way with merging config blocks
	formattedConf, err := formatSingleConfigBlock(cfg, conf, formats)
	if err != nil {
		return "", fmt.Errorf("error on formatting config block of %s, %w", ns.StringForMessage(), err)
	}

	// format lines
	// conf, err = formatConfigLines(cfg, conf, []string{ct.Format})
	// if err != nil {
	// 	return fmt.Errorf("error on formatting config lines of %s, %w", ns.StringForMessage(), err)
	// }

	relativeName := types.SelfConfigHeader + ct.Name
	err = setConfigParamForNameSpace(ns, relativeName, formattedConf, ct, verbose)
	if err != nil {
		return "", err
	}

	return formattedConf, nil
}

// joinHookBlocks puts one hook block after another on a line of its own. The
// seam is trimmed because each block already stands on its own: leaving their
// edges in place would put an empty command between them.
func joinHookBlocks(first, second string) string {
	return strings.TrimRight(first, "\n") + "\n" + strings.TrimLeft(second, "\n")
}

func setConfigParamForNameSpace(ns types.NameSpacer, name string, new string, ct *types.ConfigTemplate, verbose bool) error {
	if new == EmptyOutput {
		// if new config is empty, set "" only when no previous parameter
		if !ns.HasRelativeParam(name) {
			ns.SetRelativeParam(name, "")
			if verbose {
				fmt.Fprintf(os.Stderr, " set empty relative param to %s: %s \n", ns.StringForMessage(), name)
			}
		}
		return nil
	} else {
		if ns.HasRelativeParam(name) {
			prev, _ := ns.GetParamValue(name)
			if prev != "" {
				// A hook name carries both a module's part and the topology's.
				// The module's comes first whichever is rendered first, so that
				// what a topology asked for runs after the ground is prepared.
				if ct != nil && types.HookConfigNames[strings.TrimPrefix(name, types.SelfConfigHeader)] {
					if ct.ModuleProvided {
						ns.SetRelativeParam(name, joinHookBlocks(new, prev))
					} else {
						ns.SetRelativeParam(name, joinHookBlocks(prev, new))
					}
					return nil
				}
				// if neither is empty (duplicated configuration), raise error
				return fmt.Errorf(
					// "parameter name %s of object %s duplicated (existing parameter: %s)",
					// relativeName, parent.StringForMessage(), values,
					"parameter name %s of object %s duplicated (existing parameter: %q, new parameter: %q)",
					name, ns.StringForMessage(), headN(prev, NChars), headN(new, NChars),
				)
			}
			// if previous parameter is empty, just overwrite
		}
	}
	ns.SetRelativeParam(name, new)
	if verbose {
		fmt.Fprintf(os.Stderr, " set relative param to %s: %s (%q)\n", ns.StringForMessage(),
			name, headN(new, NChars))
	}
	return nil
}

func outputConfigFile(cfg *types.Config, ns types.NameSpacer, conf string, ct *types.ConfigTemplate, verbose bool) error {
	if conf == EmptyOutput {
		return nil
	}

	filedef, ok := cfg.FileDefinitionByName(ct.File)
	if !ok {
		return fmt.Errorf("undefined file format %s", ct.File)
	}

	// format lines
	conf, err := formatConfigLines(cfg, conf, filedef.GetFormats())
	if err != nil {
		return err
	}

	// dirname is the directory the file is written into, relative to the output
	// root; empty means the output root itself.
	var dirname, filename string
	switch obj := ns.(type) {
	case *types.NetworkModel:
		if filedef.Scope != "" && filedef.Scope != types.ClassTypeNetwork {
			return fmt.Errorf("network %s has file template, but the file scope is not network", filedef.Scope)
		}

		// Network-scope files belong to no host, so they stay at the output
		// root even when groups split the output directory.
		// For network scope, object name is empty (use Name directly)
		filename = filedef.GetFileName("")
		dirname = filedef.Subdir
	case *types.Group:
		if filedef.Scope != types.ClassTypeGroup {
			return fmt.Errorf("group has a template for file %s, but its scope is %q, not group", filedef.Name, filedef.Scope)
		}

		filename = filedef.GetFileName(obj.Name)
		if filedef.GetOutputLocation() != "root" {
			// A group-scope file goes into the group's own directory by
			// default, the same way a node-scope file goes into the node's.
			// When the group is also the output directory of its member nodes
			// (GlobalSettings.OutputGroupClass), this is the very directory
			// their files land under, so the host packs as one directory.
			dirname = filepath.Join(obj.Name, filedef.Subdir)
		}
	case *types.Node:
		if filedef.Scope != "" && filedef.Scope != types.ClassTypeNode {
			return fmt.Errorf("node %s has file template, but the file scope is not node", filedef.Scope)
		}

		outputPath, err := obj.OutputPath(cfg, filedef)
		if err != nil {
			return err
		}
		dirname, filename = filepath.Split(outputPath)
	default:
		return fmt.Errorf("network, group and node can create files, %T given", ns)
	}

	path, err := prepareOutputPath(dirname, filename)
	if err != nil {
		return err
	}
	perm := os.FileMode(0644)
	if filedef.Executable {
		perm = 0755
	}
	if err := os.WriteFile(path, []byte(conf), perm); err != nil {
		return err
	}
	if verbose {
		fmt.Fprintf(os.Stderr, " output file %s\n", path)
	}
	return nil
}

// prepareOutputPath creates dirname (relative to the current directory, which
// is the output root) if needed and returns the path to write filename to.
func prepareOutputPath(dirname, filename string) (string, error) {
	if dirname == "" {
		return "./" + filename, nil
	}
	f, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(dirname, 0755); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	} else if !f.IsDir() {
		return "", fmt.Errorf("creating directory %s fails because something already exists", dirname)
	}
	return filepath.Join(dirname, filename), nil
}

func checkConfigTemplateConditions(ns types.NameSpacer, configTemplate *types.ConfigTemplate, verbose bool) (string, bool) {
	if lo, ok := ns.(types.LabelOwner); ok {
		// Two separate reasons to write nothing. virtual is the author asking
		// for this object's configuration to be withheld; an object that is not
		// materialised has nothing for a configuration to describe. The second
		// reaches further than the object it is written on, because a node
		// nobody deploys leaves its interfaces and the links to them with
		// nothing at their end either - resolveDeployForms works that out.
		//
		// A wiring template is exempt from the first. It is not the object's
		// configuration: it tells the platform to lay a link, which is deploy's
		// question, not virtual's. Withholding it here would make virtual mean
		// different things on different platforms - containerlab writes its
		// wiring per connection and would keep the link, while TiNET and Kathara
		// write theirs per interface and would lose it, leaving Kathara with a
		// gap in lab.conf that nothing reports.
		if lo.IsVirtual() && !configTemplate.RequiredLink {
			return "virtual object", false
		}
		if !lo.IsMaterialised() {
			return "object is not materialised", false
		}

		// check classname meets if ns is LabelOwner
		classType, className := configTemplate.GetClassInfo()

		var check bool
		switch classType {
		case types.ClassTypeConnection:
			// 新仕様: Connectionオブジェクト自体がConnectionClassを持つ場合
			if conn, ok := lo.(*types.Connection); ok {
				check = conn.HasClass(className)
			} else if iface, ok := lo.(*types.Interface); ok {
				// 旧仕様: InterfaceからConnectionのクラスを参照
				check = iface.Connection.HasClass(className)
			} else {
				check = false
			}
		case types.ClassTypeMember(types.ClassTypeConnection, ""):
			// 新仕様: Connectionオブジェクト自体がConnectionClassを持つ場合
			if conn, ok := lo.(*types.Connection); ok {
				check = conn.HasClass(className)
			} else if iface, ok := lo.(*types.Interface); ok {
				// 旧仕様: InterfaceからConnectionのクラスを参照
				check = iface.Connection.HasClass(className)
			} else {
				check = false
			}
		default:
			check = lo.HasClass(className)
		}
		if !check {
			if verbose {
				fmt.Fprintf(os.Stderr, " class %s is not included in %v (actual classes: %v)\n",
					className, ns.StringForMessage(), lo.ClassLabels())
			}
			return "non-matching class", false
		}
	}

	// A wiring template tells the platform to lay a link, so it is written only
	// where the platform lays one. The connection is what it describes whichever
	// object it is scoped to: containerlab writes one entry per connection,
	// TiNET lists interfaces per node, and both are asking for the same wire.
	//
	// Nothing here has to look at the nodes at either end. A connection reaching
	// a node nobody deploys is not materialised, and neither is an interface on
	// it, so both were turned away above.
	if configTemplate.RequiredLink {
		var conn *types.Connection
		switch o := ns.(type) {
		case *types.Interface:
			conn = o.Connection
		case *types.Connection:
			conn = o
		}
		if conn != nil && conn.DeployForm() != types.DeployLink {
			return "connection is not an actual link", false
		}
	}

	// check optional conditions
	switch o := ns.(type) {
	case *types.Interface:
		// check if parent node class of the interface matches
		if !configTemplate.NodeClassCheck(o.Node) {
			return "parent node class condition", false
		}
	case *types.Neighbor:
		// check if self node class of neighbor object match
		if !configTemplate.NeighborNodeClassCheck(o.Neighbor.Node) {
			return "self node class condition for neighbor object", false
		}
		// check if neighbor node class of neighbor object match
		if !configTemplate.NodeClassCheck(o.Self.Node) {
			return "neighbor node class condition for neighbor object", false
		}
	case *types.Member:
		switch t := o.Referrer.(type) {
		case *types.Node:
			// pass
		case *types.Interface:
			if !(configTemplate.NodeClassCheck(t.Node)) {
				return "member node class condition for member object", false
			}
		default:
			panic(fmt.Sprintf("panic: unexpected type of Member Referer: %T", t))
		}
	default:
	}

	// check required_params condition
	// Check both regular params and relative params (self_* parameters)
	if len(configTemplate.RequiredParams) > 0 {
		params := ns.GetParams()
		relativeParams := ns.GetRelativeParams()
		// Merge params for checking
		allParams := make(map[string]string, len(params)+len(relativeParams))
		for k, v := range params {
			allParams[k] = v
		}
		for k, v := range relativeParams {
			allParams[k] = v
		}
		if !configTemplate.HasRequiredParams(allParams) {
			if verbose {
				fmt.Fprintf(os.Stderr, " required_params %v not satisfied for %v\n",
					configTemplate.RequiredParams, ns.StringForMessage())
			}
			return "missing required params", false
		}
	}

	return "", true
}

//func checkConfigTemplatesConditions(ns types.NameSpacer, configTemplates []*types.ConfigTemplate) ([]*types.ConfigTemplate, error) {
//	ret := make([]*types.ConfigTemplate, 0, len(configTemplates))
//
//	for _, ct := range configTemplates {
//		fail := false
//		switch o := ns.(type) {
//		case *types.Interface:
//			// keep config template only when node condition is satisfied
//			if !ct.NodeClassCheck(o.Node) {
//				fail = true
//			}
//		case *types.Neighbor:
//			if !ct.NeighborNodeClassCheck(o.Neighbor.Node) {
//				fail = true
//			}
//			if !ct.NodeClassCheck(o.Self.Node) {
//				fail = true
//			}
//		default:
//		}
//		if !fail || ct.Empty {
//			ret = append(ret, ct)
//		}
//	}
//	return ret, nil
//}

// func classifyConfigTemplates(cts []*types.ConfigTemplate) ([]*types.ConfigTemplate, []*types.ConfigTemplate) {
// 	named := []*types.ConfigTemplate{}
// 	output := []*types.ConfigTemplate{}
// 	for _, ct := range cts {
// 		if ct.Name != "" {
// 			named = append(named, ct)
// 		}
// 		if ct.File != "" {
// 			output = append(output, ct)
// 		}
// 	}
// 	return named, output
// }

// mergeConfigBlocks merges config blocks that are already formatted in Format Phase
// IMPORTANT: This function is for Merge Phase only and does NOT apply formatSingleConfigBlock
// to avoid double formatting. Blocks should be formatted before being passed to this function.
func mergeConfigBlocks(cfg *types.Config, blocks []string, formats []string) (string, error) {
	validBlocks := make([]string, 0, len(blocks))
	for _, block := range blocks {
		// ignore empty config blocks
		if block == "" || block == EmptyOutput {
			continue
		}
		validBlocks = append(validBlocks, block)
	}

	if len(validBlocks) == 0 {
		return EmptyOutput, nil
	}

	// Generate separator and result prefix/suffix for merge
	separator := ""
	var resultPrefix, resultSuffix string
	for _, format := range formats {
		if format != "" {
			fmtstyle, ok := cfg.FormatStyleByName(format)
			if !ok {
				return "", fmt.Errorf("undefined file format %s", format)
			}
			if separator != "" && fmtstyle.GetMergeBlockSeparator() != "" {
				return "", fmt.Errorf("BlockSeparator conflicted in file formats %v", formats)
			}
			separator = fmtstyle.GetMergeBlockSeparator()

			// Apply result prefix/suffix (new feature in v0.6.0)
			resultPrefix = resultPrefix + fmtstyle.GetMergeResultPrefix()
			resultSuffix = fmtstyle.GetMergeResultSuffix() + resultSuffix
		}
	}
	switch separator {
	case "":
		separator = "\n"
	case EmptySeparator:
		separator = ""
	}

	// Merge config blocks
	merged := strings.Join(validBlocks, separator)

	// Wrap merged result with prefix/suffix if specified
	if resultPrefix != "" || resultSuffix != "" {
		merged = resultPrefix + merged + resultSuffix
	}

	return merged, nil
}

func formatSingleConfigBlock(cfg *types.Config, block string, formats []string) (string, error) {
	if block == EmptyOutput {
		return EmptyOutput, nil
	}

	block, err := formatConfigLines(cfg, block, formats)
	if err != nil {
		return "", err
	}

	// add prefix and suffix
	var prefix, suffix string
	for _, format := range formats {
		if format == "" {
			continue
		} else {
			fmtstyle, ok := cfg.FormatStyleByName(format)
			if !ok {
				return "", fmt.Errorf("undefined file format %s", format)
			}
			blockPrefix := fmtstyle.GetFormatBlockPrefix()
			blockSuffix := fmtstyle.GetFormatBlockSuffix()

			prefix = prefix + blockPrefix
			suffix = blockSuffix + suffix
		}
	}

	result := prefix + block + suffix
	return result, nil
}

func formatConfigLines(cfg *types.Config, conf string, formats []string) (string, error) {
	if conf == EmptyOutput {
		return EmptyOutput, nil
	}
	var separator string
	// format lines
	for _, format := range formats {
		if format == "" {
			continue
		}
		segmentedConf := strings.Split(conf, "\n")
		fmtstyle, ok := cfg.FormatStyleByName(format)
		if !ok {
			return "", fmt.Errorf("undefined file format %s", format)
		}

		linePrefix := fmtstyle.GetFormatLinePrefix()
		lineSuffix := fmtstyle.GetFormatLineSuffix()
		lineSeparator := fmtstyle.GetFormatLineSeparator()

		newConf := []string{}
		for _, line := range segmentedConf {
			newConf = append(newConf, linePrefix+line+lineSuffix)
		}

		switch lineSeparator {
		case "":
			separator = "\n"
		case EmptySeparator:
			separator = ""
		default:
			separator = lineSeparator
		}
		conf = strings.Join(newConf, separator)
	}

	return conf, nil
}

// func mergeConfig(blocks []*ConfigData, format string) ([]string, error) {
// 	switch format {
// 	case FormatShell:
// 		return mergeConfigShell(blocks)
// 	case FormatFile:
// 		return mergeConfigFile(blocks)
// 	default:
// 		return mergeConfigFile(blocks)
// 	}
// }
//
// func mergeConfigShell(blocks []*ConfigData) ([]string, error) {
// 	sort.SliceStable(blocks, func(i, j int) bool {
// 		return blocks[i].priority < blocks[j].priority
// 	})
//
// 	buf := []string{}
// 	for _, block := range blocks {
// 		switch block.style {
// 		case "", StyleLocal:
// 			buf = append(buf, strings.Split(block.config, "\n")...)
// 		case StyleVtysh:
// 			lines := strings.Split(block.config, "\n")
// 			buf = append(buf, "vtysh -c \""+strings.Join(lines, "\" -c \"")+"\"")
// 		case StyleFRRVtysh:
// 			lines := []string{"conf t"}
// 			lines = append(lines, strings.Split(block.config, "\n")...)
// 			buf = append(buf, "vtysh -c \""+strings.Join(lines, "\" -c \"")+"\"")
// 		default:
// 			fmt.Fprintf(os.Stderr, "warning: unknown style %s\n", block.style)
// 			buf = append(buf, strings.Split(block.config, "\n")...)
// 		}
// 	}
// 	return buf, nil
// }
//
// func mergeConfigFile(blocks []*configBlock) ([]string, error) {
// 	sort.SliceStable(blocks, func(i, j int) bool {
// 		return blocks[i].priority < blocks[j].priority
// 	})
// 	buf := []string{}
// 	for _, block := range blocks {
// 		buf = append(buf, strings.Split(block.config, "\n")...)
// 	}
// 	return buf, nil
// }

// ListGeneratedFiles returns a list of files that would be generated by generateConfigFiles
func ListGeneratedFiles(cfg *types.Config, nm *types.NetworkModel, verbose bool) ([]string, error) {
	generated, err := generatedFiles(cfg, nm)
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(generated))
	for _, file := range generated {
		files = append(files, file.path)
	}

	// Sort files for consistent output
	sort.Strings(files)

	return files, nil
}
