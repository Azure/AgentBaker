package e2e

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func TestRecordCheckCapturesFailureMessage(t *testing.T) {
	s := &Scenario{}
	s.recordCheck("TotalCSEDuration", 3*time.Second, nil)
	s.recordCheck("Task_cse_start", 5*time.Second, errors.New("too slow"))

	if len(s.checks) != 2 {
		t.Fatalf("expected 2 recorded checks, got %d", len(s.checks))
	}
	if s.checks[0].Message != "" {
		t.Errorf("expected a passing check to have no message, got %q", s.checks[0].Message)
	}
	if s.checks[1].Message != "too slow" {
		t.Errorf("expected the failure message, got %q", s.checks[1].Message)
	}
	if s.checks[1].Duration != 5*time.Second {
		t.Errorf("expected the recorded duration, got %s", s.checks[1].Duration)
	}
}

func TestMarkScenarioOutcomeTreatsSkipAsNotFailed(t *testing.T) {
	failing := &Scenario{}
	markScenarioOutcome(failing, errors.New("boom"), nil)
	if !failing.failed {
		t.Error("expected an error to mark the scenario failed")
	}

	skipped := &Scenario{}
	markScenarioOutcome(skipped, fmt.Errorf("wrapped: %w", &skipError{message: "no capacity"}), nil)
	if skipped.failed {
		t.Error("expected a skip not to mark the scenario failed")
	}

	passing := &Scenario{}
	markScenarioOutcome(passing, nil, nil)
	if passing.failed {
		t.Error("expected a successful scenario not to be marked failed")
	}
}

func TestSkipIfSKUNotAvailableErr(t *testing.T) {
	original := config.Config.SkipTestsWithSKUCapacityIssue
	config.Config.SkipTestsWithSKUCapacityIssue = true
	t.Cleanup(func() { config.Config.SkipTestsWithSKUCapacityIssue = original })

	notAvailable := &azcore.ResponseError{StatusCode: http.StatusConflict, ErrorCode: "SkuNotAvailable"}
	var skip *skipError
	if err := skipIfSKUNotAvailableErr(fmt.Errorf("create vmss: %w", notAvailable)); !errors.As(err, &skip) {
		t.Errorf("expected a skip for an unavailable SKU, got %v", err)
	}

	other := &azcore.ResponseError{StatusCode: http.StatusConflict, ErrorCode: "Conflict"}
	if err := skipIfSKUNotAvailableErr(other); err != nil {
		t.Errorf("expected no skip for an unrelated conflict, got %v", err)
	}
	if err := skipIfSKUNotAvailableErr(nil); err != nil {
		t.Errorf("expected no skip without an error, got %v", err)
	}

	config.Config.SkipTestsWithSKUCapacityIssue = false
	if err := skipIfSKUNotAvailableErr(notAvailable); err != nil {
		t.Errorf("expected no skip when the option is disabled, got %v", err)
	}
}

func TestScenarioVMSizeIsScenarioLocal(t *testing.T) {
	if got := scenarioVMSize(&Scenario{}); got != config.Config.DefaultVMSKU {
		t.Errorf("expected the configured default before runtime state exists, got %q", got)
	}
	s := &Scenario{Runtime: &ScenarioRuntime{VMSize: config.DefaultV5VMSKU}}
	if got := scenarioVMSize(s); got != config.DefaultV5VMSKU {
		t.Errorf("expected the scenario-local VM size, got %q", got)
	}
	if config.Config.DefaultVMSKU == config.DefaultV5VMSKU {
		return
	}
	if scenarioVMSize(&Scenario{}) != config.Config.DefaultVMSKU {
		t.Error("a scenario-local fallback must not change the configured default")
	}
}
