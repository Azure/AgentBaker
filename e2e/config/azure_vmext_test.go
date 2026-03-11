package config

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/toolkit"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
)

// mockVMExtensionImageVersionLister implements vmExtensionImageVersionLister for testing.
type mockVMExtensionImageVersionLister struct {
	resp armcompute.VirtualMachineExtensionImagesClientListVersionsResponse
	err  error
}

func (m *mockVMExtensionImageVersionLister) ListVersions(
	ctx context.Context,
	location string,
	publisherName string,
	typeParam string,
	options *armcompute.VirtualMachineExtensionImagesClientListVersionsOptions,
) (armcompute.VirtualMachineExtensionImagesClientListVersionsResponse, error) {
	return m.resp, m.err
}

// makeVersionResponse builds a ListVersionsResponse from a list of version name pointers.
// Pass nil to represent an image with a nil Name field.
func makeVersionResponse(versions ...*string) armcompute.VirtualMachineExtensionImagesClientListVersionsResponse {
	images := make([]*armcompute.VirtualMachineExtensionImage, len(versions))
	for i, v := range versions {
		images[i] = &armcompute.VirtualMachineExtensionImage{Name: v}
	}
	return armcompute.VirtualMachineExtensionImagesClientListVersionsResponse{
		VirtualMachineExtensionImageArray: images,
	}
}

func Test_parseVersion(t *testing.T) {
	tests := []struct {
		name          string
		inputName     *string
		expectedMajor int
		expectedMinor int
		expectedPatch int
	}{
		{
			name:          "three-part version",
			inputName:     to.Ptr("1.0.1"),
			expectedMajor: 1,
			expectedMinor: 0,
			expectedPatch: 1,
		},
		{
			name:          "two-part version",
			inputName:     to.Ptr("1.151"),
			expectedMajor: 1,
			expectedMinor: 151,
			expectedPatch: 0,
		},
		{
			name:          "single-part version",
			inputName:     to.Ptr("5"),
			expectedMajor: 5,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "nil name",
			inputName:     nil,
			expectedMajor: 0,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "non-numeric parts",
			inputName:     to.Ptr("abc.def.ghi"),
			expectedMajor: 0,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "partially numeric",
			inputName:     to.Ptr("2.abc.3"),
			expectedMajor: 2,
			expectedMinor: 0,
			expectedPatch: 3,
		},
		{
			name:          "empty string",
			inputName:     to.Ptr(""),
			expectedMajor: 0,
			expectedMinor: 0,
			expectedPatch: 0,
		},
		{
			name:          "extra parts ignored",
			inputName:     to.Ptr("1.2.3.4"),
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
		},
		{
			name:          "large numbers",
			inputName:     to.Ptr("100.200.300"),
			expectedMajor: 100,
			expectedMinor: 200,
			expectedPatch: 300,
		},
		{
			name:          "leading zeros",
			inputName:     to.Ptr("01.02.03"),
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := toolkit.ContextWithT(context.Background(), t)
			img := &armcompute.VirtualMachineExtensionImage{Name: tt.inputName}
			result := parseVersion(ctx, img)

			if result.major != tt.expectedMajor {
				t.Errorf("major: got %d, want %d", result.major, tt.expectedMajor)
			}
			if result.minor != tt.expectedMinor {
				t.Errorf("minor: got %d, want %d", result.minor, tt.expectedMinor)
			}
			if result.patch != tt.expectedPatch {
				t.Errorf("patch: got %d, want %d", result.patch, tt.expectedPatch)
			}
			if result.original != img {
				t.Errorf("original: got %p, want %p", result.original, img)
			}
		})
	}
}

