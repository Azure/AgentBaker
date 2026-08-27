package e2e

import (
	"errors"
	"fmt"
)

type tagFilter struct {
	run  string
	skip string
}

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

func filterScenario(name string, s *Scenario, filter tagFilter) error {
	tags := scenarioTags(s)
	if filter.run != "" {
		matches, err := tags.MatchesFilters(filter.run)
		if err != nil {
			return fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if !matches {
			return &skipError{message: fmt.Sprintf("filtered: scenario %q tags %+v do not match %q", name, tags, filter.run)}
		}
	}
	if filter.skip != "" {
		matches, err := tags.MatchesAnyFilter(filter.skip)
		if err != nil {
			return fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if matches {
			return &skipError{message: fmt.Sprintf("filtered: scenario %q tags %+v match skip filter %q", name, tags, filter.skip)}
		}
	}
	return nil
}

func tagSelectedEntries(entries []scenarioEntry, filter tagFilter) int {
	count := 0
	for _, entry := range entries {
		var skip *skipError
		if errors.As(filterScenario(entry.name, entry.scenario, filter), &skip) {
			continue
		}
		count++
	}
	return count
}
