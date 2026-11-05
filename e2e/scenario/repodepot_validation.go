package scenario

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"os"
)

//go:embed repodepot_fixture.py
var repoDepotFixture string

// ValidateRepoDepotConfiguration exercises real prerequisite setup with an isolated HTTPS repository fixture.
func ValidateRepoDepotConfiguration(ctx context.Context, s *Scenario) error {
	initializer, err := os.ReadFile(repoPath("parts/linux/cloud-init/artifacts/init-aks-cloud.sh"))
	if err != nil {
		return fmt.Errorf("read cloud prerequisite script: %w", err)
	}
	command := fmt.Sprintf("printf '%%s' '%s' | base64 -d | sudo python3 - '%s'",
		base64.StdEncoding.EncodeToString([]byte(repoDepotFixture)),
		base64.StdEncoding.EncodeToString(initializer))
	_, err = execScriptOnVMForScenarioValidateExitCode(ctx, s, command, 0,
		"expected RepoDepot prerequisite configuration to match the HTTPS fixture")
	return err
}
