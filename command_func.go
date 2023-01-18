package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cpflat/dot2tinet/pkg/clab"
	"github.com/cpflat/dot2tinet/pkg/model"
	"github.com/cpflat/dot2tinet/pkg/tinet"
	"github.com/urfave/cli/v2"
)

func loadContext(c *cli.Context) (nd *model.NetworkDiagram, cfg *model.Config, err error) {

	dotPaths := c.Args().Slice()

	nd, err = model.NetworkDiagramFromDotFile(dotPaths[0])
	if err != nil {
		return nil, nil, err
	}
	for _, dotPath := range dotPaths[1:] {
		newnd, err := model.NetworkDiagramFromDotFile(dotPath)
		if err != nil {
			return nil, nil, err
		}
		nd.MergeDiagram(newnd)
	}

	cfgPath := c.String("config")
	cfg, err = model.LoadConfig(cfgPath)
	if err != nil {
		return nd, cfg, err
	}

	return nd, cfg, err
}

func outputString(name string, buffer []byte) error {
	if name == "" {
		fmt.Fprintln(os.Stdout, string(buffer))
	} else if _, err := os.Stat(name); err == nil {
		return fmt.Errorf("file %v already exists", name)
	} else {
		err = os.WriteFile(name, buffer, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func outputFiles(buffers map[string][]byte, dirname string) error {
	f, err := os.Stat(dirname)
	if os.IsNotExist(err) {
		err = os.Mkdir(dirname, 0755)
		if err != nil {
			return err
		}
	} else if !f.IsDir() {
		return fmt.Errorf("file %v already exists", dirname)
	}
	for filename, buffer := range buffers {
		path := filepath.Join(dirname, filename)
		err = os.WriteFile(path, buffer, 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func generateScriptBuffers(nm *model.NetworkModel, cfgmap map[string]string) map[string][]byte {
	buffers := make(map[string][]byte, len(nm.Nodes))
	for _, n := range nm.Nodes {
		filename := cfgmap[n.Name]
		buffer := strings.Join(n.Commands, "\n")
		buffers[filename] = []byte(buffer)
	}
	return buffers
}

func CmdCommand(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd, model.OutputAsis)
	if err != nil {
		return err
	}
	dir := c.String("dir")

	cfgmap := map[string]string{}
	for _, n := range nm.Nodes {
		filename := n.Name
		cfgmap[n.Name] = filename
	}
	buffers := generateScriptBuffers(nm, cfgmap)
	return outputFiles(buffers, dir)
}

func CmdTinet(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	name := c.String("output")
	dir := c.String("dir")

	nm, err := model.BuildNetworkModel(cfg, nd, model.OutputTinet)
	if err != nil {
		return err
	}

	if dir == "" {
		// generate clab topology file (incluing configuration)
		spec, err := tinet.GetTinetSpecificationConfig(cfg, nm)
		if err != nil {
			return err
		}
		// output clab topology file
		err = outputString(name, spec)
		return err
	} else {
		// generate script files in dir
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
		cfgmap := tinet.GetScriptPaths(cfg, nm)
		buffers := generateScriptBuffers(nm, cfgmap)

		// generate tinet specification file without configuration commands
		spec, err := tinet.GetTinetSpecification(cfg, nm, cfgmap, dir)
		if err != nil {
			return err
		}

		// output script files
		err = outputFiles(buffers, dir)
		if err != nil {
			return err
		}

		// output clab topology file
		err = outputString(name, spec)
		return err
	}
}

func CmdClab(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	name := c.String("output")
	dir := c.String("dir")

	nm, err := model.BuildNetworkModel(cfg, nd, model.OutputClab)
	if err != nil {
		return err
	}

	if dir == "" {
		// generate clab topology file (incluing configuration)
		spec, err := clab.GetClabTopologyConfig(cfg, nm)
		if err != nil {
			return err
		}
		// output clab topology file
		err = outputString(name, spec)
		return err
	} else {
		// generate script files in dir
		dir, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
		cfgmap := clab.GetScriptPaths(cfg, nm)
		buffers := generateScriptBuffers(nm, cfgmap)

		// generate clab topology file
		spec, err := clab.GetClabTopology(cfg, nm, cfgmap, dir)
		if err != nil {
			return err
		}

		// output script files
		err = outputFiles(buffers, dir)
		if err != nil {
			return err
		}

		// output clab topology file
		err = outputString(name, spec)
		return err
	}
}

func CmdNumber(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}
	name := c.String("output")
	flagall := c.Bool("all")

	nm, err := model.BuildNetworkModel(cfg, nd, model.OutputAsis)
	if err != nil {
		return err
	}

	var nodeNumbers map[string]string
	var ifaceNumbers map[string]string
	lines := []string{}
	for _, node := range nm.Nodes {
		if flagall {
			nodeNumbers = node.RelativeNumbers
		} else {
			nodeNumbers = node.Numbers
		}

		keys := []string{}
		for num := range nodeNumbers {
			keys = append(keys, num)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, num := range keys {
			val := nodeNumbers[num]
			lines = append(lines, fmt.Sprintf("%+v {{ .%+v }} = %+v", node.Name, num, val))
		}

		for _, iface := range node.Interfaces {
			if flagall {
				ifaceNumbers = iface.RelativeNumbers
			} else {
				ifaceNumbers = iface.Numbers
			}

			keys := []string{}
			for num := range ifaceNumbers {
				keys = append(keys, num)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
			for _, num := range keys {
				val := ifaceNumbers[num]
				lines = append(lines, fmt.Sprintf("%+v.%+v {{ .%+v }} = %+v", node.Name, iface.Name, num, val))
			}
		}
	}

	buf := strings.Join(lines, "\n")
	err = outputString(name, []byte(buf))
	return err
}
