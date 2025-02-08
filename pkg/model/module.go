package model

import (
	"fmt"

	containerlab "github.com/cpflat/dot2net/mod/containerlab"
	tinet "github.com/cpflat/dot2net/mod/tinet"
	"github.com/cpflat/dot2net/pkg/types"
)

func LoadModules(cfg *types.Config) error {
	var m types.Module
	modules := []types.Module{}
	for _, name := range cfg.Modules {

		// load modules based on given names in Config.Modules
		switch name {
		case "tinet":
			m = tinet.NewModule()
		case "containerlab":
			m = containerlab.NewModule()
		default:
			return fmt.Errorf("unknown module: %s", name)
		}

		err := m.UpdateConfig(cfg)
		if err != nil {
			return err
		}
		modules = append(modules, m)
	}
	cfg.LoadedModules = modules

	return nil
}

func getModuleNodeClassLabels(cfg *types.Config) []string {
	ret := []string{}
	for _, module := range cfg.LoadedModules {
		ret = append(ret, module.GetModuleNodeClassLabels()...)
	}
	return ret
}

func getModuleInterfaceClassLabels(cfg *types.Config) []string {
	ret := []string{}
	for _, module := range cfg.LoadedModules {
		ret = append(ret, module.GetModuleInterfaceClassLabels()...)
	}
	return ret
}

func getModuleConnectionClassLabels(cfg *types.Config) []string {
	ret := []string{}
	for _, module := range cfg.LoadedModules {
		ret = append(ret, module.GetModuleConnectionClassLabels()...)
	}
	return ret
}

func generateModuleParameters(cfg *types.Config, nm *types.NetworkModel) error {
	for _, module := range cfg.LoadedModules {
		err := module.GenerateParameters(cfg, nm)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	for _, module := range cfg.LoadedModules {
		err := module.CheckModuleRequirements(cfg, nm)
		if err != nil {
			return fmt.Errorf("module %T: %w", module, err)
		}
	}
	return nil
}
