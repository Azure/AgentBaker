package main

import (
	"slices"
	"strings"
	"testing"
)

func TestResolveImageReferences(t *testing.T) {
	data := []byte(`{
		"ContainerImages": [
			{
				"downloadURL": "registry.example/mixed:*",
				"amd64OnlyVersions": ["amd64"],
				"multiArchVersions": ["ignored"],
				"multiArchVersionsV2": [
					{"latestVersion": "latest", "previousLatestVersion": "previous"},
					{"latestVersion": "latest2", "previousLatestVersion": null}
				],
				"windowsVersions": [{"latestVersion": "windows"}]
			},
			{"downloadURL": "registry.example/legacy:*", "multiArchVersions": ["old"]},
			{"downloadURL": "registry.example/null:*", "multiArchVersionsV2": null, "multiArchVersions": ["fallback"]},
			{"downloadURL": "registry.example/empty:*", "multiArchVersionsV2": [], "multiArchVersions": ["ignored"]},
			{"downloadURL": "registry.example/windows:*", "windowsVersions": [{"latestVersion": "windows"}]},
			{"downloadURL": "registry.example/skip:*", "multiArchVersionsV2": [{"latestVersion": null}]},
			{"downloadURL": "registry.example/repeated-*:*", "multiArchVersions": ["v1"]}
		],
		"Packages": [{"name": "ignored"}],
		"GPUContainerImages": [{"downloadURL": "registry.example/gpu:*", "gpuVersion": {"latestVersion": "ignored"}}]
	}`)
	for _, arch := range []string{"amd64", "arm64"} {
		t.Run(arch, func(t *testing.T) {
			want := []string{
				"registry.example/mixed:latest",
				"registry.example/mixed:previous",
				"registry.example/mixed:latest2",
				"registry.example/legacy:old",
				"registry.example/null:fallback",
				"registry.example/repeated-v1:v1",
			}
			if arch == "amd64" {
				want = append([]string{"registry.example/mixed:amd64"}, want...)
			}
			got, err := resolveImageReferences(data, arch)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, want) {
				t.Fatalf("references = %v, want %v", got, want)
			}
		})
	}
}

func TestResolveImageReferencesInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name, data, arch, want string
	}{
		{"invalid JSON", `{`, "amd64", "decode components"},
		{"missing images", `{}`, "amd64", "ContainerImages must be an array"},
		{"null images", `{"ContainerImages":null}`, "amd64", "ContainerImages must be an array"},
		{"wrong type", `{"ContainerImages":{}}`, "amd64", "decode components"},
		{"wrong version type", `{"ContainerImages":[{"multiArchVersionsV2":"bad"}]}`, "amd64", "decode components"},
		{"missing URL", `{"ContainerImages":[{"multiArchVersions":["v1"]}]}`, "amd64", "no downloadURL"},
		{"unsupported arch", `{"ContainerImages":[]}`, "s390x", "unsupported architecture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveImageReferences([]byte(tc.data), tc.arch)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestResolveImageReferencesEmpty(t *testing.T) {
	refs, err := resolveImageReferences([]byte(`{"ContainerImages":[]}`), "amd64")
	if err != nil || len(refs) != 0 {
		t.Fatalf("references = %v, error = %v", refs, err)
	}
}
