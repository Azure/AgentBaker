package e2e

import "testing"

func TestTagsMatchesRequiresCurrentSourceVHDFilter(t *testing.T) {
	tags := Tags{RequiresCurrentSourceVHD: true}

	matches, err := tags.MatchesAnyFilter("RequiresCurrentSourceVHD=true")
	if err != nil {
		t.Fatalf("matching RequiresCurrentSourceVHD tag: %v", err)
	}
	if !matches {
		t.Fatal("expected RequiresCurrentSourceVHD tag to match")
	}
}
