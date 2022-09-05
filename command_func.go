package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/cpflat/dot2tinet/pkg/model"
	"github.com/cpflat/dot2tinet/pkg/output"
	"github.com/urfave/cli/v2"
)

func loadContext(c *cli.Context) (nd *model.NetworkDiagram, cfg *model.Config, err error) {

	dotPath := c.Args().Get(0)

	nd, err = model.NetworkDiagramFromDotFile(dotPath)
	if err != nil {
		return nd, cfg, err
	}

	cfgPath := c.String("config")
	cfg, err = model.LoadConfig(cfgPath)
	if err != nil {
		return nd, cfg, err
	}

	return nd, cfg, err
}

func CmdBuild(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd)
	if err != nil {
		return err
	}

	spec, err := output.GetSpecification(cfg, nm)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, spec)

	return nil
}

func CmdNumber(c *cli.Context) error {
	nd, cfg, err := loadContext(c)
	if err != nil {
		return err
	}

	nm, err := model.BuildNetworkModel(cfg, nd)
	if err != nil {
		return err
	}

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
				fmt.Printf("%+v.%+v %+v=%+v\n", node.Name, iface.Name, num, val)
			}
		}
	}
	return nil
}
