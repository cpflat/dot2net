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
	fileFormat := &types.FileFormat{
		Name:          FRRVtyshCLIFormatName,
		LineSeparator: "\" -c \"",
		BlockPrefix:   "vtysh -c \"conf t\" -c \"",
		BlockSuffix:   "\"",
	}
	cfg.AddFileFormat(fileFormat)
	return nil
}

func (m FRRModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}

func (m FRRModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}
