package image

import "testing"

func TestIsID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"bare sha256 id", "sha256:1f2972bc2f1e4f7e3c2bef9cb382859a0cdd5458465a72c0c9568083b8007f3c", true},
		{"tag reference", "mcr.microsoft.com/oss/kubernetes/pause:3.6", false},
		{"digest reference", "notarycontainerregistry.azurecr.io/notary-demo@sha256:1f2972bc2f1e4f7e3c2bef9cb382859a0cdd5458465a72c0c9568083b8007f3c", false},
		{"digest reference no registry path", "name@sha256:abc", false},
		{"empty", "", false},
		{"sha256 substring in tag", "sha256-test:latest", false},
		{"unrelated", "image:tag", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isID(tc.in); got != tc.want {
				t.Fatalf("isID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
