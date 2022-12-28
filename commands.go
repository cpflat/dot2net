package main

import "github.com/urfave/cli/v2"

var commands = []*cli.Command{
	commandCommand,
	commandTinet,
	commandClab,
	commandNumber,
}

var commandCommand = &cli.Command{
	Name:   "command",
	Usage:  "Build per-device configurations (commands) to a directory",
	Action: CmdCommand,
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
			Usage:   "Specify name of output file or directory.",
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
			Usage:   "Specify name of output file or directory.",
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
			Usage:   "Specify name of output file or directory.",
			Value:   "",
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
