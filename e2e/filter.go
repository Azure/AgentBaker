package e2e

import (
	"errors"
	"fmt"

	"github.com/Azure/agentbaker/e2e/config"
)

func scenarioTags(s *Scenario) Tags {
	tags := s.Tags
	tags.Name = s.Name
	tags.VHDCaching = s.VHDCaching
	if s.VHD != nil {
		tags.OS = string(s.VHD.OS)
		tags.Arch = s.VHD.Arch
		tags.ImageName = s.VHD.Name
	}
	return tags
}

func filterScenario(name string, s *Scenario) error {
	tags := scenarioTags(s)
	if config.Config.TagsToRun != "" {
		matches, err := tags.MatchesFilters(config.Config.TagsToRun)
		if err != nil {
			return fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if !matches {
			return &skipError{message: fmt.Sprintf("filtered: scenario %q tags %+v do not match %q", name, tags, config.Config.TagsToRun)}
		}
	}
	if config.Config.TagsToSkip != "" {
		matches, err := tags.MatchesAnyFilter(config.Config.TagsToSkip)
		if err != nil {
			return fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if matches {
			return &skipError{message: fmt.Sprintf("filtered: scenario %q tags %+v match skip filter %q", name, tags, config.Config.TagsToSkip)}
		}
	}
	return nil
}

func tagSelectedEntries(entries []scenarioEntry) int {
	count := 0
	for _, entry := range entries {
		var skip *skipError
		if errors.As(filterScenario(entry.name, entry.scenario), &skip) {
			continue
		}
		count++
	}
	return count
}
