package main

import "github.com/urfave/cli/v2"

var commands = []*cli.Command{
	commandBuild,
	commandNumber,
}

var commandBuild = &cli.Command{
	Name:   "build",
	Usage:  "Build TiNet specification file from DOT file specified in arguments",
	Action: CmdBuild,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}

var commandNumber = &cli.Command{
	Name:   "number",
	Usage:  "List available numbers for config templates",
	Action: CmdNumber,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}
