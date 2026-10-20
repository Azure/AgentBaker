package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
)

const cvmEvidenceLimit = 128 * 1024

const (
	cvmStatusPassed         = "passed"
	cvmStatusFailed         = "failed"
	cvmStatusUnableToVerify = "unable_to_verify"
)

type cvmCheck struct {
	Name     string         `json:"name"`
	Passed   bool           `json:"passed"`
	Status   string         `json:"status"`
	Expected any            `json:"expected,omitempty"`
	Observed any            `json:"observed,omitempty"`
	Evidence map[string]any `json:"evidence,omitempty"`
}

type observedCVMModel struct {
	Location               string                             `json:"location"`
	SKU                    string                             `json:"sku"`
	ImageReference         *armcompute.ImageReference         `json:"imageReference,omitempty"`
	SecurityType           armcompute.SecurityTypes           `json:"securityType"`
	SecureBootEnabled      bool                               `json:"secureBootEnabled"`
	VTPMEnabled            bool                               `json:"vTPMEnabled"`
	EncryptionAtHost       bool                               `json:"encryptionAtHost"`
	SecurityEncryptionType armcompute.SecurityEncryptionTypes `json:"securityEncryptionType"`
	DiskEncryptionSetID    string                             `json:"diskEncryptionSetId,omitempty"`
}

func validateCVM(platform cvmPlatform, securityEncryptionType armcompute.SecurityEncryptionTypes) func(context.Context, *Scenario) error {
	return func(ctx context.Context, s *Scenario) error {
		var checks []cvmCheck
		var validationErrors []error

		modelChecks, err := validateObservedCVMModel(s, platform, securityEncryptionType)
		checks = append(checks, modelChecks...)
		if err != nil {
			validationErrors = append(validationErrors, err)
		}

		attestationSummary, attestationErr := validateCVMAttestation(ctx, s, platform)
		attestationCheck := newCVMCheck(
			"attestation.verified",
			"valid MAA signature, freshness, nonce, compliance, platform, and policy claims",
			cvmAttestationObserved(attestationErr),
			attestationErr == nil,
			"cvm/attestation-claims.json",
			"",
		)
		attestationCheck.Status = cvmAttestationStatus(attestationErr)
		if attestationErr != nil {
			attestationCheck.Evidence = map[string]any{"path": "cvm-attestation-summary.json", "claim": "/0/attestation"}
		}
		checks = append(checks, attestationCheck)
		if attestationErr != nil {
			validationErrors = append(validationErrors, fmt.Errorf("CVM attestation %s: %w", attestationCheck.Status, attestationErr))
			if err := writeFailedCVMAttestationSummary(s, attestationSummary, attestationErr); err != nil {
				validationErrors = append(validationErrors, err)
			}
		}

		guestChecks, err := validateCVMGuestEvidence(ctx, s, platform)
		checks = append(checks, guestChecks...)
		if err != nil {
			validationErrors = append(validationErrors, err)
		}

		checksJSON, err := json.MarshalIndent(checks, "", "  ")
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("serialize CVM checks: %w", err))
		} else if err := writeToFile(s.artifactName, "cvm/cvm-checks.json", string(checksJSON)+"\n"); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("write CVM checks: %w", err))
		}

		return errors.Join(validationErrors...)
	}
}

func cvmAttestationObserved(err error) string {
	if err == nil {
		return "verified"
	}
	return err.Error()
}

func writeFailedCVMAttestationSummary(s *Scenario, verifiedSummary *cvmAttestationSummary, attestationErr error) error {
	record := map[string]any{}
	if verifiedSummary != nil {
		record = cvmAttestationVerdicts(verifiedSummary)[0]
	} else {
		for _, name := range []string{"maaCompliance", "attestationType", "secureBoot", "vTPM"} {
			record[name] = map[string]any{
				"status":   cvmStatusUnableToVerify,
				"passed":   nil,
				"observed": nil,
				"reason":   "no accepted attestation result is available",
			}
		}
		var failure *cvmAttestationFailure
		if errors.As(attestationErr, &failure) && failure.check != "" {
			record[failure.check] = map[string]any{
				"status":   cvmStatusFailed,
				"passed":   false,
				"observed": failure.observed,
				"reason":   failure.Error(),
			}
		}
	}
	record["attestation"] = map[string]any{
		"status": cvmAttestationStatus(attestationErr),
		"passed": false,
		"reason": attestationErr.Error(),
	}
	summaryJSON, err := json.MarshalIndent([]map[string]any{record}, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize failed CVM attestation summary: %w", err)
	}
	if err := writeToFile(s.artifactName, "cvm-attestation-summary.json", string(summaryJSON)+"\n"); err != nil {
		return fmt.Errorf("write failed CVM attestation summary: %w", err)
	}
	return nil
}

