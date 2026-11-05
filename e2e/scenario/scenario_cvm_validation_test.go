package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cvmTestImageID = "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/galleries/test/images/cvm/versions/1.2.3"

func cvmModelTestScenario(name string, vmss *armcompute.VirtualMachineScaleSet) *Scenario {
	return &Scenario{
		artifactName: name,
		Runtime: &ScenarioRuntime{
			ResolvedVHD: cvmTestImageID,
			VM: &ScenarioVM{
				VMSS: vmss,
				VM: &armcompute.VirtualMachineScaleSetVM{
					InstanceID: to.Ptr("0"),
					Properties: &armcompute.VirtualMachineScaleSetVMProperties{
						StorageProfile: &armcompute.StorageProfile{ImageReference: &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID)}},
					},
				},
			},
		},
	}
}

func TestValidateObservedCVMModel(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	vmss := &armcompute.VirtualMachineScaleSet{
		Location: to.Ptr(cvmLocation),
		Properties: &armcompute.VirtualMachineScaleSetProperties{
			VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
				StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID)},
				},
			},
		},
	}
	configureConfidentialVM(vmss, cvmSNPSKU, armcompute.SecurityEncryptionTypesVMGuestStateOnly)
	scenario := cvmModelTestScenario("cvm-model", vmss)

	checks, err := validateObservedCVMModel(scenario, cvmPlatformSEVSNP, armcompute.SecurityEncryptionTypesVMGuestStateOnly)
	require.NoError(t, err)
	require.NotEmpty(t, checks)
	for _, check := range checks {
		assert.True(t, check.Passed, check.Name)
	}
	for _, name := range []string{"vmss-model.json", "disk-security.json", "encryption-at-host.json"} {
		_, err := os.Stat(filepath.Join(loggingDir, "cvm-model", "cvm", name))
		require.NoError(t, err)
	}
}

func TestValidateObservedCVMModelReportsConfigurationDrift(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	scenario := &Scenario{
		artifactName: "cvm-drift",
		Runtime: &ScenarioRuntime{VM: &ScenarioVM{VMSS: &armcompute.VirtualMachineScaleSet{
			Location: to.Ptr("eastus"),
			SKU:      &armcompute.SKU{Name: to.Ptr("wrong-sku")},
		}}},
	}

	checks, err := validateObservedCVMModel(scenario, cvmPlatformTDX, armcompute.SecurityEncryptionTypesDiskWithVMGuestState)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "vmss.location")
	assert.Condition(t, func() bool {
		for _, check := range checks {
			if !check.Passed {
				return true
			}
		}
		return false
	})
}

func TestValidateObservedCVMDiskSecurity(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	for _, platform := range []cvmPlatform{cvmPlatformSEVSNP, cvmPlatformTDX} {
		for _, encryptionType := range []armcompute.SecurityEncryptionTypes{
			armcompute.SecurityEncryptionTypesVMGuestStateOnly,
			armcompute.SecurityEncryptionTypesDiskWithVMGuestState,
		} {
			t.Run(string(platform)+"/"+string(encryptionType), func(t *testing.T) {
				sku := cvmSNPSKU
				if platform == cvmPlatformTDX {
					sku = cvmTDXSKU
				}
				vmss := &armcompute.VirtualMachineScaleSet{
					Location: to.Ptr(cvmLocation),
					Properties: &armcompute.VirtualMachineScaleSetProperties{
						VirtualMachineProfile: &armcompute.VirtualMachineScaleSetVMProfile{
							StorageProfile: &armcompute.VirtualMachineScaleSetStorageProfile{
								ImageReference: &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID)},
							},
						},
					},
				}
				configureConfidentialVM(vmss, sku, encryptionType)
				s := cvmModelTestScenario("cvm-disk-security", vmss)

				_, err := validateObservedCVMModel(s, platform, encryptionType)
				require.NoError(t, err)

				otherType := armcompute.SecurityEncryptionTypesVMGuestStateOnly
				if encryptionType == otherType {
					otherType = armcompute.SecurityEncryptionTypesDiskWithVMGuestState
				}
				checks, err := validateObservedCVMModel(s, platform, otherType)
				require.ErrorContains(t, err, "vmss.osDiskSecurity")
				for _, check := range checks {
					if check.Name == "vmss.osDiskSecurity" {
						assert.Equal(t, otherType, check.Expected)
						assert.Equal(t, encryptionType, check.Observed)
						assert.False(t, check.Passed)
					}
				}
			})
		}
	}
}

