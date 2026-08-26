package config

import (
	"testing"

	"github.com/urfave/cli/v3"
)

func TestFlagsRepeatedParseDoesNotInheritPreviousRun(t *testing.T) {
	original := Config
	defer func() { Config = original }()

	trueDefault := DefaultConfiguration().DefaultLocation

	Config = DefaultConfiguration()
	cmd1 := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	if err := cmd1.Run(t.Context(), []string{"e2e-test-config", "--location", "custom-location-xyz"}); err != nil {
		t.Fatalf("first parse failed: %v", err)
	}
	if Config.DefaultLocation != "custom-location-xyz" {
		t.Fatalf("first parse did not set DefaultLocation, got %q", Config.DefaultLocation)
	}

	cmd2 := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	if err := cmd2.Run(t.Context(), []string{"e2e-test-config"}); err != nil {
		t.Fatalf("second parse failed: %v", err)
	}
	if Config.DefaultLocation != trueDefault {
		t.Fatalf("second parse leaked the first run's value: got %q, want default %q", Config.DefaultLocation, trueDefault)
	}
}

func TestFlagsEnvironmentSourceStillWorks(t *testing.T) {
	original := Config
	defer func() { Config = original }()

	t.Setenv("E2E_PARALLEL", "7")
	Config = DefaultConfiguration()
	cmd := &cli.Command{Name: "e2e-test-config", Flags: Flags()}
	if err := cmd.Run(t.Context(), []string{"e2e-test-config"}); err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if Config.Parallel != 7 {
		t.Fatalf("E2E_PARALLEL was not applied: got %d, want 7", Config.Parallel)
	}
}
