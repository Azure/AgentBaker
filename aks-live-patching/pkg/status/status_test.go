package status

import (
	"testing"
)

const goal = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestParse(t *testing.T) {
	t.Run("empty yields zero status", func(t *testing.T) {
		s, err := Parse("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.CurrentHash != "" || len(s.Components) != 0 {
			t.Fatalf("expected zero status, got %+v", s)
		}
	})

	t.Run("parses the document written by the node", func(t *testing.T) {
		// Same shape as write_generic_status in the node-side live-patching loop.
		data := `{"currentHash":"` + goal + `","components":{` +
			`"securityPatch":{"code":"Succeeded","current":"20260505T000000Z"},` +
			`"npdConfig":{"code":"Failed","message":"image pull error"}}}`
		s, err := Parse(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.CurrentHash != goal {
			t.Errorf("CurrentHash = %q, want %q", s.CurrentHash, goal)
		}
		if got := s.Components[ComponentSecurityPatch]; got.Code != CodeSucceeded || got.Current != "20260505T000000Z" {
			t.Errorf("securityPatch = %+v", got)
		}
		if got := s.Components[ComponentNPDConfig]; got.Code != "Failed" || got.Message != "image pull error" {
			t.Errorf("npdConfig = %+v", got)
		}
	})

	t.Run("malformed json errors", func(t *testing.T) {
		if _, err := Parse("{not json"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestStatusChecks(t *testing.T) {
	succeeded := ComponentStatus{Code: CodeSucceeded}
	failed := ComponentStatus{Code: "Failed"}

	tests := []struct {
		name                   string
		status                 *Status
		goalHash               string
		wantProcessed          bool
		wantConverged          bool
		wantSecurityPatchReady bool
	}{
		{name: "nil status", status: nil, goalHash: goal},
		{name: "empty goal hash", status: &Status{CurrentHash: ""}, goalHash: ""},
		{name: "hash mismatch", status: &Status{CurrentHash: "old", Components: map[string]ComponentStatus{ComponentSecurityPatch: succeeded}}, goalHash: goal},
		{
			name:          "processed, no components",
			status:        &Status{CurrentHash: goal},
			goalHash:      goal,
			wantProcessed: true, wantConverged: true,
		},
		{
			name:          "all succeeded",
			status:        &Status{CurrentHash: goal, Components: map[string]ComponentStatus{ComponentSecurityPatch: succeeded, ComponentNPDConfig: succeeded}},
			goalHash:      goal,
			wantProcessed: true, wantConverged: true, wantSecurityPatchReady: true,
		},
		{
			name:          "securityPatch succeeded, other component failed",
			status:        &Status{CurrentHash: goal, Components: map[string]ComponentStatus{ComponentSecurityPatch: succeeded, ComponentNPDConfig: failed}},
			goalHash:      goal,
			wantProcessed: true, wantSecurityPatchReady: true,
		},
		{
			name:          "securityPatch failed",
			status:        &Status{CurrentHash: goal, Components: map[string]ComponentStatus{ComponentSecurityPatch: failed}},
			goalHash:      goal,
			wantProcessed: true,
		},
		{
			name:          "securityPatch missing",
			status:        &Status{CurrentHash: goal, Components: map[string]ComponentStatus{ComponentNPDConfig: succeeded}},
			goalHash:      goal,
			wantProcessed: true, wantConverged: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.status.IsProcessed(tc.goalHash); got != tc.wantProcessed {
				t.Errorf("IsProcessed = %v, want %v", got, tc.wantProcessed)
			}
			if got := tc.status.IsConverged(tc.goalHash); got != tc.wantConverged {
				t.Errorf("IsConverged = %v, want %v", got, tc.wantConverged)
			}
			if got := tc.status.ComponentSucceeded(tc.goalHash, ComponentSecurityPatch); got != tc.wantSecurityPatchReady {
				t.Errorf("ComponentSucceeded(securityPatch) = %v, want %v", got, tc.wantSecurityPatchReady)
			}
		})
	}
}
