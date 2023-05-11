package model

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
)

// format
const FormatShell string = "shell"
const FormatFile string = "file"

// style
const StyleLocal string = "local"
const StyleVtysh string = "vtysh"
const StyleFRRVtysh string = "frr-vtysh"

type ConfigFiles struct {
	mapper map[string]*ConfigFile
}

func (files *ConfigFiles) newConfigBlock(cfg *Config, ct *ConfigTemplate) (*configBlock, error) {
	filedef, ok := cfg.FileDefinitionByName(ct.File)
	if !ok {
		return nil, fmt.Errorf("undefined file %s", ct.File)
	}
	file := files.GetFile(filedef.Name)
	if file == nil {
		file = &ConfigFile{
			FileDefinition: filedef,
		}
		files.addFile(file)
	}

	block := &configBlock{
		priority: ct.Priority,
		style:    ct.Style,
	}
	file.blocks = append(file.blocks, block)
	return block, nil
}

func (files *ConfigFiles) addFile(file *ConfigFile) {
	files.mapper[file.FileDefinition.Name] = file
}

func (files *ConfigFiles) GetFile(filename string) *ConfigFile {
	if file, ok := files.mapper[filename]; ok {
		return file
	} else {
		return nil
	}
}

func (files *ConfigFiles) FileNames() []string {
	filenames := []string{}
	for filename := range files.mapper {
		if filename != "" {
			filenames = append(filenames, filename)
		}
	}
	sort.SliceStable(filenames, func(i, j int) bool {
		return filenames[i] > filenames[j]
	})
	return filenames
}

func (files *ConfigFiles) GetFiles() []*ConfigFile {
	ret := []*ConfigFile{}
	for _, filename := range files.FileNames() {
		ret = append(ret, files.GetFile(filename))
	}
	return ret
}

func (files *ConfigFiles) GetEmbeddedConfig() *ConfigFile {
	return files.mapper[""]
}

type ConfigFile struct {
	Content        []string
	FileDefinition *FileDefinition // nil if config is described in platform configs (e.g., tinet spec file)

	blocks []*configBlock
}

type configBlock struct {
	config   string
	priority int
	style    string
}

func loadTemplate(tpl []string, path string) (*template.Template, error) {
	if len(tpl) == 0 && path == "" {
		fmt.Printf("%+v\n", tpl)
		return nil, fmt.Errorf("empty config template")
	} else if len(tpl) == 0 {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		buf := convertLineFeed(string(bytes), "\n")
		return template.New("").Parse(buf)
	} else if path == "" {
		buf := strings.Join(tpl, "\n")
		return template.New("").Parse(buf)
	} else {
		bytes, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		buf := strings.Join(tpl, "\n") + "\n" + convertLineFeed(string(bytes), "\n")
		return template.New("").Parse(buf)
	}
}

func getConfig(tpl *template.Template, numbers map[string]string) (string, error) {
	writer := new(strings.Builder)
	err := tpl.Execute(writer, numbers)
	if err != nil {
		return "", err
	}
	return writer.String(), nil
}

func generateConfig(cfg *Config, nm *NetworkModel, outputPlatform string) error {
	for _, node := range nm.Nodes {
		files := &ConfigFiles{mapper: map[string]*ConfigFile{}}

		for _, cls := range node.Labels.ClassLabels {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return fmt.Errorf("undefined NodeClass name %v", cls)
			}
			for _, nct := range nc.ConfigTemplates {
				if !nct.platformSet.Contains(outputPlatform) {
					continue
				}
				block, err := files.newConfigBlock(cfg, &nct)
				if err != nil {
					return err
				}

				conf, err := getConfig(nct.parsedTemplate, node.RelativeNumbers)
				if err != nil {
					return err
				}
				block.config = conf
			}
		}

		for _, iface := range node.Interfaces {
			for _, cls := range iface.Labels.ClassLabels {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined InterfaceClass name %v", cls)
				}
				for _, ict := range ic.ConfigTemplates {
					if !ict.platformSet.Contains(outputPlatform) {
						continue
					}
					if !(ict.NodeClass == "" || node.HasClass(ict.NodeClass)) {
						// interfaces of different node class -> ignore
						continue
					}

					block, err := files.newConfigBlock(cfg, &ict)
					if err != nil {
						return err
					}

					conf, err := getConfig(ict.parsedTemplate, iface.RelativeNumbers)
					if err != nil {
						return err
					}
					block.config = conf
				}
			}

			if iface.Connection == nil {
				continue
			}
			for _, cls := range iface.Connection.Labels.ClassLabels {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined ConnectionClass name %v", cls)
				}
				for _, cct := range cc.ConfigTemplates {
					if !cct.platformSet.Contains(outputPlatform) {
						continue
					}
					if !(cct.NodeClass == "" || node.HasClass(cct.NodeClass)) {
						// interfaces of different node class -> ignore
						continue
					}

					block, err := files.newConfigBlock(cfg, &cct)
					if err != nil {
						return err
					}

					conf, err := getConfig(cct.parsedTemplate, iface.RelativeNumbers)
					if err != nil {
						return err
					}
					block.config = conf
				}
			}
		}

		for _, file := range files.mapper {
			file.Content, _ = mergeConfig(file.blocks, file.FileDefinition.Format)
		}
		node.Files = files
	}

	return nil
}

func mergeConfig(blocks []*configBlock, format string) ([]string, error) {
	switch format {
	case FormatShell:
		return mergeConfigShell(blocks)
	case FormatFile:
		return mergeConfigFile(blocks)
	default:
		return mergeConfigFile(blocks)
	}
}

func mergeConfigShell(blocks []*configBlock) ([]string, error) {
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].priority < blocks[j].priority
	})

	buf := []string{}
	for _, block := range blocks {
		switch block.style {
		case "", StyleLocal:
			buf = append(buf, strings.Split(block.config, "\n")...)
		case StyleVtysh:
			lines := strings.Split(block.config, "\n")
			buf = append(buf, "vtysh -c \""+strings.Join(lines, "\" -c \"")+"\"")
		case StyleFRRVtysh:
			lines := []string{"conf t"}
			lines = append(lines, strings.Split(block.config, "\n")...)
			buf = append(buf, "vtysh -c \""+strings.Join(lines, "\" -c \"")+"\"")
		default:
			fmt.Fprintf(os.Stderr, "warning: unknown style %s\n", block.style)
			buf = append(buf, strings.Split(block.config, "\n")...)
		}
	}
	return buf, nil
}

func mergeConfigFile(blocks []*configBlock) ([]string, error) {
	sort.SliceStable(blocks, func(i, j int) bool {
		return blocks[i].priority < blocks[j].priority
	})
	buf := []string{}
	for _, block := range blocks {
		buf = append(buf, strings.Split(block.config, "\n")...)
	}
	return buf, nil
}
