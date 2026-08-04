package model

import (
	"fmt"

	containerlab "github.com/cpflat/dot2net/mod/containerlab"
	frr "github.com/cpflat/dot2net/mod/frr"
	tinet "github.com/cpflat/dot2net/mod/tinet"
	"github.com/cpflat/dot2net/pkg/types"
)

func LoadModules(cfg *types.Config) error {
	var m types.Module
	modules := []types.Module{}
	for _, name := range cfg.Modules {

		// load modules based on given names in Config.Modules
		switch name {
		case "frr":
			m = frr.NewModule()
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
	for _, mod := range cfg.LoadedModules {
		ret = append(ret, mod.GetModuleNodeClassLabels()...)
	}
	return ret
}

func getModuleInterfaceClassLabels(cfg *types.Config) []string {
	ret := []string{}
	for _, mod := range cfg.LoadedModules {
		ret = append(ret, mod.GetModuleInterfaceClassLabels()...)
	}
	return ret
}

func getModuleConnectionClassLabels(cfg *types.Config) []string {
	ret := []string{}
	for _, mod := range cfg.LoadedModules {
		ret = append(ret, mod.GetModuleConnectionClassLabels()...)
	}
	return ret
}

// generateModuleParameters runs the ParameterProvider hook of every module that
// implements it. Modules without it are skipped rather than forced to carry an
// empty method.
func generateModuleParameters(cfg *types.Config, nm *types.NetworkModel) error {
	for _, mod := range cfg.LoadedModules {
		provider, ok := mod.(types.ParameterProvider)
		if !ok {
			continue
		}
		if err := provider.GenerateParameters(cfg, nm); err != nil {
			return fmt.Errorf("module %T: %w", mod, err)
		}
	}
	return nil
}

// checkModuleRequirements runs the RequirementChecker hook of every module that
// implements it.
func checkModuleRequirements(cfg *types.Config, nm *types.NetworkModel) error {
	for _, mod := range cfg.LoadedModules {
		checker, ok := mod.(types.RequirementChecker)
		if !ok {
			continue
		}
		if err := checker.CheckModuleRequirements(cfg, nm); err != nil {
			return fmt.Errorf("module %T: %w", mod, err)
		}
	}
	return nil
}
