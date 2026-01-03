package model

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/goccy/go-yaml"

	"github.com/cpflat/dot2net/pkg/types"
)

func getParameterCandidates(cfg *types.Config, rule *types.ParameterRule, cnt int) ([]string, error) {
	params := []string{}

	switch rule.Type {
	case "file":
		data, err := os.Open(types.GetRelativeFilePath(rule.SourceFile, cfg))
		if err != nil {
			return nil, err
		}
		defer data.Close()
		scanner := bufio.NewScanner(data)
		for scanner.Scan() {
			param := scanner.Text()
			if param != "" {
				params = append(params, param)
			}
			if len(params) >= cnt {
				break
			}
		}
	default: // "int"
		if rule.Max > 0 && rule.Max-rule.Min < cnt {
			return nil, fmt.Errorf("not enough candidates for %s (%d required)", rule.Name, cnt)
		}
		for i := 0; i < cnt; i++ {
			value := fmt.Sprintf("%s%d%s", rule.Header, rule.Min+i, rule.Footer)
			params = append(params, value)
		}
	}
	return params, nil
}

// =============================================================================
// Generic helpers for distribute mode parameter assignment
// =============================================================================

// collectFlaggedParams collects objects grouped by their flagged parameter rule names.
// T must implement types.ValueOwner (which includes IterateFlaggedParams).
//
// Example:
//
//	nodes := []*types.Node{node1, node2, node3}
//	// If node1 has "vlan_id" flag, node2 has "vlan_id" and "as_number" flags:
//	result := collectFlaggedParams(nodes)
//	// result["vlan_id"] = [node1, node2]
//	// result["as_number"] = [node2]
func collectFlaggedParams[T types.ValueOwner](objects []T) map[string][]T {
	result := make(map[string][]T)
	for _, obj := range objects {
		for key := range obj.IterateFlaggedParams() {
			result[key] = append(result[key], obj)
		}
	}
	return result
}

// assignDistributeParamsToObjects assigns parameters to objects using distribute mode.
// This is the common logic extracted from assignNodeParameters, assignGroupParameters, etc.
//
// Parameters:
//   - cfg: Configuration containing parameter rules
//   - objectsForParams: Map from param_rule name to list of objects that need that parameter
//
// The function:
//  1. Sorts keys (param_rule names) for stable iteration order
//  2. For each key, sorts objects by name for stable parameter assignment
//  3. Looks up each param_rule by name
//  4. Skips attach mode rules (handled separately)
//  5. Generates parameter candidates using getParameterCandidates
//  6. Assigns one parameter value to each object via AddParam
//
// Note: Sorting ensures deterministic parameter assignment regardless of map iteration order.
// This is important for reproducible test results and predictable output.
func assignDistributeParamsToObjects[T types.ValueOwner](
	cfg *types.Config,
	objectsForParams map[string][]T,
) error {
	// Sort keys for stable parameter assignment order
	keys := make([]string, 0, len(objectsForParams))
	for key := range objectsForParams {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		objects := objectsForParams[key]
		// Sort objects by SortKey for stable parameter assignment
		sort.Slice(objects, func(i, j int) bool {
			return objects[i].SortKey() < objects[j].SortKey()
		})

		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		// Skip attach mode rules (handled separately by assignAttachModeParameters)
		if rule.IsAttachMode() {
			continue
		}

		// Currently only "object" assign mode is supported in this generic function.
		// Interface-specific "segment" and "connection" modes are handled separately.
		switch rule.Assign {
		default:
			params, err := getParameterCandidates(cfg, rule, len(objects))
			if err != nil {
				return err
			}
			for i, obj := range objects {
				obj.AddParam(key, params[i])
			}
		}
	}
	return nil
}

func assignNetworkParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Note: cfg.NetworkClasses.Name are not used as network name because a network may belong to multiple network classes
	nm.AddParam(types.ReservedParamName, cfg.Name)
	return nil
}

func assignNodeParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Set basic name parameter for each node
	for _, node := range nm.Nodes {
		node.AddParam(types.ReservedParamName, node.Name)
	}

	// Collect nodes by their flagged param_rule names
	nodesForParams := collectFlaggedParams(nm.Nodes)

	// Assign distribute mode parameters using the generic helper
	return assignDistributeParamsToObjects(cfg, nodesForParams)
}

