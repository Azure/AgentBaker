package e2e

import (
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

func filterReason(name string, s *Scenario, filter tagFilter) (string, error) {
	tags := scenarioTags(s)
	if filter.run != "" {
		matches, err := tags.MatchesFilters(filter.run)
		if err != nil {
			return "", fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if !matches {
			return fmt.Sprintf("filtered: scenario %q tags %+v do not match %q", name, tags, filter.run), nil
		}
	}
	if filter.skip != "" {
		matches, err := tags.MatchesAnyFilter(filter.skip)
		if err != nil {
			return "", fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if matches {
			return fmt.Sprintf("filtered: scenario %q tags %+v match skip filter %q", name, tags, filter.skip), nil
		}
	}
	return "", nil
}

func partitionEntries(entries []scenarioEntry, filter tagFilter) ([]scenarioEntry, []scenarioResult, error) {
	var runnable []scenarioEntry
	var filtered []scenarioResult
	for _, entry := range entries {
		reason, err := filterReason(entry.name, entry.scenario, filter)
		if err != nil {
			return nil, nil, err
		}
		if reason == "" {
			runnable = append(runnable, entry)
			continue
		}
		filtered = append(filtered, scenarioResult{
			Name:     entry.name,
			Status:   statusSkipped,
			Attempts: []attemptResult{{Attempt: 1, Status: statusSkipped, Message: reason}},
		})
	}
	return runnable, filtered, nil
}
