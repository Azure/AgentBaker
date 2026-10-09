package image

import "testing"

func TestIsID(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		want      bool
	}{
		{
			name:      "containerd image ID",
			imageName: "sha256:5d563037009d5808012da05e785268aad175765d2f4473d70b6ac8da19f6bcfc",
			want:      true,
		},
		{
			name:      "tag pinned to digest",
			imageName: "mcr.microsoft.com/geneva/mdsd:recommended@sha256:a028b633d37e1f0dc9aa93ce75aaa1ebe35d74ccddd243cdbad4c68cc4c79100",
			want:      false,
		},
		{
			name:      "ordinary tag",
			imageName: "mcr.microsoft.com/oss/v2/kubernetes/pause:3.10.1",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isID(tt.imageName); got != tt.want {
				t.Fatalf("isID(%q) = %t, want %t", tt.imageName, got, tt.want)
			}
		})
	}
}
