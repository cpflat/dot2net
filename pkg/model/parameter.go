package model

import (
	"bufio"
	"fmt"
	"os"

	mapset "github.com/deckarep/golang-set/v2"
)

func getParameterCandidates(cfg *Config, rule *ParameterRule, cnt int) ([]string, error) {
	params := []string{}

	switch rule.Type {
	case "file":
		data, err := os.Open(getPath(rule.SourceFile, cfg))
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

func assignNodeParameters(cfg *Config, nm *NetworkModel) error {
	nodesForParams := map[string][]*Node{}
	for _, node := range nm.Nodes {
		node.addNumber(NumberReplacerName, node.Name)
		for key := range node.iterNumbered() {
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
				obj.addNumber(key, params[i])
			}
		}
	}

	return nil
}

func assignInterfaceParameters(cfg *Config, nm *NetworkModel) error {

	interfacesForParams := map[string][]*Interface{}
	for _, node := range nm.Nodes {
		for _, iface := range node.Interfaces {
			iface.addNumber(NumberReplacerName, iface.Name)
			for key := range iface.iterNumbered() {
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
			targetSegments := []*SegmentMembers{}
			targetObjects := map[*SegmentMembers][]*Interface{}
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
					iface.addNumber(key, params[i])
				}
			}
		case "connection":
			// assign parameters per connection
			// interfaces adjacent to the same connection should have the same parameter

			// list up target interfaces and connections
			// mapper := mapset.NewSet[*Interface](ifaces...)
			mapper := mapset.NewThreadUnsafeSet(ifaces...)
			targetConnections := []*Connection{}
			targetObjects := map[*Connection][]*Interface{}
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
					iface.addNumber(key, params[i])
				}
			}
		default:
			// assign parameters per interface
			params, err := getParameterCandidates(cfg, rule, len(ifaces))
			if err != nil {
				return err
			}
			for i, iface := range ifaces {
				iface.addNumber(key, params[i])
			}
		}
	}

	return nil
}

func assignGroupParameters(cfg *Config, nm *NetworkModel) error {
	groupsForParams := map[string][]*Group{}
	for _, group := range nm.Groups {
		group.addNumber(NumberReplacerName, group.Name)
		for key := range group.iterNumbered() {
			groupsForParams[key] = append(groupsForParams[key], group)
		}
	}

	for key, groups := range groupsForParams {
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
				group.addNumber(key, params[i])
			}
		}
	}

	return nil
}
