package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

const (
	dir         = "dot2net"
	description = "Generate config files for large-scale emulation networks from DOT files"
)

var (
	Version = "0.6.1"
)

func main() {
	if err := newApp().Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = dir
	app.Version = Version
	app.Usage = description
	app.Authors = []*cli.Author{
		{
			Name: "Satoru Kobayashi",
			// Email: "sat@3at.work",
			Email: "sat@okayama-u.ac.jp",
		},
	}
	app.Commands = commands

	return app
}