func validateObservedCVMModel(s *Scenario, platform cvmPlatform, securityEncryptionType armcompute.SecurityEncryptionTypes) ([]cvmCheck, error) {
	const evidencePath = "cvm/vmss-model.json"
	if s == nil || s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.VMSS == nil {
		return nil, errors.New("CVM validation requires the observed VMSS model")
	}
	vmss := s.Runtime.VM.VMSS
	model := observedCVMModel{Location: stringValue(vmss.Location)}
	if vmss.SKU != nil {
		model.SKU = stringValue(vmss.SKU.Name)
	}
	if vmss.Properties != nil && vmss.Properties.VirtualMachineProfile != nil {
		profile := vmss.Properties.VirtualMachineProfile
		if profile.SecurityProfile != nil {
			model.SecurityType = enumValue(profile.SecurityProfile.SecurityType)
			model.EncryptionAtHost = boolValue(profile.SecurityProfile.EncryptionAtHost)
			if profile.SecurityProfile.UefiSettings != nil {
				model.SecureBootEnabled = boolValue(profile.SecurityProfile.UefiSettings.SecureBootEnabled)
				model.VTPMEnabled = boolValue(profile.SecurityProfile.UefiSettings.VTpmEnabled)
			}
		}
		if profile.StorageProfile != nil {
			model.ImageReference = profile.StorageProfile.ImageReference
			if profile.StorageProfile.OSDisk != nil && profile.StorageProfile.OSDisk.ManagedDisk != nil {
				managedDisk := profile.StorageProfile.OSDisk.ManagedDisk
				if managedDisk.SecurityProfile != nil {
					model.SecurityEncryptionType = enumValue(managedDisk.SecurityProfile.SecurityEncryptionType)
				}
				if managedDisk.DiskEncryptionSet != nil {
					model.DiskEncryptionSetID = stringValue(managedDisk.DiskEncryptionSet.ID)
				}
			}
		}
	}

	modelJSON, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize observed CVM VMSS model: %w", err)
	}
	if err := writeToFile(s.artifactName, evidencePath, string(modelJSON)+"\n"); err != nil {
		return nil, fmt.Errorf("write observed CVM VMSS model: %w", err)
	}
	diskJSON, err := json.MarshalIndent(map[string]any{
		"securityEncryptionType": model.SecurityEncryptionType,
		"diskEncryptionSetId":    model.DiskEncryptionSetID,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize CVM disk security evidence: %w", err)
	}
	if err := writeToFile(s.artifactName, "cvm/disk-security.json", string(diskJSON)+"\n"); err != nil {
		return nil, fmt.Errorf("write CVM disk security evidence: %w", err)
	}
	encryptionJSON, err := json.MarshalIndent(map[string]any{"encryptionAtHost": model.EncryptionAtHost}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize Encryption at Host evidence: %w", err)
	}
	if err := writeToFile(s.artifactName, "cvm/encryption-at-host.json", string(encryptionJSON)+"\n"); err != nil {
		return nil, fmt.Errorf("write Encryption at Host evidence: %w", err)
	}

	expectedSKU := cvmSNPSKU
	if platform == cvmPlatformTDX {
		expectedSKU = cvmTDXSKU
	}
	checks := []cvmCheck{
		newCVMCheck("vmss.location", cvmLocation, model.Location, strings.EqualFold(model.Location, cvmLocation), evidencePath, "/location"),
		newCVMCheck("vmss.sku", expectedSKU, model.SKU, model.SKU == expectedSKU, evidencePath, "/sku"),
		newCVMCheck("vmss.imageReference", "non-empty", model.ImageReference, model.ImageReference != nil, evidencePath, "/imageReference"),
		newCVMCheck("vmss.securityType", armcompute.SecurityTypesConfidentialVM, model.SecurityType, model.SecurityType == armcompute.SecurityTypesConfidentialVM, evidencePath, "/securityType"),
		newCVMCheck("vmss.secureBoot", true, model.SecureBootEnabled, model.SecureBootEnabled, evidencePath, "/secureBootEnabled"),
		newCVMCheck("vmss.vTPM", true, model.VTPMEnabled, model.VTPMEnabled, evidencePath, "/vTPMEnabled"),
		newCVMCheck("vmss.encryptionAtHost", true, model.EncryptionAtHost, model.EncryptionAtHost, evidencePath, "/encryptionAtHost"),
		newCVMCheck("vmss.osDiskSecurity", securityEncryptionType, model.SecurityEncryptionType, model.SecurityEncryptionType == securityEncryptionType, "cvm/disk-security.json", "/securityEncryptionType"),
	}
	return checks, failedCVMChecks(checks)
}

