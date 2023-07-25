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
