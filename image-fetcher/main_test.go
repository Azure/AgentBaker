package main

import (
	"context"
	"strings"
	"testing"
)

func TestTransferBeforeSizing(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		snapshotter string
		fetchOnly   bool
		expected    bool
	}{
		{
			name:        "overlayfs retains referrers before local unpack",
			snapshotter: "overlayfs",
			expected:    true,
		},
		{
			name:        "overlayfs fetch-only uses transfer",
			snapshotter: "overlayfs",
			fetchOnly:   true,
			expected:    true,
		},
		{
			name:        "EROFS fetch-only retains referrers",
			snapshotter: "erofs",
			fetchOnly:   true,
			expected:    true,
		},
		{
			name:        "EROFS sizes before immediate transfer unpack",
			snapshotter: "erofs",
			expected:    false,
		},
		{
			name:     "legacy path keeps existing fetch-first behavior",
			expected: false,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := transferBeforeSizing(testCase.snapshotter, testCase.fetchOnly); actual != testCase.expected {
				t.Fatalf("expected %t, got %t", testCase.expected, actual)
			}
		})
	}
}

func TestShouldUnpack(t *testing.T) {
	if !shouldUnpack(pullSizeThreshold - 1) {
		t.Fatal("image below the threshold should unpack")
	}
	if shouldUnpack(pullSizeThreshold) {
		t.Fatal("image exactly at the threshold should remain packed")
	}
	if shouldUnpack(pullSizeThreshold + 1) {
		t.Fatal("image above the threshold should remain packed")
	}
}

func TestFetchImageRejectsUnsupportedTransferSnapshotter(t *testing.T) {
	t.Setenv(transferSnapshotterEnv, "overlaybd")

	err := fetchImage(context.Background(), nil, "example.invalid/image:v1")
	if err == nil || !strings.Contains(err.Error(), "unsupported "+transferSnapshotterEnv) {
		t.Fatalf("expected unsupported snapshotter error, got %v", err)
	}
}
