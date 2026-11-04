package runner

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/agentbaker/e2e/scenario"
)

const nbcSamplesPerOS = 2

func nbcSamplingSeed(buildID string, now time.Time) string {
	return buildID + "/" + now.UTC().Format(time.DateOnly)
}

func sampleNBCScenarios(scenarios []*scenario.Scenario, seed string) ([]*scenario.Scenario, error) {
	if config.Config.DisableScriptless || config.Config.TestPreProvision {
		return nil, &usageError{message: "NBC sampling cannot be combined with --disable-scriptless or --test-pre-provision"}
	}
	groups := map[string][]*scenario.Scenario{}
	names := map[string]bool{}
	for _, s := range scenarios {
		names[strings.ToLower(s.Name)] = true
		if group := s.NBCSamplingOS(); group != "" {
			groups[group] = append(groups[group], s)
		}
	}
	var sampled []*scenario.Scenario
	for _, group := range []string{"Ubuntu2204", "Ubuntu2404", "Ubuntu2604", "AzureLinuxV3"} {
		candidates := groups[group]
		if len(candidates) < nbcSamplesPerOS {
			return nil, &usageError{message: fmt.Sprintf("NBC sampling requires %d eligible %s scenarios after filtering; found %d", nbcSamplesPerOS, group, len(candidates))}
		}
		// Seeded hash ordering is independent of registration order and reproducible across retries.
		sort.Slice(candidates, func(i, j int) bool {
			a := sha256.Sum256([]byte(seed + "\x00" + candidates[i].Name))
			b := sha256.Sum256([]byte(seed + "\x00" + candidates[j].Name))
			if a == b {
				return candidates[i].Name < candidates[j].Name
			}
			return string(a[:]) < string(b[:])
		})
		for _, original := range candidates[:nbcSamplesPerOS] {
			copied := *original
			copied.Name += "/ScriptlessNBCCSECmd"
			if names[strings.ToLower(copied.Name)] {
				return nil, &usageError{message: fmt.Sprintf("NBC sample name collides with an existing scenario: %s", copied.Name)}
			}
			names[strings.ToLower(copied.Name)] = true
			copied.ForceScriptlessNBCCSECmd = true
			sampled = append(sampled, &copied)
		}
	}
	return sampled, nil
}
