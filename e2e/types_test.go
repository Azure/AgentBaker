package e2e

import "testing"

func TestTagsMatchesRequiresCurrentSourceVHDFilter(t *testing.T) {
	testCases := []struct {
		name        string
		tags        Tags
		wantMatches bool
	}{
		{
			name:        "tag set",
			tags:        Tags{RequiresCurrentSourceVHD: true},
			wantMatches: true,
		},
		{
			name:        "tag not set",
			tags:        Tags{},
			wantMatches: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			matches, err := testCase.tags.MatchesAnyFilter("RequiresCurrentSourceVHD=true")
			if err != nil {
				t.Fatalf("matching RequiresCurrentSourceVHD tag: %v", err)
			}
			if matches != testCase.wantMatches {
				t.Fatalf("got match %t, want %t", matches, testCase.wantMatches)
			}
		})
	}
}
