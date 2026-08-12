package frr

import (
	"embed"

	"github.com/cpflat/dot2net/pkg/types"
)

//go:embed templates/*
var templates embed.FS

const FRRVtyshCLIFormatName = "FRRVtyshCLI"

// LogFileClassName is a class a node class can pull in with use: to have FRR
// write to a log file.
//
// A scenario writes use: [frrLogFile] and nothing else: the class adds what it
// has to do to the node's startup, ahead of whatever the scenario asked for
// there.
//
// It exists because FRR cannot make that file itself: its daemons run as the
// frr user and /var/log belongs to root, so a log file named in a configuration
// read at boot is one FRR reports it cannot open. The file has to be made, given
// to frr, and named to a running FRR - all of which the container can do to
// itself, which is why this is a startup block rather than a generated file.
//
// What is lost by doing it after boot is what FRR logged before this runs; from
// then on the log is complete.
const LogFileClassName = "frrLogFile"

// LogPathParamName is where the log is written. A node class of the scenario's
// own can set it to something else: a user's value outranks a module's.
const LogPathParamName = "frr_log_path"

const DefaultLogPath = "/var/log/frr.log"

// LogLevelParamName is how much FRR writes there. Named rather than left to
// FRR's own default, which is debugging and floods a lab's log.
const LogLevelParamName = "frr_log_level"

const DefaultLogLevel = "informational"

type FRRModule struct {
	*types.StandardModule
}

// Capabilities provided by this module. FRR only registers a FormatStyle, so it
// implements nothing beyond the required Module interface.
var _ types.Module = (*FRRModule)(nil)

func NewModule() types.Module {
	return &FRRModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *FRRModule) UpdateConfig(cfg *types.Config) error {
	formatStyle := &types.FormatStyle{
		Name:                FRRVtyshCLIFormatName,
		FormatLineSeparator: "\" -c \"",
		FormatBlockPrefix:   "vtysh -c \"conf t\" -c \"",
		FormatBlockSuffix:   "\"",
	}
	cfg.AddFormatStyle(formatStyle)

	bytes, err := templates.ReadFile("templates/setup.node_frr_log_setup")
	if err != nil {
		return err
	}
	cfg.AddNodeClass(&types.NodeClass{
		Name: LogFileClassName,
		// The log is the module's to make, so it is the module's to bring back:
		// a scenario that never named the file should not have to name it here.
		Collect: []string{"{{ ." + LogPathParamName + " }}"},
		Values: map[string]string{
			LogPathParamName:  DefaultLogPath,
			LogLevelParamName: DefaultLogLevel,
		},
		ConfigTemplates: []*types.ConfigTemplate{
			{Name: "startup", Template: []string{string(bytes)}},
		},
	})
	return nil
}
