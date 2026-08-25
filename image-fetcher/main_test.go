package main

import (
	"context"
	"strings"
	"testing"
)

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
	for _, snapshotter := range []string{"erofs", "overlaybd"} {
		t.Run(snapshotter, func(t *testing.T) {
			t.Setenv(transferSnapshotterEnv, snapshotter)

			err := fetchImage(context.Background(), nil, "example.invalid/image:v1")
			if err == nil || !strings.Contains(err.Error(), "unsupported "+transferSnapshotterEnv) {
				t.Fatalf("expected unsupported snapshotter error, got %v", err)
			}
		})
	}
}
