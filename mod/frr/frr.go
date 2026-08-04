package frr

import (
	"github.com/cpflat/dot2net/pkg/types"
)

const FRRVtyshCLIFormatName = "FRRVtyshCLI"

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
	return nil
}