func validateCVMGuestEvidence(ctx context.Context, s *Scenario, platform cvmPlatform) ([]cvmCheck, error) {
	var checks []cvmCheck
	var validationErrors []error

	platformOutput, platformErr := collectCVMEvidence(ctx, s, "cvm/platform-evidence.log", `
set -u
printf '%s\n' 'systemd-detect-virt --cvm:'
systemd-detect-virt --cvm 2>&1 || true
printf '%s\n' 'kernel CVM messages:'
sudo dmesg 2>/dev/null | grep -Ei -m 80 'sev|snp|tdx|confidential' || true
`)
	expectedMarker := "sev"
	if platform == cvmPlatformTDX {
		expectedMarker = "tdx"
	}
	platformPassed := platformErr == nil && strings.Contains(strings.ToLower(platformOutput), expectedMarker)
	checks = append(checks, newCVMCheck("guest.platform", expectedMarker, strings.TrimSpace(platformOutput), platformPassed, "cvm/platform-evidence.log", ""))
	if platformErr != nil {
		validationErrors = append(validationErrors, platformErr)
	}

	pcrOutput, pcrErr := collectCVMEvidence(ctx, s, "cvm/measured-boot-pcrs.log", `
set -u
command -v tpm2_pcrread
sudo tpm2_pcrread sha256
`)
	pcrPassed := pcrErr == nil && strings.Contains(pcrOutput, "sha256")
	checks = append(checks, newCVMCheck("guest.measuredBootPCRs", "readable SHA-256 PCR bank", strings.TrimSpace(pcrOutput), pcrPassed, "cvm/measured-boot-pcrs.log", ""))
	if pcrErr != nil {
		validationErrors = append(validationErrors, pcrErr)
	}

	if err := failedCVMChecks(checks); err != nil {
		validationErrors = append(validationErrors, err)
	}
	return checks, errors.Join(validationErrors...)
}

func collectCVMEvidence(ctx context.Context, s *Scenario, artifactPath, script string) (string, error) {
	result, err := execScriptOnVMForScenario(ctx, s, script)
	if err != nil {
		return "", fmt.Errorf("collect %s: %w", artifactPath, err)
	}
	output := "stdout:\n" + result.stdout + "\nstderr:\n" + result.stderr
	if len(output) > cvmEvidenceLimit {
		output = output[:cvmEvidenceLimit] + "\n[truncated]\n"
	}
	if writeErr := writeToFile(s.artifactName, artifactPath, output); writeErr != nil {
		return output, fmt.Errorf("write %s: %w", artifactPath, writeErr)
	}
	if result.exitCode != "0" {
		return output, fmt.Errorf("collect %s: command exited with code %s", artifactPath, result.exitCode)
	}
	return output, nil
}

func newCVMCheck(name string, expected, observed any, passed bool, path, claim string) cvmCheck {
	evidence := map[string]any{"path": path}
	if claim != "" {
		evidence["claim"] = claim
	}
	status := cvmStatusFailed
	if passed {
		status = cvmStatusPassed
	}
	return cvmCheck{Name: name, Passed: passed, Status: status, Expected: expected, Observed: observed, Evidence: evidence}
}

func failedCVMChecks(checks []cvmCheck) error {
	var failures []string
	for _, check := range checks {
		if !check.Passed {
			failures = append(failures, check.Name)
		}
	}
	if len(failures) == 0 {
		return nil
	}
	return fmt.Errorf("CVM validation checks failed: %s", strings.Join(failures, ", "))
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func enumValue[T ~string](value *T) T {
	if value == nil {
		var zero T
		return zero
	}
	return *value
}
