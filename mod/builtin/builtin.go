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
	return nil
}

func (m *BuiltinModule) GenerateParameters(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}

func (m *BuiltinModule) CheckModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	return nil
}
