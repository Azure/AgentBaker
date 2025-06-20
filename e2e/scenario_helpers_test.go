package e2e

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
)

func TestMain(m *testing.M) {
	log.Printf("using E2E environment configuration:\n%s\n", config.Config)
	// clean up logs from previous run
	if _, err := os.Stat("scenario-logs"); err == nil {
		_ = os.RemoveAll("scenario-logs")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := ensureResourceGroup(ctx)
	mustNoError(err)
	// _, err = config.Azure.CreateVMManagedIdentity(ctx)
	// mustNoError(err)
	m.Run()
}
