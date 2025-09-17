package model

import (
	"bufio"
	"fmt"
	"os"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"

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

func assignNetworkParameters(cfg *types.Config, nm *types.NetworkModel) error {
	// Note: cfg.NetworkClasses.Name are not used as network name because a network may belong to multiple network classes
	nm.AddParam(NumberReplacerName, cfg.Name)
	return nil
}

func assignNodeParameters(cfg *types.Config, nm *types.NetworkModel) error {
	nodesForParams := map[string][]*types.Node{}
	for _, node := range nm.Nodes {
		node.AddParam(NumberReplacerName, node.Name)
		for key := range node.IterateFlaggedParams() {
			nodesForParams[key] = append(nodesForParams[key], node)
		}
	}

	for key, nodes := range nodesForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		switch rule.Assign {
		default:
			params, err := getParameterCandidates(cfg, rule, len(nodes))
			if err != nil {
				return err
			}
			for i, obj := range nodes {
				obj.AddParam(key, params[i])
			}
		}
	}

	return nil
}

func assignInterfaceParameters(cfg *types.Config, nm *types.NetworkModel) error {

	interfacesForParams := map[string][]*types.Interface{}
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			iface.AddParam(NumberReplacerName, iface.Name)
			// Add node reference parameters with node_ prefix
			for key, value := range node.GetParams() {
				iface.AddParam(NumberPrefixNode+key, value)
			}
			// Add connection reference parameters with conn_ prefix
			if iface.Connection != nil {
				for key, value := range iface.Connection.GetParams() {
					iface.AddParam(NumberPrefixConnection+key, value)
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
	groupsForParams := map[string][]*types.Group{}
	for _, group := range nm.Groups {
		group.AddParam(NumberReplacerName, group.Name)
		for key := range group.IterateFlaggedParams() {
			groupsForParams[key] = append(groupsForParams[key], group)
		}
	}

	// Sort keys for stable parameter assignment order
	keys := make([]string, 0, len(groupsForParams))
	for key := range groupsForParams {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		groups := groupsForParams[key]
		// Sort groups by name for stable parameter assignment
		sort.Slice(groups, func(i, j int) bool {
			return groups[i].Name < groups[j].Name
		})

		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		switch rule.Assign {
		default:
			params, err := getParameterCandidates(cfg, rule, len(groups))
			if err != nil {
				return err
			}
			for i, group := range groups {
				group.AddParam(key, params[i])
			}
		}
	}

	return nil
}

func assignConnectionParameters(cfg *types.Config, nm *types.NetworkModel) error {
	connectionsForParams := map[string][]*types.Connection{}
	for _, conn := range nm.Connections {
		conn.AddParam(NumberReplacerName, conn.Name)
		for key := range conn.IterateFlaggedParams() {
			connectionsForParams[key] = append(connectionsForParams[key], conn)
		}
	}

	for key, connections := range connectionsForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		switch rule.Assign {
		default:
			params, err := getParameterCandidates(cfg, rule, len(connections))
			if err != nil {
				return err
			}
			for i, conn := range connections {
				conn.AddParam(key, params[i])
			}
		}
	}

	return nil
}

func assignSegmentParameters(cfg *types.Config, nm *types.NetworkModel) error {
	segmentsForParams := map[string][]*types.NetworkSegment{}
	for _, segmentList := range nm.NetworkSegments {
		for _, seg := range segmentList {
			// Skip name parameter for segments (not practically needed)
			for key := range seg.IterateFlaggedParams() {
				segmentsForParams[key] = append(segmentsForParams[key], seg)
			}
		}
	}

	for key, segments := range segmentsForParams {
		rule, ok := cfg.ParameterRuleByName(key)
		if !ok {
			return fmt.Errorf("invalid parameter rule name %s", key)
		}
		switch rule.Assign {
		default:
			params, err := getParameterCandidates(cfg, rule, len(segments))
			if err != nil {
				return err
			}
			for i, seg := range segments {
				seg.AddParam(key, params[i])
			}
		}
	}

	return nil
}
