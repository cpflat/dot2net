package frr

import (
	"github.com/cpflat/dot2net/pkg/types"
)

const FRRVtyshCLIFormatName = "FRRVtyshCLI"

type FRRModule struct {
	*types.StandardModule
}

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

func (m FRRModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}

func (m FRRModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}
