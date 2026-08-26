package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestContainerdConfigEntryDiff_NamesDifferingEntry(t *testing.T) {
	pc := base64.StdEncoding.EncodeToString([]byte("version = 4\noom_score = -999\n"))
	nbc := base64.StdEncoding.EncodeToString([]byte("version = 4\noom_score = -999\nroot = \"/mnt/aks/containers\"\n"))
	got := containerdConfigEntryDiff("CONTAINERD_CONFIG_CONTENT", pc, nbc)
	if !strings.Contains(got, "root") || !strings.Contains(got, "only-in-nbc-cmd") {
		t.Fatalf("expected diff to name the root entry as only-in-nbc-cmd, got %q", got)
	}
	// non-containerd key returns empty
	if d := containerdConfigEntryDiff("SOME_OTHER_VAR", pc, nbc); d != "" {
		t.Fatalf("expected empty for non-containerd key, got %q", d)
	}
}
