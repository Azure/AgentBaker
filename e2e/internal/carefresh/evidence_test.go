package carefresh

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func snapshotOutput() string {
	out := "BundlePath=/etc/ssl/certs/ca-certificates.crt\nBundleSHA256=" + strings.Repeat("a", 64)
	for _, service := range []string{"kubelet", "containerd"} {
		out += fmt.Sprintf("\n[%s]\nId=%s.service\nActiveState=active\nSubState=running\nMainPID=100\nExecMainStartTimestampMonotonic=1000\nInvocationID=%s\nNRestarts=0",
			service, service, strings.Repeat("b", 32))
	}
	return out + "\n"
}

func TestParseSnapshot(t *testing.T) {
	valid := snapshotOutput()
	for _, output := range []string{valid, strings.ReplaceAll(valid, "/etc/ssl/certs/ca-certificates.crt", "/etc/pki/tls/certs/ca-bundle.crt"), strings.ReplaceAll(valid, "\n[", "\n\n[")} {
		s, err := ParseSnapshot(output)
		if err != nil || s.Containerd.PID != 100 || s.Kubelet.Started != 1000 {
			t.Fatalf("valid snapshot: %+v %v", s, err)
		}
	}
	for _, test := range []struct{ old, replacement string }{
		{"MainPID=100", "MainPID=0"}, {"MainPID=100", "MainPID=bad"},
		{"ExecMainStartTimestampMonotonic=1000", "ExecMainStartTimestampMonotonic=0"},
		{"ActiveState=active", "ActiveState=inactive"}, {"SubState=running", "SubState=failed"},
		{"NRestarts=0", "NRestarts=-1"}, {"NRestarts=0", "NRestarts=0\nNRestarts=0"},
		{strings.Repeat("b", 32), ""}, {strings.Repeat("b", 32), strings.Repeat("0", 32)},
		{strings.Repeat("a", 64), "garbage"},
		{"[containerd]", "[kubelet]"}, {"Id=containerd.service", "Id=other.service"},
		{"/etc/ssl/certs/ca-certificates.crt", "/custom/bundle"},
	} {
		if _, err := ParseSnapshot(strings.Replace(valid, test.old, test.replacement, 1)); err == nil {
			t.Errorf("accepted invalid snapshot mutation %+v", test)
		}
	}
}

func TestResult(t *testing.T) {
	for _, marker := range []string{"unchanged", "restarted", "recovered", "inactive"} {
		result, err := Result("+ echo CA_REFRESH_RESULT=restarted\nCA_REFRESH_RESULT=" + marker + "\n")
		if err != nil || result != marker {
			t.Fatalf("result %q: %q %v", marker, result, err)
		}
	}
	for _, output := range []string{
		"", "+ echo CA_REFRESH_RESULT=restarted", "prefix CA_REFRESH_RESULT=unchanged",
		"CA_REFRESH_RESULT=bad", "CA_REFRESH_RESULT=unchanged\nCA_REFRESH_RESULT=restarted",
		"CA_REFRESH_RESULT=unchanged suffix", "CA_REFRESH_RESULT=\nCA_REFRESH_RESULT=unchanged",
	} {
		if _, err := Result(output); err == nil {
			t.Errorf("accepted invalid result %q", output)
		}
	}
}

func TestTransition(t *testing.T) {
	before, err := ParseSnapshot(snapshotOutput())
	if err != nil {
		t.Fatal(err)
	}
	restarted := before
	restarted.BundleSHA256 = strings.Repeat("c", 64)
	restarted.Containerd = ServiceIdentity{PID: 200, Started: 2000, InvocationID: strings.Repeat("d", 32)}
	tests := []struct {
		name, marker string
		after        Snapshot
		valid        bool
	}{
		{"no-op", "unchanged", before, true},
		{"manual restart NRestarts still zero", "restarted", restarted, true},
		{"recovered new instance", "recovered", restarted, true},
		{"false restart marker", "restarted", before, false},
		{"false unchanged marker", "unchanged", restarted, false},
		{"inactive not allowed on running node", "inactive", before, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateTransition(before, test.after, test.marker); (err == nil) != test.valid {
				t.Fatalf("unexpected transition result: %v", err)
			}
		})
	}
	for _, change := range []func(*Snapshot){
		func(s *Snapshot) { s.BundleSHA256 = before.BundleSHA256 },
		func(s *Snapshot) { s.BundlePath = "/etc/pki/tls/certs/ca-bundle.crt" },
		func(s *Snapshot) { s.Containerd.PID = before.Containerd.PID },
		func(s *Snapshot) { s.Containerd.Started = before.Containerd.Started },
		func(s *Snapshot) { s.Containerd.Started = before.Containerd.Started - 1 },
		func(s *Snapshot) { s.Containerd.InvocationID = before.Containerd.InvocationID },
		func(s *Snapshot) { s.Containerd.Restarts++ },
		func(s *Snapshot) { s.Kubelet.PID++ },
		func(s *Snapshot) { s.Kubelet.Restarts++ },
	} {
		after := restarted
		change(&after)
		if err := ValidateTransition(before, after, "restarted"); err == nil {
			t.Errorf("accepted invalid transition %+v", after)
		}
	}
	// A later unrelated restart must not be tolerated because the first restart
	// was authorized. Compare against the post-refresh instance, not baseline.
	later := restarted
	later.Containerd.InvocationID = strings.Repeat("e", 32)
	if ValidateStable(restarted, later) == nil || ValidateStable(restarted, restarted) != nil {
		t.Fatal("post-refresh stability checks must detect subsequent restarts")
	}
}

func TestTimeoutBudgets(t *testing.T) {
	recovery := 90*time.Second + 2*12*(5*time.Second+5*time.Second+5*time.Second)
	acquisitionAndLocking := 5*time.Minute + time.Minute
	if MaxJitter != 300*time.Second || RefreshTimeout < MaxJitter+recovery+acquisitionAndLocking+time.Minute ||
		HelperTimeout <= RefreshTimeout || FixtureTimeout <= 2*RefreshTimeout ||
		HealthTimeout <= 2*HelperTimeout {
		t.Fatal("timeouts must allow actual jitter, acquisition/recovery, and a second refresh")
	}
}

func TestFixtureReport(t *testing.T) {
	s, err := ParseSnapshot(snapshotOutput())
	if err != nil {
		t.Fatal(err)
	}
	want := FixtureReport{Before: s, After: s}
	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	line := FixtureReportPrefix + string(encoded)
	got, err := ParseFixtureReport("other output\n" + line + "\n")
	if err != nil || got != want {
		t.Fatalf("report roundtrip: %+v %v", got, err)
	}
	for _, invalid := range []string{"", FixtureReportPrefix + "{}", FixtureReportPrefix + "bad", line + "\n" + line} {
		if _, err := ParseFixtureReport(invalid); err == nil {
			t.Errorf("accepted invalid fixture report %q", invalid)
		}
	}
}
