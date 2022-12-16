package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

const (
	name        = "dot2tinet"
	description = "Generate tinet specification from DOT files"
)

var (
	Version = "0.0.2"
)

func main() {
	if err := newApp().Run(os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func newApp() *cli.App {
	app := cli.NewApp()
	app.Name = name
	app.Version = Version
	app.Usage = description
	app.Authors = []*cli.Author{
		{
			Name:  "Satoru Kobayashi",
			Email: "sat@3at.work",
		},
	}
	app.Commands = commands

	return app
}
