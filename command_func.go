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

func loadContext(c *cli.Context) (nd *model.NetworkDiagram, cfg *model.Config, name string, err error) {

	dotPath := c.Args().Get(0)

	nd, err = model.NetworkDiagramFromDotFile(dotPath)
	if err != nil {
		return nd, cfg, "", err
	}

	cfgPath := c.String("config")
	cfg, err = model.LoadConfig(cfgPath)
	if err != nil {
		return nd, cfg, "", err
	}

	name = c.String("output")
	return nd, cfg, name, err
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

func CmdCommand(c *cli.Context) error {
	nd, cfg, name, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd)
	if err != nil {
		return err
	}

	if name == "" {
		name = "commands"
	}
	f, err := os.Stat(name)
	if os.IsNotExist(err) {
		err = os.Mkdir(name, 0755)
		if err != nil {
			return err
		}
	} else if !f.IsDir() {
		return fmt.Errorf("file %v already exists", name)
	}
	for _, n := range nm.Nodes {
		filename := filepath.Join(name, n.Name+".conf")
		buf := strings.Join(n.Commands, "\n")
		err = os.WriteFile(filename, []byte(buf), 0644)
		if err != nil {
			return err
		}
	}
	return nil
}

func CmdTinet(c *cli.Context) error {
	nd, cfg, name, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd)
	if err != nil {
		return err
	}

	spec, err := tinet.GetTinetSpecification(cfg, nm)
	if err != nil {
		return err
	}

	err = outputString(name, spec)
	return err
}

func CmdClab(c *cli.Context) error {
	nd, cfg, name, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd)
	if err != nil {
		return err
	}

	spec, err := clab.GetClabTopologyConfig(cfg, nm)
	if err != nil {
		return err
	}

	err = outputString(name, spec)
	return err
}

func CmdNumber(c *cli.Context) error {
	nd, cfg, name, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd)
	if err != nil {
		return err
	}

	lines := []string{}
	for _, node := range nm.Nodes {
		keys := []string{}
		for num := range node.RelativeNumbers {
			keys = append(keys, num)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		for _, num := range keys {
			val := node.RelativeNumbers[num]
			fmt.Printf("%+v %+v=%+v\n", node.Name, num, val)
		}

		for _, iface := range node.Interfaces {
			keys := []string{}
			for num := range iface.RelativeNumbers {
				keys = append(keys, num)
			}
			sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

			for _, num := range keys {
				val := iface.RelativeNumbers[num]
				lines = append(lines, fmt.Sprintf("%+v.%+v %+v=%+v\n", node.Name, iface.Name, num, val))
			}
		}
	}

	buf := strings.Join(lines, "\n")
	err = outputString(name, []byte(buf))
	return err
}
