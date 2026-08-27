package config

import "testing"

func TestPollUntilDoneOptionsAreNotShared(t *testing.T) {
	first := PollUntilDoneOptions()
	second := PollUntilDoneOptions()
	if first == second {
		t.Fatal("expected independent options per operation, got the same pointer")
	}
	if first.Frequency != defaultPollUntilDoneFrequency {
		t.Errorf("expected the fixed poll frequency %s, got %s", defaultPollUntilDoneFrequency, first.Frequency)
	}
	first.Frequency = 0
	if second.Frequency != defaultPollUntilDoneFrequency {
		t.Error("mutating one set of options must not affect another")
	}
}