func assignInterfaceParameters(cfg *types.Config, nm *types.NetworkModel) error {

	interfacesForParams := map[string][]*types.Interface{}
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			iface.AddParam(types.ReservedParamName, iface.Name)
			// Add node reference parameters with node_ prefix
			for key, value := range node.GetParams() {
				iface.AddParam(types.NumberPrefixNode+key, value)
			}
			// Add connection reference parameters with conn_ prefix
			if iface.Connection != nil {
				for key, value := range iface.Connection.GetParams() {
					iface.AddParam(types.NumberPrefixConnection+key, value)
				}
			}
			for key := range iface.IterateFlaggedParams() {
				interfacesForParams[key] = append(interfacesForParams[key], iface)
			}
		}
	}

	for key, ifaces := range interfacesForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		// Skip attach mode rules (handled separately)
		if rule.IsAttachMode() {
			continue
		}
		switch rule.Assign {
		case "segment":
			// assign parameters per segment
			// interfaces in the same segment should have the same parameter
			if rule.Layer == "" {
				return fmt.Errorf("invalid parameter rule %s: layer is required", key)
			}

			// list up target interfaces and segments
			mapper := mapset.NewThreadUnsafeSet(ifaces...)
			targetSegments := []*types.NetworkSegment{}
			targetObjects := map[*types.NetworkSegment][]*types.Interface{}
			segs, ok := nm.NetworkSegments[rule.Layer]
			if !ok {
				return fmt.Errorf("invalid parameter rule %s: layer %s not found", key, rule.Layer)
			}
			for _, seg := range segs {
				for _, conn := range seg.Connections {
					if mapper.Contains(conn.Src) {
						targetObjects[seg] = append(targetObjects[seg], conn.Src)
					}
					if mapper.Contains(conn.Dst) {
						targetObjects[seg] = append(targetObjects[seg], conn.Dst)
					}
				}
				if _, ok := targetObjects[seg]; ok {
					targetSegments = append(targetSegments, seg)
				}
			}

			// assign parameters for parameter-aware objects
			params, err := getParameterCandidates(cfg, rule, len(targetObjects))
			if err != nil {
				return err
			}
			for i, seg := range targetSegments {
				for _, iface := range targetObjects[seg] {
					iface.AddParam(key, params[i])
				}
			}
		case "connection":
			// assign parameters per connection
			// interfaces adjacent to the same connection should have the same parameter

			// list up target interfaces and connections
			// mapper := mapset.NewSet[*Interface](ifaces...)
			mapper := mapset.NewThreadUnsafeSet(ifaces...)
			targetConnections := []*types.Connection{}
			targetObjects := map[*types.Connection][]*types.Interface{}
			for _, conn := range nm.Connections {
				if mapper.Contains(conn.Src) {
					targetObjects[conn] = append(targetObjects[conn], conn.Src)
				}
				if mapper.Contains(conn.Dst) {
					targetObjects[conn] = append(targetObjects[conn], conn.Dst)
				}
				if _, ok := targetObjects[conn]; ok {
					targetConnections = append(targetConnections, conn)
				}
			}

			// assign parameters for parameter-aware objects
			params, err := getParameterCandidates(cfg, rule, len(targetObjects))
			if err != nil {
				return err
			}
			for i, conn := range targetConnections {
				for _, iface := range targetObjects[conn] {
					iface.AddParam(key, params[i])
				}
			}
		default:
			// assign parameters per interface
			params, err := getParameterCandidates(cfg, rule, len(ifaces))
			if err != nil {
				return err
			}
			for i, iface := range ifaces {
				iface.AddParam(key, params[i])
			}
		}
	}

	return nil
}

func assignGroupParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Set basic name parameter for each group
	for _, group := range nm.Groups {
		group.AddParam(types.ReservedParamName, group.Name)
	}

	// Collect groups by their flagged param_rule names
	groupsForParams := collectFlaggedParams(nm.Groups)

	// Assign distribute mode parameters using the generic helper
	return assignDistributeParamsToObjects(cfg, groupsForParams)
}

func assignConnectionParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Set basic name parameter for each connection
	for _, conn := range nm.Connections {
		conn.AddParam(types.ReservedParamName, conn.Name)
	}

	// Collect connections by their flagged param_rule names
	connectionsForParams := collectFlaggedParams(nm.Connections)

	// Assign distribute mode parameters using the generic helper
	return assignDistributeParamsToObjects(cfg, connectionsForParams)
}

func assignSegmentParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Flatten segments from nested map and set basic name parameter
	var allSegments []*types.NetworkSegment
	for _, segmentList := range nm.NetworkSegments {
		for _, seg := range segmentList {
			seg.AddParam(types.ReservedParamName, seg.Name)
			allSegments = append(allSegments, seg)
		}
	}

	// Collect segments by their flagged param_rule names
	segmentsForParams := collectFlaggedParams(allSegments)

	// Assign distribute mode parameters using the generic helper
	return assignDistributeParamsToObjects(cfg, segmentsForParams)
}

// generateValueParamsFromSource generates parameter sets from param_rule source.
// Returns a slice of parameter maps, one per Value to be created.
func generateValueParamsFromSource(cfg *types.Config, rule *types.ParameterRule) ([]map[string]string, error) {
	var params []map[string]string

	if rule.Source == nil {
		return nil, fmt.Errorf("param_rule %s has no source", rule.Name)
	}

	switch rule.Source.Type {
	case "range":
		for i := rule.Source.Start; i <= rule.Source.End; i++ {
			paramSet := make(map[string]string)
			paramSet["value"] = fmt.Sprintf("%d", i)
			paramSet["index"] = fmt.Sprintf("%d", i-rule.Source.Start)
			params = append(params, paramSet)
		}
	case "sequence":
		count := rule.Source.End - rule.Source.Start
		if count <= 0 {
			count = 10 // default count
		}
		for i := 0; i < count; i++ {
			paramSet := make(map[string]string)
			paramSet["value"] = fmt.Sprintf("%d", i)
			paramSet["index"] = fmt.Sprintf("%d", i)
			params = append(params, paramSet)
		}
	case "list":
		for i, item := range rule.Source.Values {
			paramSet := make(map[string]string)
			paramSet["index"] = fmt.Sprintf("%d", i)
			for k, v := range item {
				switch val := v.(type) {
				case string:
					paramSet[k] = val
				default:
					paramSet[k] = fmt.Sprintf("%v", v)
				}
			}
			params = append(params, paramSet)
		}
	case "file":
		path := types.GetRelativeFilePath(rule.Source.File, cfg)
		var err error
		params, err = parseFileSource(path, rule.Source.Format)
		if err != nil {
			return nil, fmt.Errorf("failed to parse source file %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unknown source type: %s", rule.Source.Type)
	}

	// Apply param_format to generate final parameters
	if rule.ParamFormat != nil && len(rule.ParamFormat) > 0 {
		for i := range params {
			for k, tmplVal := range rule.ParamFormat {
				// Simple variable substitution (TODO: use proper template engine)
				params[i][k] = tmplVal
			}
		}
	}

	return params, nil
}

// generateValueParamsFromGenerator generates parameter sets using a module generator.
// The generator string is in format "moduleName.generatorName" (e.g., "clab.filemounts")
func generateValueParamsFromGenerator(cfg *types.Config, nm *types.NetworkModel, rule *types.ParameterRule, target types.ValueOwner) ([]map[string]string, error) {
	if rule.Generator == "" {
		return nil, fmt.Errorf("param_rule %s has no generator", rule.Name)
	}

	// Parse generator string: "moduleName.generatorName"
	parts := strings.Split(rule.Generator, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid generator format %s: expected 'module.generator'", rule.Generator)
	}
	moduleName := parts[0]
	generatorName := parts[1]

	// Find the module that provides this generator
	var generator types.ParameterGenerator
	for _, mod := range cfg.LoadedModules {
		// Check if module name matches (compare with common prefixes)
		if pg, ok := mod.(types.ParameterGenerator); ok {
			// For now, use a simple name matching approach
			// Module names: "containerlab" -> "clab", "tinet" -> "tinet"
			if matchesModuleName(mod, moduleName) {
				generator = pg
				break
			}
		}
	}

	if generator == nil {
		return nil, fmt.Errorf("no generator found for %s", rule.Generator)
	}

	// Call the generator
	params, err := generator.GenerateValueParameters(generatorName, target, cfg, nm)
	if err != nil {
		return nil, fmt.Errorf("generator %s failed: %w", rule.Generator, err)
	}

	// Apply param_format to generate final parameters
	if rule.ParamFormat != nil && len(rule.ParamFormat) > 0 {
		for i := range params {
			for k, tmplVal := range rule.ParamFormat {
				params[i][k] = tmplVal
			}
		}
	}

	return params, nil
}

// matchesModuleName checks if a module matches the given short name
func matchesModuleName(mod types.Module, shortName string) bool {
	// Map short names to module type names
	// This is a simple approach; could be enhanced with module registration
	switch shortName {
	case "clab":
		// Check if module is containerlab
		_, ok := mod.(interface{ GetClabModuleName() string })
		if !ok {
			// Fallback: check by type name
			typeName := fmt.Sprintf("%T", mod)
			return strings.Contains(strings.ToLower(typeName), "clab")
		}
		return true
	case "tinet":
		typeName := fmt.Sprintf("%T", mod)
		return strings.Contains(strings.ToLower(typeName), "tinet")
	default:
		return false
	}
}

// attachValuesToOwner creates Values and attaches them to a ValueOwner
func attachValuesToOwner(owner types.ValueOwner, rule *types.ParameterRule, paramSets []map[string]string) {
	for i, paramSet := range paramSets {
		value := types.NewValue(rule.Name, owner, i)
		for k, v := range paramSet {
			value.AddParam(k, v)
		}
		owner.AddValue(value)
	}
}

// hasFlaggedParam checks if a NameSpacer has a specific param flag set
func hasFlaggedParam(ns types.NameSpacer, paramName string) bool {
	for key := range ns.IterateFlaggedParams() {
		if key == paramName {
			return true
		}
	}
	return false
}

// assignAttachModeParameters processes all attach mode param_rules
func assignAttachModeParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Collect all attach mode param_rules and their target objects
	for _, rule := range cfg.ParameterRules {
		if !rule.IsAttachMode() {
			continue
		}

		// Determine if this is source-based or generator-based
		isGenerator := rule.Generator != ""

		// For source-based rules, generate once and share across all targets
		var sharedParamSets []map[string]string
		if !isGenerator && rule.Source != nil {
			var err error
			sharedParamSets, err = generateValueParamsFromSource(cfg, rule)
			if err != nil {
				return fmt.Errorf("failed to generate values for %s: %w", rule.Name, err)
			}
		}

		// Helper function to attach values to a target
		attachToTarget := func(target types.ValueOwner) error {
			if !hasFlaggedParam(target, rule.Name) {
				return nil
			}

			var paramSets []map[string]string
			if isGenerator {
				// Generator: call for each target
				var err error
				paramSets, err = generateValueParamsFromGenerator(cfg, nm, rule, target)
				if err != nil {
					return err
				}
			} else {
				// Source: use shared param sets
				paramSets = sharedParamSets
			}

			if len(paramSets) > 0 {
				attachValuesToOwner(target, rule, paramSets)
			}
			return nil
		}

		// Find all objects that reference this param_rule and attach Values
		// Check nodes
		for _, node := range nm.Nodes {
			if err := attachToTarget(node); err != nil {
				return err
			}
		}

		// Check interfaces
		for _, node := range nm.Nodes {
			for _, iface := range node.Interfaces {
				if err := attachToTarget(iface); err != nil {
					return err
				}
			}
		}

		// Check connections
		for _, conn := range nm.Connections {
			if err := attachToTarget(conn); err != nil {
				return err
			}
		}

		// Check groups
		for _, group := range nm.Groups {
			if err := attachToTarget(group); err != nil {
				return err
			}
		}

		// Check segments
		for _, segmentList := range nm.NetworkSegments {
			for _, seg := range segmentList {
				if err := attachToTarget(seg); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// determineFileFormat determines the file format from explicit format or file extension.
// Returns "yaml", "json", "csv", or "text" (default).
func determineFileFormat(path, explicitFormat string) string {
	if explicitFormat != "" {
		return explicitFormat
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".csv":
		return "csv"
	default:
		return "text"
	}
}

// parseFileSource parses a file based on its format and returns parameter sets.
// Each parameter set includes an "index" field.
func parseFileSource(path, explicitFormat string) ([]map[string]string, error) {
	format := determineFileFormat(path, explicitFormat)

	switch format {
	case "yaml":
		return parseYAMLFile(path)
	case "json":
		return parseJSONFile(path)
	case "csv":
		return parseCSVFile(path)
	default:
		return parseTextFile(path)
	}
}

// parseTextFile parses a text file with one value per line (original behavior).
func parseTextFile(path string) ([]map[string]string, error) {
	data, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer data.Close()

	var params []map[string]string
	scanner := bufio.NewScanner(data)
	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			paramSet := make(map[string]string)
			paramSet["value"] = line
			paramSet["index"] = fmt.Sprintf("%d", i)
			params = append(params, paramSet)
			i++
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return params, nil
}

// parseYAMLFile parses a YAML file containing an array of objects.
func parseYAMLFile(path string) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var items []map[string]interface{}
	if err := yaml.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return convertToStringMaps(items)
}

// parseJSONFile parses a JSON file containing an array of objects.
func parseJSONFile(path string) ([]map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var items []map[string]interface{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return convertToStringMaps(items)
}

// parseCSVFile parses a CSV file with header row defining parameter names.
func parseCSVFile(path string) ([]map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 1 {
		return nil, nil // Empty file
	}

	headers := records[0]
	var params []map[string]string
	for i, record := range records[1:] {
		paramSet := make(map[string]string)
		for j, header := range headers {
			if j < len(record) {
				paramSet[header] = record[j]
			}
		}
		paramSet["index"] = fmt.Sprintf("%d", i)
		params = append(params, paramSet)
	}
	return params, nil
}

// convertToStringMaps converts []map[string]interface{} to []map[string]string with index.
func convertToStringMaps(items []map[string]interface{}) ([]map[string]string, error) {
	var params []map[string]string
	for i, item := range items {
		paramSet := make(map[string]string)
		for k, v := range item {
			switch val := v.(type) {
			case string:
				paramSet[k] = val
			default:
				paramSet[k] = fmt.Sprintf("%v", v)
			}
		}
		paramSet["index"] = fmt.Sprintf("%d", i)
		params = append(params, paramSet)
	}
	return params, nil
}

// generateValueReferenceParams generates values_xxx params for ValueOwners.
// This must be called after makeRelativeNamespace so Values have their relative params.
func generateValueReferenceParams(cfg *types.Config, nm *types.NetworkModel) error {
	for _, vo := range nm.ValueOwners() {
		// Collect all param_rule names that this ValueOwner references
		// (either through attached Values or through flagged params)
		paramRuleNames := make(map[string]bool)

		// From attached Values
		for _, v := range vo.GetValues() {
			paramRuleNames[v.ParamRuleName] = true
		}

		// From flagged params (param_rules referenced via NodeClass.Parameters, etc.)
		for flag := range vo.IterateFlaggedParams() {
			if rule, ok := cfg.ParameterRuleByName(flag); ok && rule.IsAttachMode() {
				paramRuleNames[flag] = true
			}
		}

		// For each param_rule, generate values_xxx params
		for ruleName := range paramRuleNames {
			rule, ok := cfg.ParameterRuleByName(ruleName)
			if !ok {
				continue
			}

			// Get Values for this param_rule
			values := vo.GetValuesByParamRule(ruleName)

			// For each ConfigTemplate in the param_rule, generate combined output
			for _, ct := range rule.ConfigTemplates {
				if ct.Name == "" {
					continue
				}

				paramName := "values_" + ct.Name

				if len(values) == 0 {
					// Set empty string when no Values are attached
					vo.SetRelativeParam(paramName, "")
					continue
				}

				// Generate config for each Value and combine
				var configs []string
				for _, v := range values {
					conf, err := generateValueConfig(v, ct)
					if err != nil {
						return err
					}
					if conf != "" {
						configs = append(configs, conf)
					}
				}

				// Combine configs and add to owner's relative params
				if len(configs) > 0 {
					combined := combineValueConfigs(cfg, configs, ct)
					vo.SetRelativeParam(paramName, combined)
				} else {
					vo.SetRelativeParam(paramName, "")
				}
			}
		}
	}
	return nil
}

// generateValueConfig generates config output for a single Value using a ConfigTemplate.
func generateValueConfig(v *types.Value, ct *types.ConfigTemplate) (string, error) {
	if ct.ParsedTemplate == nil {
		return "", nil
	}
	return getConfig(ct.ParsedTemplate, v.GetRelativeParams())
}

// combineValueConfigs combines multiple config outputs based on FormatStyle.
func combineValueConfigs(cfg *types.Config, configs []string, ct *types.ConfigTemplate) string {
	// Get block separator from FormatStyle if available
	separator := "\n"
	if ct.Format != "" {
		if fs, ok := cfg.FormatStyleByName(ct.Format); ok {
			separator = fs.GetMergeBlockSeparator()
		}
	}
	return strings.Join(configs, separator)
}
