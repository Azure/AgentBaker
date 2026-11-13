package config

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveSIGVersionTag(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("git test executable uses a POSIX shell")
	}
	for _, tc := range []struct {
		name   string
		script string
		want   string
		err    string
	}{
		{
			name:   "remote default is dev",
			script: "printf 'ref: refs/heads/dev\\tHEAD\\n0123456789\\tHEAD\\n'",
			want:   "refs/heads/dev",
		},
		{
			name:   "remote default changed",
			script: "printf 'ref: refs/heads/release/default\\tHEAD\\n0123456789\\tHEAD\\n'",
			want:   "refs/heads/release/default",
		},
		{
			name:   "detached remote HEAD",
			script: "printf '0123456789\\tHEAD\\n'",
			err:    "origin did not advertise a default branch",
		},
		{
			name:   "non-branch HEAD",
			script: "printf 'ref: refs/tags/v1\\tHEAD\\n'",
			err:    "origin did not advertise a default branch",
		},
		{
			name:   "remote query failed",
			script: "exit 1",
			err:    "resolve default SIG branch from origin",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0o755))
			t.Setenv("PATH", bin)
			cfg := DefaultConfiguration()
			err := cfg.resolveSIGVersionTag(context.Background())
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.Empty(t, cfg.SIGVersionTagValue)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, cfg.SIGVersionTagValue)
			}
		})
	}
}

func TestResolveSIGVersionTagPreservesExplicitSelection(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tagName  string
		tagValue string
		metadata string
	}{
		{name: "explicit branch", tagName: "branch", tagValue: "refs/heads/main"},
		{name: "explicit build", tagName: "buildId", tagValue: "190956218"},
		{name: "unfiltered selection"},
		{name: "metadata selection", tagName: "branch", metadata: "vhd-metadata.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			cfg := DefaultConfiguration()
			cfg.SIGVersionTagName = tc.tagName
			cfg.SIGVersionTagValue = tc.tagValue
			cfg.VHDMetadataFile = tc.metadata
			require.NoError(t, cfg.resolveSIGVersionTag(context.Background()))
			require.Equal(t, tc.tagValue, cfg.SIGVersionTagValue)
		})
	}
}

func TestResolveSIGVersionTagCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := DefaultConfiguration()
	require.ErrorIs(t, cfg.resolveSIGVersionTag(ctx), context.Canceled)
	require.Empty(t, cfg.SIGVersionTagValue)
}
