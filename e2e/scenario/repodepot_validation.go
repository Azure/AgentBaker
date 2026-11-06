package scenario

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"

	"github.com/Azure/agentbaker/e2e/logging"
)

//go:embed repodepot_fixture.py
var repoDepotFixture string

// ValidateRepoDepotConfiguration exercises real prerequisite setup with an isolated HTTPS repository fixture.
func ValidateRepoDepotConfiguration(ctx context.Context, s *Scenario) error {
	if laneResolvedMainBuiltImage() {
		logging.Logf(ctx, "SKIP: RepoDepot prerequisite fixture requires a newly built VHD; this lane selects default main-built images")
		return nil
	}
	command := fmt.Sprintf("printf '%%s' '%s' | base64 -d | sudo python3 -",
		base64.StdEncoding.EncodeToString([]byte(repoDepotFixture)))
	_, err := execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0,
		"expected RepoDepot prerequisite configuration to match the HTTPS fixture")
	return err
}
