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
const EmptySeparator string = "#NONE#"
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
	ca.belong[k] = append(ca.belong[k], sorterKey{sorter: sorter, group: group})

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
		return "", fmt.Errorf("missing variables in parameters: %W", err)
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

// integrateConfigsFromDependencies integrates config blocks from dependent objects
func integrateConfigsFromDependencies(cfg *types.Config, ca *ConfigAggregator, ns types.NameSpacer, verbose bool) error {
	// Process each dependency class
	depClasses, err := ns.DependClasses()
	if err != nil {
		return err
	}

	for _, depClass := range depClasses {
		deps, err := ns.Depends(depClass)
		if err != nil || len(deps) == 0 {
			continue
		}

		// Collect all configs from each dependency, grouped by config name
		configsByName := make(map[string][]string)
		formatsByName := make(map[string][]string)

		for _, dep := range deps {
			// Find all stored configs for this dependency
			for childKey, childConfigs := range ca.childConfigs {
				if childKey.child == dep {
					for _, cc := range childConfigs {
						configsByName[childKey.name] = append(configsByName[childKey.name], cc.config)
						if len(formatsByName[childKey.name]) == 0 {
							formatsByName[childKey.name] = cc.formats
						}
					}
				}
			}
		}

		// Now integrate each config name with appropriate prefix
		for configName, configs := range configsByName {
			if len(configs) == 0 {
				continue
			}

			var relativeName string
			// Determine the relative name based on the first dependency's type
			if len(deps) > 0 {
				switch obj := deps[0].(type) {
				case *types.Node:
					relativeName = ChildNodesConfigHeader + configName
				case *types.Interface:
					relativeName = ChildInterfacesConfigHeader + configName
				case *types.Connection:
					relativeName = ChildConnectionsConfigHeader + configName
				case *types.NetworkSegment:
					relativeName = ChildSegmentsConfigHeader + configName
				case *types.Group:
					relativeName = ChildGroupsConfigHeader + configName
				case *types.Neighbor:
					relativeName = ChildNeighborsConfigHeader + obj.Layer + NumberSeparator + configName
				case *types.Member:
					relativeName = ChildMembersConfigHeader + obj.ClassType + NumberSeparator + obj.ClassName + NumberSeparator + configName
				default:
					return fmt.Errorf("unsupported dependency type: %T", obj)
				}
			}

			if relativeName == "" {
				continue
			}

			// Merge and add to namespace
			mergedConfig, err := mergeConfigBlocks(cfg, configs, formatsByName[configName])
			if err != nil {
				return fmt.Errorf("error merging configs from %s: %w", depClass, err)
			}

			err = setConfigParamForNameSpace(ns, relativeName, mergedConfig, verbose)
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

	relativeName := SelfConfigHeader + ct.Name
	err = setConfigParamForNameSpace(ns, relativeName, formattedConf, verbose)
	if err != nil {
		return "", err
	}

	return formattedConf, nil
}

func setConfigParamForNameSpace(ns types.NameSpacer, name string, new string, verbose bool) error {
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

	switch obj := ns.(type) {
	case *types.NetworkModel:
		if filedef.Scope != "" && filedef.Scope != types.ClassTypeNetwork {
			return fmt.Errorf("network %s has file template, but the file scope is not network", filedef.Scope)
		}

		path := "./" + filedef.Name
		err := os.WriteFile(path, []byte(conf), 0644)
		if err != nil {
			return err
		}
		if verbose {
			fmt.Fprintf(os.Stderr, " output file %s\n", path)
		}
	case *types.Node:
		if filedef.Scope != "" && filedef.Scope != types.ClassTypeNode {
			return fmt.Errorf("node %s has file template, but the file scope is not node", filedef.Scope)
		}

		// create directory if not exists
		dirname := obj.Name
		f, err := os.Stat(dirname)
		if os.IsNotExist(err) {
			err = os.Mkdir(dirname, 0755)
			if err != nil {
				return err
			}
		} else if !f.IsDir() {
			return fmt.Errorf("creating directory %s fails because something already exists", dirname)
		}

		path := filepath.Join(dirname, filedef.Name)
		err = os.WriteFile(path, []byte(conf), 0644)
		if err != nil {
			return err
		}
		if verbose {
			fmt.Fprintf(os.Stderr, " output file %s\n", path)
		}
	default:
		return fmt.Errorf("network and node can create files, %T given", ns)
	}
	return nil
}

func checkConfigTemplateConditions(ns types.NameSpacer, configTemplate *types.ConfigTemplate, verbose bool) (string, bool) {
	if lo, ok := ns.(types.LabelOwner); ok {
		// check virtual object or not if ns is LabelOwner
		if lo.IsVirtual() {
			return "virtual object", false
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

	// check optional conditions
	switch o := ns.(type) {
	case *types.Interface:
		// check if parent node class of the interface matches
		if !configTemplate.NodeClassCheck(o.Node) {
			return "parent node class condition", false
		}
		// check if connection involves virtual nodes
		if o.Connection != nil {
			if o.Connection.Src != nil && o.Connection.Src.Node != nil && o.Connection.Src.Node.IsVirtual() {
				return "connection from virtual node", false
			}
			if o.Connection.Dst != nil && o.Connection.Dst.Node != nil && o.Connection.Dst.Node.IsVirtual() {
				return "connection to virtual node", false
			}
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
	var files []string

	// Helper function to check if a file name is in a list
	contains := func(list []string, name string) bool {
		for _, item := range list {
			if item == name {
				return true
			}
		}
		return false
	}

	// Generate file list from FileDefinitions
	for _, fileDef := range cfg.FileDefinitions {
		// Skip empty file names
		if fileDef.Name == "" {
			continue
		}

		switch fileDef.Scope {
		case types.ClassTypeNetwork:
			// Network-scope files (root level)
			// Check if the network actually generates this file
			if contains(nm.FilesToGenerate(cfg), fileDef.Name) {
				files = append(files, fileDef.Name)
			}
		case types.ClassTypeNode, "":
			// Node-scope files (node_name/file_name) - Scope="" defaults to node scope
			for _, node := range nm.Nodes {
				if !node.IsVirtual() && contains(node.FilesToGenerate(cfg), fileDef.Name) {
					files = append(files, node.Name+"/"+fileDef.Name)
				}
			}
		}
	}

	// Sort files for consistent output
	sort.Strings(files)

	return files, nil
}
