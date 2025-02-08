package model

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/cpflat/dot2net/pkg/types"
)

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
		fmt.Fprintf(os.Stderr, "%s", nm.StringAllObjectClasses(cfg))
	}
	// files := newConfigFiles()

	// post order traversal with depth first search
	// so that a parent node is processed after all its child nodes
	var traverse func(types.NameSpacer) error
	traverse = func(parent types.NameSpacer) error {
		// list up child classtype candidates
		classes, err := parent.ChildClasses()
		if err != nil {
			return err
		}
		for _, cls := range classes {
			// list up child objects of the classtype
			objs, err := parent.Childs(cls)
			if err != nil {
				return err
			}
			// traverse child objects
			for _, obj := range objs {
				err := traverse(obj)
				if err != nil {
					return fmt.Errorf("error on processing %s: %w", obj.StringForMessage(), err)
				}
			}

			// generate config blocks for the child objects
			err = generateConfigForObjects(cfg, objs, parent, verbose)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err := traverse(nm)
	if err != nil {
		return err
	}
	err = generateConfigForObjects(cfg, []types.NameSpacer{nm}, nil, verbose)
	if err != nil {
		return err
	}

	return nil
}

func generateConfigForObjects(cfg *types.Config, objs []types.NameSpacer, parent types.NameSpacer, verbose bool) error {

	// named config can be used for output config, so explicitly generate them
	configForNamespace := map[*types.ConfigTemplate][]string{}
	for _, ns := range objs {
		configTemplates := ns.GetConfigTemplates(cfg)
		named, _ := classifyConfigTemplates(configTemplates)
		if verbose {
			fmt.Fprintf(os.Stderr, "processing %s (%d templates)\n", ns.StringForMessage(), len(configTemplates))
		}
		for _, ct := range named {

			if verbose {
				fmt.Fprintf(
					os.Stderr, "generating config blocks for %s with %s\n",
					ns.StringForMessage(), ct.String(),
				)
			}

			conf, err := generateConfigBlock(ct, ns)
			if err != nil {
				return err
			}
			// ignore empty config
			if conf == "" {
				continue
			}

			if ct.Name != "" {
				// add to self's namespace if ct.Name is specified
				err := addSelfConfigToNameSpace(cfg, ns, conf, ct, verbose)
				if err != nil {
					return err
				}
				// add to parent's namespace if ct.Name is specified
				if parent == nil {
					fmt.Fprintln(os.Stderr, "warning: config.name for network class is meaningless, just ignored")
				} else {
					configForNamespace[ct] = append(configForNamespace[ct], conf)
				}
			}
		}
	}
	for ct, confs := range configForNamespace {
		err := addChildsConfigToNameSpace(cfg, parent, objs, confs, ct, verbose)
		if err != nil {
			return err
		}
	}

	for _, ns := range objs {
		configTemplates := ns.GetConfigTemplates(cfg)
		_, output := classifyConfigTemplates(configTemplates)
		for _, ct := range output {

			if verbose {
				fmt.Fprintf(
					os.Stderr, "generating config blocks for %s with %s\n",
					ns.StringForMessage(), ct.String(),
				)
			}

			conf, err := generateConfigBlock(ct, ns)
			if err != nil {
				return err
			}
			// ignore empty config
			if conf == "" {
				continue
			}
			// output file if ct.File is specified
			if ct.File != "" {
				err = outputConfigFile(cfg, ns, conf, ct, verbose)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func generateConfigBlock(ct *types.ConfigTemplate, ns types.NameSpacer) (string, error) {
	switch o := ns.(type) {
	case *types.NetworkModel:
		// pass
	case *types.Node:
		// pass
	case *types.Interface:
		// skip if node class does not match
		if !(ct.NodeClassCheck(o.Node)) {
			return "", nil
		}
	case *types.Neighbor:
		// skip if self node class does not match
		if !(ct.NodeClassCheck(o.Self.Node)) {
			return "", nil
		}
		// skip if neighbor node class does not match
		if !(ct.NodeClassCheck(o.Neighbor.Node)) {
			return "", nil
		}
	case *types.Member:
		switch t := o.Referrer.(type) {
		case *types.Node:
			// pass
		case *types.Interface:
			if !(ct.NodeClassCheck(t.Node)) {
				return "", nil
			}
		default:
			return "", fmt.Errorf("panic: unexpected type of Member Referer: %T", t)
		}
	default:
		return "", fmt.Errorf("unexpected type of NameSpacer: %T", o)
	}

	conf, err := getConfig(ct.ParsedTemplate, ns.GetRelativeParams())
	if err != nil {
		return "", fmt.Errorf("templating failure for %s, %w", ns.StringForMessage(), err)
	}
	return conf, nil
}

func addSelfConfigToNameSpace(cfg *types.Config, ns types.NameSpacer, conf string, ct *types.ConfigTemplate, verbose bool) error {
	// format config block in the same way with merging config blocks
	conf, err := formatSingleConfigBlock(cfg, conf, ct)
	if err != nil {
		return fmt.Errorf("error on formatting config block of %s, %w", ns.StringForMessage(), err)
	}

	// format lines
	conf, err = formatConfigLines(cfg, conf, []string{ct.Format})
	if err != nil {
		return fmt.Errorf("error on formatting config lines of %s, %w", ns.StringForMessage(), err)
	}

	relativeName := SelfConfigHeader + ct.Name
	if ns.HasRelativeParam(relativeName) {
		value, _ := ns.GetParamValue(relativeName)
		return fmt.Errorf(
			"parameter name %s of object %s duplicated (existing parameter: %s)",
			relativeName, ns.StringForMessage(), value,
		)
	}
	ns.SetRelativeParam(relativeName, conf)
	if verbose {
		fmt.Fprintf(os.Stderr, " set relative param to %s: %s\n", ns.StringForMessage(), relativeName)
		// fmt.Fprintf(os.Stderr, "%s\n", conf)
	}
	return nil
}

func addChildsConfigToNameSpace(cfg *types.Config, parent types.NameSpacer, childs []types.NameSpacer,
	confs []string, ct *types.ConfigTemplate, verbose bool) error {

	if len(childs) == 0 {
		return nil
	}

	var relativeName string
	top := childs[0]
	switch obj := childs[0].(type) {
	case *types.Node:
		relativeName = ChildNodesConfigHeader + ct.Name
	case *types.Interface:
		relativeName = ChildInterfacesConfigHeader + ct.Name
	case *types.Group:
		relativeName = ChildGroupsConfigHeader + ct.Name
	case *types.Neighbor:
		relativeName = ChildNeighborsConfigHeader + obj.Layer + NumberSeparator + ct.Name
	case *types.Member:
		relativeName = ChildMembersConfigHeader + obj.ClassType + NumberSeparator + obj.ClassName + NumberSeparator + ct.Name
	default:
		return fmt.Errorf("unexpected type of NameSpacer as child node: %T", top)
	}

	for _, child := range childs {
		if reflect.TypeOf(top) != reflect.TypeOf(child) {
			return fmt.Errorf("different types of child nodes are mixed")
		}
	}

	// merge config blocks of childs
	result, err := mergeConfigBlocks(cfg, confs, ct)
	if err != nil {
		return fmt.Errorf("error on merging config blocks of %v, %w", childs, err)
	}

	// format lines
	result, err = formatConfigLines(cfg, result, []string{ct.Format})
	if err != nil {
		return fmt.Errorf("error on formatting config lines of %v, %w", childs, err)
	}

	// add merged config blocks to parent namespace
	if parent.HasRelativeParam(relativeName) {
		value, _ := parent.GetParamValue(relativeName)
		return fmt.Errorf(
			"parameter name %s of object %s duplicated (existing parameter: %s)",
			relativeName, parent.StringForMessage(), value,
		)
	}
	parent.SetRelativeParam(relativeName, result)
	if verbose {
		fmt.Fprintf(os.Stderr, " set relative param to %s: %s\n", parent.StringForMessage(), relativeName)
		// fmt.Fprintf(os.Stderr, "%s\n", result)
	}
	return nil
}

func outputConfigFile(cfg *types.Config, ns types.NameSpacer, conf string, ct *types.ConfigTemplate, verbose bool) error {
	filedef, ok := cfg.FileDefinitionByName(ct.File)
	if !ok {
		return fmt.Errorf("undefined file format %s", ct.File)
	}

	// format lines
	conf, err := formatConfigLines(cfg, conf, filedef.Formats)
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
			return fmt.Errorf("creating directory %s fails because a file already exists", dirname)
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

func classifyConfigTemplates(cts []*types.ConfigTemplate) ([]*types.ConfigTemplate, []*types.ConfigTemplate) {
	named := []*types.ConfigTemplate{}
	output := []*types.ConfigTemplate{}
	for _, ct := range cts {
		if ct.Name != "" {
			named = append(named, ct)
		}
		if ct.File != "" {
			output = append(output, ct)
		}
	}
	return named, output
}

func mergeConfigBlocks(cfg *types.Config, blocks []string, ct *types.ConfigTemplate) (string, error) {
	if ct.Format == "" {
		return strings.Join(blocks, "\n"), nil
	} else {
		filefmt, ok := cfg.FileFormatByName(ct.Format)
		if !ok {
			return "", fmt.Errorf("undefined file format %s", ct.Format)
		}
		return filefmt.BlockPrefix + strings.Join(blocks, filefmt.BlockSeparator) + filefmt.BlockSuffix, nil
	}
}

func formatSingleConfigBlock(cfg *types.Config, block string, ct *types.ConfigTemplate) (string, error) {
	if ct.Format == "" {
		return block, nil
	} else {
		filefmt, ok := cfg.FileFormatByName(ct.Format)
		if !ok {
			return "", fmt.Errorf("undefined file format %s", ct.Format)
		}
		return filefmt.BlockPrefix + block + filefmt.BlockSuffix, nil
	}
}

func formatConfigLines(cfg *types.Config, conf string, formats []string) (string, error) {
	var separator string
	// format lines
	for _, format := range formats {
		if format == "" {
			continue
		}
		segmentedConf := strings.Split(conf, "\n")
		filefmt, ok := cfg.FileFormatByName(format)
		if !ok {
			return "", fmt.Errorf("undefined file format %s", format)
		}
		newConf := []string{}
		for _, line := range segmentedConf {
			newConf = append(newConf, filefmt.LinePrefix+line+filefmt.LineSuffix)
		}

		if filefmt.LineSeparator == "" {
			separator = "\n"
		} else if filefmt.LineSeparator == "#NONE#" {
			separator = ""
		} else {
			separator = filefmt.LineSeparator
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