func TestCVMImageReferenceMatchesResolvedVersion(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		observed *armcompute.ImageReference
		want     bool
	}{
		{name: "exact", expected: cvmTestImageID, observed: &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID)}, want: true},
		{name: "ARM casing", expected: cvmTestImageID, observed: &armcompute.ImageReference{ID: to.Ptr(strings.ToUpper(cvmTestImageID))}, want: true},
		{name: "different version", expected: cvmTestImageID, observed: &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID + "4")}},
		{name: "empty reference", expected: cvmTestImageID, observed: &armcompute.ImageReference{}},
		{name: "missing reference", expected: cvmTestImageID},
		{name: "unresolved", observed: &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID)}},
		{name: "moving latest alias", expected: "/images/cvm/versions/latest", observed: &armcompute.ImageReference{ID: to.Ptr("/images/cvm/versions/latest")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, cvmImageMatches(tt.expected, tt.observed))
		})
	}
}

func TestRefreshCVMModelDoesNotTrustCreationResponse(t *testing.T) {
	previousAzure, previousLogDir := config.Azure, config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = t.TempDir()
	t.Cleanup(func() { config.Azure, config.Config.E2ELoggingDir = previousAzure, previousLogDir })
	for _, tt := range []struct {
		name          string
		vmssDrift     bool
		instanceDrift bool
		readFails     bool
	}{
		{name: "matching"},
		{name: "VMSS changed", vmssDrift: true},
		{name: "instance differs from VMSS", instanceDrift: true},
		{name: "read fails", readFails: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			vmss := &armcompute.VirtualMachineScaleSet{Location: to.Ptr(cvmLocation)}
			configureConfidentialVM(vmss, cvmSNPSKU, armcompute.SecurityEncryptionTypesVMGuestStateOnly)
			vmss.Properties.VirtualMachineProfile.StorageProfile.ImageReference = &armcompute.ImageReference{ID: to.Ptr(cvmTestImageID)}
			s := cvmModelTestScenario("fresh-model", vmss)
			s.Runtime.VMSSName = "vmss"
			s.Runtime.Cluster = &Cluster{Model: &armcontainerservice.ManagedCluster{
				Properties: &armcontainerservice.ManagedClusterProperties{NodeResourceGroup: to.Ptr("rg")},
			}}
			vmssBody, err := json.Marshal(vmss)
			require.NoError(t, err)
			vmBody, err := json.Marshal(s.Runtime.VM.VM)
			require.NoError(t, err)
			var paths []string
			respond := vmssCreationTestPolicy(func(req *http.Request) *http.Response {
				assert.Equal(t, http.MethodGet, req.Method)
				paths = append(paths, req.URL.Path)
				body := string(vmssBody)
				drift := tt.vmssDrift
				if strings.HasSuffix(req.URL.Path, "/virtualMachines/0") {
					body = string(vmBody)
					drift = tt.instanceDrift
				}
				if drift {
					body = strings.ReplaceAll(body, cvmTestImageID, cvmTestImageID+"4")
				}
				status := http.StatusOK
				if tt.readFails {
					status, body = http.StatusForbidden, `{"error":{"code":"AuthorizationFailed","message":"read denied"}}`
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}
			})
			options := &arm.ClientOptions{ClientOptions: policy.ClientOptions{PerCallPolicies: []policy.Policy{respond}}}
			config.Azure = &config.AzureClient{}
			config.Azure.VMSS, err = armcompute.NewVirtualMachineScaleSetsClient("test", nil, options)
			require.NoError(t, err)
			config.Azure.VMSSVM, err = armcompute.NewVirtualMachineScaleSetVMsClient("test", nil, options)
			require.NoError(t, err)
			err = refreshCVMModel(t.Context(), s)
			if tt.readFails {
				require.ErrorContains(t, err, "read fresh CVM VMSS model")
				return
			}
			require.NoError(t, err)
			require.Len(t, paths, 2)
			assert.Equal(t, "/subscriptions/test/resourceGroups/rg/providers/Microsoft.Compute/virtualMachineScaleSets/vmss", paths[0])
			assert.Equal(t, paths[0]+"/virtualMachines/0", paths[1])
			_, err = validateObservedCVMModel(s, cvmPlatformSEVSNP, armcompute.SecurityEncryptionTypesVMGuestStateOnly)
			switch {
			case tt.vmssDrift:
				require.ErrorContains(t, err, "vmss.imageReference")
			case tt.instanceDrift:
				require.ErrorContains(t, err, "vm.imageReference")
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestParseCVMPCRs(t *testing.T) {
	var valid strings.Builder
	valid.WriteString("sha256:\n")
	for index := range 24 {
		fmt.Fprintf(&valid, "  %d : 0x%s\n", index, strings.Repeat("AB", 32))
	}
	output := valid.String()
	for _, tt := range []struct {
		name   string
		output string
		valid  bool
		digest string
	}{
		{name: "valid bank", output: output, valid: true, digest: strings.Repeat("ab", 32)},
		{name: "unextended PCRs", output: strings.ReplaceAll(output, strings.Repeat("AB", 32), strings.Repeat("00", 32)), valid: true, digest: strings.Repeat("00", 32)},
		{name: "header only", output: "sha256:\n"},
		{name: "unrelated text", output: "cannot read sha256 PCRs"},
		{name: "wrong bank", output: strings.Replace(output, "sha256:", "sha1:", 1)},
		{name: "short digest", output: strings.Replace(output, strings.Repeat("AB", 32), "AB", 1)},
		{name: "non hexadecimal", output: strings.Replace(output, "0xAB", "0xZZ", 1)},
		{name: "missing register", output: strings.Replace(output, "  0 :", "  24 :", 1)},
		{name: "duplicate register", output: strings.Replace(output, "  1 :", "  0 :", 1)},
		{name: "multiple documents", output: output + "---\n" + output},
		{name: "stderr is not PCR evidence", output: "stdout:\n\nstderr:\n" + output},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pcrs, err := parseCVMPCRs(tt.output)
			if !tt.valid {
				require.Error(t, err)
				assert.Nil(t, pcrs)
				return
			}
			require.NoError(t, err)
			require.Len(t, pcrs, 24)
			assert.Equal(t, tt.digest, pcrs[7])
		})
	}
}

func TestCVMAttestationVerdictsUsesOneSummaryRecord(t *testing.T) {
	verdicts := cvmAttestationVerdicts(&cvmAttestationSummary{
		Platform:         cvmPlatformTDX,
		AttestationType:  "tdxvm",
		ComplianceStatus: "azure-compliant-cvm",
		SecureBoot:       true,
		VTPM:             true,
	})

	require.Len(t, verdicts, 1)
	for _, name := range []string{"maaCompliance", "attestationType", "secureBoot", "vTPM"} {
		verdict, ok := verdicts[0][name]
		require.True(t, ok, name)
		require.NotNil(t, verdict.Passed, name)
		assert.True(t, *verdict.Passed, name)
		assert.Equal(t, cvmStatusPassed, verdict.Status, name)
	}
	assert.Equal(t, cvmStatusPassed, verdicts[0]["attestation"].Status)
}

func TestWriteFailedCVMAttestationSummary(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	verifiedSummary := &cvmAttestationSummary{
		Platform: cvmPlatformTDX, AttestationType: "tdxvm",
		ComplianceStatus: "azure-compliant-cvm", SecureBoot: true, VTPM: true,
	}
	tests := []struct {
		name            string
		err             error
		verifiedSummary *cvmAttestationSummary
		status          string
		failedCheck     string
	}{
		{name: "collector-error", err: assert.AnError, status: cvmStatusUnableToVerify},
		{name: "timeout", err: fmt.Errorf("collector: %w", context.DeadlineExceeded), status: cvmStatusUnableToVerify},
		{name: "invalid-token", err: &cvmAttestationError{err: errors.New("invalid signature")}, status: cvmStatusFailed},
		{
			name: "secure-boot-disabled",
			err: fmt.Errorf("verify: %w", errors.Join(assert.AnError, &cvmAttestationError{
				err: errors.New("Secure Boot disabled"), check: "secureBoot", observed: false,
			})),
			status: cvmStatusFailed, failedCheck: "secureBoot",
		},
		{
			name: "cleanup-error", err: assert.AnError,
			verifiedSummary: verifiedSummary, status: cvmStatusUnableToVerify,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scenario := &Scenario{artifactName: test.name}
			require.NoError(t, writeFailedCVMAttestationSummary(scenario, test.verifiedSummary, test.err))
			content, err := os.ReadFile(filepath.Join(loggingDir, test.name, "cvm-attestation-summary.json"))
			require.NoError(t, err)
			var records []map[string]map[string]any
			require.NoError(t, json.Unmarshal(content, &records))
			require.Len(t, records, 1)
			record := records[0]
			assert.Equal(t, test.status, record["attestation"]["status"])
			assert.Equal(t, false, record["attestation"]["passed"])
			assert.Equal(t, test.err.Error(), record["attestation"]["reason"])
			for _, name := range []string{"maaCompliance", "attestationType", "secureBoot", "vTPM"} {
				switch {
				case name == test.failedCheck:
					assert.Equal(t, cvmStatusFailed, record[name]["status"])
					assert.Equal(t, false, record[name]["passed"])
					assert.Equal(t, false, record[name]["observed"])
				case test.verifiedSummary != nil:
					assert.Equal(t, cvmStatusPassed, record[name]["status"])
					assert.Equal(t, true, record[name]["passed"])
				default:
					assert.Equal(t, cvmStatusUnableToVerify, record[name]["status"])
					assert.Nil(t, record[name]["passed"])
					assert.Nil(t, record[name]["observed"])
				}
			}
		})
	}
}
