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

func getConfig(tpl *template.Template, numbers map[string]string) (string, error) {
	writer := new(strings.Builder)
	err := tpl.Execute(writer, numbers)
	if err != nil {
		return "", err
	}
	return writer.String(), nil
}

func generateConfigBlock(cfg *Config, ct *ConfigTemplate, files *ConfigFiles, ns NameSpacer, outputPlatform string) error {
	// skip if platform does not match
	if !ct.platformSet.Contains(outputPlatform) {
		return nil
	}

	// skip if noode class does not match
	switch o := ns.(type) {
	case *Node:
		// pass
	case *Interface:
		if !(ct.NodeClass == "" || o.Node.HasClass(ct.NodeClass)) {
			return nil
		}
	case *Neighbor:
		if !(ct.NodeClass == "" || o.Self.Node.HasClass(ct.NodeClass)) {
			return nil
		}
	default:
		return fmt.Errorf("unexpected type of NameSpacer: %T", o)
	}

	block, err := files.newConfigBlock(cfg, ct)
	if err != nil {
		return err
	}

	conf, err := getConfig(ct.parsedTemplate, ns.GetRelativeNumbers())
	if err != nil {
		return err
	}
	block.config = conf
	return nil
}

func generateConfigFiles(cfg *Config, nm *NetworkModel, outputPlatform string) error {
	for _, node := range nm.Nodes {
		if node.Virtual {
			continue
		}
		files := &ConfigFiles{mapper: map[string]*ConfigFile{}}

		for _, cls := range node.classLabels {
			nc, ok := cfg.nodeClassMap[cls]
			if !ok {
				return fmt.Errorf("undefined NodeClass name %v", cls)
			}
			for _, ct := range nc.ConfigTemplates {
				err := generateConfigBlock(cfg, &ct, files, node, outputPlatform)
				if err != nil {
					return err
				}
			}
		}

		for _, iface := range node.Interfaces {
			if iface.Virtual {
				continue
			}
			for _, cls := range iface.classLabels {
				ic, ok := cfg.interfaceClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined InterfaceClass name %v", cls)
				}
				for i := range ic.ConfigTemplates {
					ct := &ic.ConfigTemplates[i]
					err := generateConfigBlock(cfg, ct, files, iface, outputPlatform)
					if err != nil {
						return err
					}
				}
				for _, nc := range ic.NeighborClasses {
					for i := range nc.ConfigTemplates {
						ct := &nc.ConfigTemplates[i]
						neighbors, ok := iface.Neighbors[nc.IPSpace]
						if !ok {
							continue
							//return fmt.Errorf("neighbors not generated for %s", nc.IPSpace)
						}
						for _, neighbor := range neighbors {
							err := generateConfigBlock(cfg, ct, files, neighbor, outputPlatform)
							if err != nil {
								return err
							}
						}
					}
				}
			}

			if iface.Connection == nil {
				continue
			}
			for _, cls := range iface.Connection.classLabels {
				cc, ok := cfg.connectionClassMap[cls]
				if !ok {
					return fmt.Errorf("undefined ConnectionClass name %v", cls)
				}
				for i := range cc.ConfigTemplates {
					ct := &cc.ConfigTemplates[i]
					err := generateConfigBlock(cfg, ct, files, iface, outputPlatform)
					if err != nil {
						return err
					}
				}
				for _, nc := range cc.NeighborClasses {
					for i := range nc.ConfigTemplates {
						ct := &nc.ConfigTemplates[i]
						neighbors, ok := iface.Neighbors[nc.IPSpace]
						if !ok {
							continue
							// return fmt.Errorf("neighbors not generated for %s", nc.IPSpace)
						}
						for _, neighbor := range neighbors {
							err := generateConfigBlock(cfg, ct, files, neighbor, outputPlatform)
							if err != nil {
								return err
							}
						}
					}
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
