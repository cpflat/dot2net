package model

import (
	"fmt"

	assert "github.com/cpflat/dot2net/mod/assert"
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
		case "assert":
			m = assert.NewModule()
		default:
			return fmt.Errorf("unknown module: %s", name)
		}

		// Through the config, so that the classes a module registers are marked
		// as module-provided (they must keep the weakest tier).
		if err := cfg.LoadModuleConfig(m); err != nil {
			return err
		}
		modules = append(modules, m)
	}
	cfg.LoadedModules = modules

	return nil
}

// classifyModuleObjects runs the ObjectClassifier hook of every module that
// implements it. It sits between buildSkeleton and checkClasses so that labels
// added here are still resolved into classes.
func classifyModuleObjects(cfg *types.Config, nm *types.NetworkModel) error {
	for _, mod := range cfg.LoadedModules {
		classifier, ok := mod.(types.ObjectClassifier)
		if !ok {
			continue
		}
		if err := classifier.ClassifyObjects(cfg, nm); err != nil {
			return fmt.Errorf("module %T: %w", mod, err)
		}
	}
	return nil
}

func getModuleGroupClassLabels(cfg *types.Config) []string {
	ret := []string{}
	for _, mod := range cfg.LoadedModules {
		ret = append(ret, mod.GetModuleGroupClassLabels()...)
	}
	return ret
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
