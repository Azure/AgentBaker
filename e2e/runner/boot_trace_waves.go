package runner

import (
	"fmt"

	"github.com/Azure/agentbaker/e2e/scenario"
)

func bootTraceWaves(scenarios []*scenario.Scenario) ([][]*scenario.Scenario, error) {
	invalid := func(message string) ([][]*scenario.Scenario, error) {
		return nil, &usageError{message: "--boot-trace-waves " + message}
	}
	if len(scenarios) != 21 {
		return invalid("requires all 21 CA matrix scenarios")
	}
	waves := make([][]*scenario.Scenario, 3)
	variants := make([]map[string]bool, len(waves))
	for index := range variants {
		variants[index] = make(map[string]bool)
	}
	for _, s := range scenarios {
		if s == nil || s.BootTraceExperiment == nil || s.VHD == nil ||
			s.BootTraceBackend != "ftrace" || s.BootTraceExperiment.Variant == "" ||
			s.BootTraceWave < 1 || s.BootTraceWave > len(waves) {
			return invalid("accepts only CA matrix scenarios with a wave from 1 to 3")
		}
		index := s.BootTraceWave - 1
		key := s.VHD.Name + "/" + s.BootTraceExperiment.Variant
		if variants[index][key] {
			return invalid(fmt.Sprintf("has duplicate variant %s in wave %d", key, s.BootTraceWave))
		}
		variants[index][key] = true
		waves[index] = append(waves[index], s)
	}
	for index, wave := range waves {
		if len(wave) != 7 {
			return invalid(fmt.Sprintf("requires seven variants in wave %d", index+1))
		}
		for key := range variants[0] {
			if !variants[index][key] {
				return invalid(fmt.Sprintf("is missing variant %s in wave %d", key, index+1))
			}
		}
	}
	return waves, nil
}
