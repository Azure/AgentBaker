package e2e

import (
	"log"
	"os"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
)

func TestMain(m *testing.M) {
	log.Printf("using E2E environment configuration:\n%s\n", config.Config)
	// clean up logs from previous run
	if _, err := os.Stat("scenario-logs"); err == nil {
		_ = os.RemoveAll("scenario-logs")
	}
	m.Run()
}