func Test_vmExtensionVersion_cmp(t *testing.T) {
	tests := []struct {
		name     string
		a        vmExtensionVersion
		b        vmExtensionVersion
		expected int
	}{
		{
			name:     "equal",
			a:        vmExtensionVersion{major: 1, minor: 2, patch: 3},
			b:        vmExtensionVersion{major: 1, minor: 2, patch: 3},
			expected: 0,
		},
		{
			name:     "a higher major",
			a:        vmExtensionVersion{major: 2, minor: 0, patch: 0},
			b:        vmExtensionVersion{major: 1, minor: 9, patch: 9},
			expected: 1,
		},
		{
			name:     "a lower major",
			a:        vmExtensionVersion{major: 1, minor: 9, patch: 9},
			b:        vmExtensionVersion{major: 2, minor: 0, patch: 0},
			expected: -1,
		},
		{
			name:     "same major, a higher minor",
			a:        vmExtensionVersion{major: 1, minor: 5, patch: 0},
			b:        vmExtensionVersion{major: 1, minor: 3, patch: 9},
			expected: 1,
		},
		{
			name:     "same major, a lower minor",
			a:        vmExtensionVersion{major: 1, minor: 3, patch: 9},
			b:        vmExtensionVersion{major: 1, minor: 5, patch: 0},
			expected: -1,
		},
		{
			name:     "same major+minor, a higher patch",
			a:        vmExtensionVersion{major: 1, minor: 2, patch: 5},
			b:        vmExtensionVersion{major: 1, minor: 2, patch: 3},
			expected: 1,
		},
		{
			name:     "same major+minor, a lower patch",
			a:        vmExtensionVersion{major: 1, minor: 2, patch: 3},
			b:        vmExtensionVersion{major: 1, minor: 2, patch: 5},
			expected: -1,
		},
		{
			name:     "both zero",
			a:        vmExtensionVersion{major: 0, minor: 0, patch: 0},
			b:        vmExtensionVersion{major: 0, minor: 0, patch: 0},
			expected: 0,
		},
		{
			name:     "zero vs non-zero",
			a:        vmExtensionVersion{major: 0, minor: 0, patch: 0},
			b:        vmExtensionVersion{major: 0, minor: 0, patch: 1},
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.a.cmp(tt.b)
			if got != tt.expected {
				t.Errorf("(%v).cmp(%v) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func Test_getLatestVMExtensionImageVersion(t *testing.T) {
	tests := []struct {
		name        string
		mock        *mockVMExtensionImageVersionLister
		expected    string
		errContains string
	}{
		{
			name: "multiple versions, returns latest",
			mock: &mockVMExtensionImageVersionLister{
				resp: makeVersionResponse(to.Ptr("1.0.0"), to.Ptr("2.1.0"), to.Ptr("1.5.3")),
			},
			expected: "2.1.0",
		},
		{
			name: "single version",
			mock: &mockVMExtensionImageVersionLister{
				resp: makeVersionResponse(to.Ptr("3.2.1")),
			},
			expected: "3.2.1",
		},
		{
			name: "two-part versions",
			mock: &mockVMExtensionImageVersionLister{
				resp: makeVersionResponse(to.Ptr("1.100"), to.Ptr("1.151"), to.Ptr("1.50")),
			},
			expected: "1.151",
		},
		{
			name: "API error propagated",
			mock: &mockVMExtensionImageVersionLister{
				err: fmt.Errorf("network failure"),
			},
			errContains: "listing extension versions",
		},
		{
			name: "empty list",
			mock: &mockVMExtensionImageVersionLister{
				resp: makeVersionResponse(),
			},
			errContains: "no extension versions found",
		},
		{
			name: "all nil names",
			mock: &mockVMExtensionImageVersionLister{
				resp: makeVersionResponse(nil),
			},
			errContains: "latest extension version has nil name",
		},
		{
			name: "mix valid and malformed",
			mock: &mockVMExtensionImageVersionLister{
				resp: makeVersionResponse(to.Ptr("abc"), to.Ptr("1.2.3"), to.Ptr("xyz")),
			},
			expected: "1.2.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := toolkit.ContextWithT(context.Background(), t)
			got, err := getLatestVMExtensionImageVersion(
				ctx,
				tt.mock,
				"eastus",
				"TestExtension",
				"TestPublisher",
			)

			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
