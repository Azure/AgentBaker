package runner

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/Azure/agentbaker/e2e/scenario"
)

type tagFilter struct {
	run  string
	skip string
}

func filterReason(name string, s *scenario.Scenario, filter tagFilter) (string, error) {
	tags := s.EffectiveTags()
	if filter.run != "" {
		matches, err := matchFilters(tags, filter.run, true)
		if err != nil {
			return "", fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if !matches {
			return fmt.Sprintf("filtered: scenario %q tags %+v do not match %q", name, tags, filter.run), nil
		}
	}
	if filter.skip != "" {
		matches, err := matchFilters(tags, filter.skip, false)
		if err != nil {
			return "", fmt.Errorf("could not match tags for %q: %w", name, err)
		}
		if matches {
			return fmt.Sprintf("filtered: scenario %q tags %+v match skip filter %q", name, tags, filter.skip), nil
		}
	}
	return "", nil
}

func partitionScenarios(scenarios []*scenario.Scenario, filter tagFilter) ([]*scenario.Scenario, []scenarioResult, error) {
	var runnable []*scenario.Scenario
	var filtered []scenarioResult
	for _, scenario := range scenarios {
		reason, err := filterReason(scenario.Name, scenario, filter)
		if err != nil {
			return nil, nil, err
		}
		if reason == "" {
			runnable = append(runnable, scenario)
			continue
		}
		filtered = append(filtered, scenarioResult{
			Name:     scenario.Name,
			Status:   statusSkipped,
			Attempts: []attemptResult{{Attempt: 1, Status: statusSkipped, Message: reason}},
		})
	}
	return runnable, filtered, nil
}

func matchFilters(t scenario.Tags, filters string, all bool) (bool, error) {
	if filters == "" {
		return true, nil
	}

	v := reflect.ValueOf(t)
	filterPairs := strings.Split(filters, ",")

	allNameFilters := true
	anyMatch := false
	allMatch := true

	for _, pair := range filterPairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			return false, fmt.Errorf("invalid filter format: %s", pair)
		}

		key := strings.TrimSpace(kv[0])
		value := strings.TrimSpace(kv[1])

		if !strings.EqualFold(key, "Name") {
			allNameFilters = false
		}

		// Case-insensitive field lookup
		field := reflect.Value{}
		for i := 0; i < v.NumField(); i++ {
			if strings.EqualFold(v.Type().Field(i).Name, key) {
				field = v.Field(i)
				break
			}
		}

		if !field.IsValid() {
			return false, fmt.Errorf("unknown filter key: %s", key)
		}

		var match bool
		switch field.Kind() {
		case reflect.String:
			fieldValue := field.String()
			if strings.EqualFold(key, "Name") {
				fieldValue = trimLegacyTestPrefix(fieldValue)
				value = trimLegacyTestPrefix(value)
			}
			match = strings.EqualFold(fieldValue, value)
		case reflect.Bool:
			boolValue, err := strconv.ParseBool(value)
			if err != nil {
				return false, fmt.Errorf("invalid boolean value for %s: %s", key, value)
			}
			match = field.Bool() == boolValue
		default:
			return false, fmt.Errorf("unsupported field type for %s", key)
		}

		if match {
			anyMatch = true
		} else {
			allMatch = false
		}
	}

	if allNameFilters {
		return anyMatch, nil
	}
	if all {
		return allMatch, nil
	}
	return anyMatch, nil
}

func trimLegacyTestPrefix(name string) string {
	if len(name) >= len("Test_") && strings.EqualFold(name[:len("Test_")], "Test_") {
		return name[len("Test_"):]
	}
	return name
}
