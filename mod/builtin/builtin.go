package builtin

import (
	"github.com/cpflat/dot2net/pkg/types"
)

type BuiltinModule struct {
	*types.StandardModule
}

func NewModule() types.Module {
	return &BuiltinModule{
		StandardModule: types.NewStandardModule(),
	}
}

func (m *BuiltinModule) UpdateConfig(cfg *types.Config) error {
	// Default format for Format Phase (config block generation)
	formatStyle := &types.FormatStyle{
		Name:                types.DefaultFormatPhaseFormatName,
		FormatLineSeparator: "\n",
	}
	cfg.AddFormatStyle(formatStyle)

	// Default format for Merge Phase (config block assembly)
	formatStyle = &types.FormatStyle{
		Name:                types.DefaultMergePhaseFormatName,
		MergeBlockSeparator: "\n",
	}
	cfg.AddFormatStyle(formatStyle)

	return nil
}

func (m *BuiltinModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}

func (m *BuiltinModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}
