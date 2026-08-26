package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestContainerdConfigEntryDiff_NamesDifferingEntry(t *testing.T) {
	pcB64 := base64.StdEncoding.EncodeToString([]byte("version = 4\noom_score = -999\n"))
	nbcB64 := base64.StdEncoding.EncodeToString([]byte("version = 4\noom_score = -999\nroot = \"/mnt/aks/containers\"\n"))
	// provision-config value is bare; nbc-cmd value is quote-wrapped (as parsed from the shell assignment).
	pc := pcB64
	nbc := "\"" + nbcB64 + "\""
	got := containerdConfigEntryDiff("CONTAINERD_CONFIG_CONTENT", pc, nbc)
	if !strings.Contains(got, "root") || !strings.Contains(got, "only-in-nbc-cmd") {
		t.Fatalf("expected diff to name the root entry as only-in-nbc-cmd, got %q", got)
	}
	if d := containerdConfigEntryDiff("SOME_OTHER_VAR", pc, nbc); d != "" {
		t.Fatalf("expected empty for non-containerd key, got %q", d)
	}
}
