package model

import (
	"bufio"
	"fmt"
	"os"
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
		if rule.Max-rule.Min < cnt {
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
			segs, ok := nm.NetworkSegments[rule.Layer]
			if !ok {
				return fmt.Errorf("invalid parameter rule %s: layer %s not found", key, rule.Layer)
			}
			params, err := getParameterCandidates(cfg, rule, len(segs))
			if err != nil {
				return err
			}
			for i, seg := range segs {
				for _, iface := range seg.Interfaces {
					iface.addNumber(key, params[i])
				}
			}
		default:
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
