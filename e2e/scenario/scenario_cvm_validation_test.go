package scenario

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
					ImageReference: &armcompute.ImageReference{ID: to.Ptr("/subscriptions/test/images/cvm")},
				},
			},
		},
	}
	configureConfidentialVM(vmss, cvmSNPSKU, armcompute.SecurityEncryptionTypesVMGuestStateOnly)
	scenario := &Scenario{
		artifactName: "cvm-model",
		Runtime:      &ScenarioRuntime{VM: &ScenarioVM{VMSS: vmss}},
	}

	checks, err := validateObservedCVMModel(scenario, cvmPlatformSEVSNP)
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

	checks, err := validateObservedCVMModel(scenario, cvmPlatformTDX)
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
		verdict, ok := verdicts[0][name].(map[string]any)
		require.True(t, ok, name)
		assert.Equal(t, true, verdict["passed"], name)
	}
}

func TestWriteFailedCVMAttestationSummary(t *testing.T) {
	loggingDir := t.TempDir()
	original := config.Config.E2ELoggingDir
	config.Config.E2ELoggingDir = loggingDir
	t.Cleanup(func() { config.Config.E2ELoggingDir = original })

	scenario := &Scenario{artifactName: "failed-attestation"}
	require.NoError(t, writeFailedCVMAttestationSummary(scenario, assert.AnError))

	content, err := os.ReadFile(filepath.Join(loggingDir, "failed-attestation", "cvm-attestation-summary.json"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `"passed": false`)
	assert.Contains(t, string(content), assert.AnError.Error())
}

func TestIsPHCOnlyChrony(t *testing.T) {
	assert.True(t, isPHCOnlyChrony("refclock PHC 0 poll 3"))
	assert.False(t, isPHCOnlyChrony("refclock PHC 0 poll 3\npool ntp.ubuntu.com iburst"))
	assert.False(t, isPHCOnlyChrony("pool ntp.ubuntu.com iburst"))
}
