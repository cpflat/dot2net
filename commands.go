package main

import "github.com/urfave/cli/v2"

var commands = []*cli.Command{
	commandBuild,
	commandTinet,
	commandClab,
	commandParams,
	commandVisual,
	commandData,
}

var commandBuild = &cli.Command{
	Name:   "build",
	Usage:  "Build configuration files",
	Action: CmdBuild,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.StringFlag{
			Name:    "dir",
			Aliases: []string{"d"},
			Usage:   "Specify name of directory for per-device configuration.",
			Value:   "commands",
		},
		&cli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Profile CPU performance in generating internal config model and output to the specified file.",
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}

var commandTinet = &cli.Command{
	Name:   "tinet",
	Usage:  "Build TiNet specification file from DOT file specified in arguments",
	Action: CmdTinet,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Specify name of output specification file.",
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Profile CPU performance in generating internal config model and output to the specified file.",
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}

var commandClab = &cli.Command{
	Name:   "clab",
	Usage:  "Build Containerlab topology file from DOT file",
	Action: CmdClab,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Specify name of output topology file.",
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "profile",
			Aliases: []string{"p"},
			Usage:   "Profile CPU performance in generating internal config model and output to the specified file.",
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}

var commandParams = &cli.Command{
	Name:   "params",
	Usage:  "List available numbers for config templates",
	Action: CmdParams,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Show all numbers including relative ones.",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}

var commandVisual = &cli.Command{
	Name:   "visual",
	Usage:  "Visualize IP address assignment in DOT file",
	Action: CmdVisual,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
		&cli.StringFlag{
			Name:    "layer",
			Aliases: []string{"l"},
			Usage:   "Specify layer name to visualize. If not given, all information will be visualized.",
			Value:   "",
		},
		&cli.BoolFlag{
			Name:    "verbose",
			Aliases: []string{"v"},
			Usage:   "Verbose",
		},
	},
}

var commandData = &cli.Command{
	Name:   "data",
	Usage:  "Output parameter data in JSON format",
	Action: CmdData,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "Specify the Config file.",
			Value:   "config.yaml",
		},
	},
}
