package scenario

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v8"
	"gopkg.in/yaml.v3"
)

const cvmEvidenceLimit = 128 * 1024

const (
	cvmStatusPassed         = "passed"
	cvmStatusFailed         = "failed"
	cvmStatusUnableToVerify = "unable_to_verify"
	cvmCheckCompliance      = "maaCompliance"
	cvmCheckAttestationType = "attestationType"
	cvmCheckSecureBoot      = "secureBoot"
	cvmCheckVTPM            = "vTPM"
)

type cvmEvidence struct {
	Path  string `json:"path"`
	Claim string `json:"claim,omitempty"`
}

type cvmAttestationVerdict struct {
	Status   string       `json:"status"`
	Passed   *bool        `json:"passed"`
	Observed any          `json:"observed,omitempty"`
	Reason   string       `json:"reason,omitempty"`
	Evidence *cvmEvidence `json:"evidence,omitempty"`
}

type cvmCheck struct {
	Name     string      `json:"name"`
	Passed   bool        `json:"passed"`
	Status   string      `json:"status"`
	Expected any         `json:"expected,omitempty"`
	Observed any         `json:"observed,omitempty"`
	Evidence cvmEvidence `json:"evidence"`
}

type observedCVMModel struct {
	Location               string                             `json:"location"`
	SKU                    string                             `json:"sku"`
	ImageReference         *armcompute.ImageReference         `json:"imageReference,omitempty"`
	ExpectedImageID        string                             `json:"expectedImageId"`
	InstanceImageReference *armcompute.ImageReference         `json:"instanceImageReference,omitempty"`
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

		err := refreshCVMModel(ctx, s)
		if err == nil {
			var modelChecks []cvmCheck
			modelChecks, err = validateObservedCVMModel(s, platform, securityEncryptionType)
			checks = append(checks, modelChecks...)
		} else {
			check := newCVMCheck("vmss.observedModel", "fresh VMSS and VM-instance reads", err.Error(), false, "cvm/cvm-checks.json", "")
			check.Status = cvmStatusUnableToVerify
			checks = append(checks, check)
		}
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
			attestationCheck.Evidence = cvmEvidence{Path: "cvm-attestation-summary.json", Claim: "/0/attestation"}
		}
		checks = append(checks, attestationCheck)
		if attestationErr != nil {
			validationErrors = append(validationErrors, fmt.Errorf("CVM attestation %s: %w", attestationCheck.Status, attestationErr))
			if summaryErr := writeFailedCVMAttestationSummary(s, attestationSummary, attestationErr); summaryErr != nil {
				validationErrors = append(validationErrors, summaryErr)
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

func refreshCVMModel(ctx context.Context, s *Scenario) error {
	if s == nil || s.Runtime == nil || s.Runtime.VM == nil || s.Runtime.VM.VM == nil ||
		s.Runtime.VM.VM.InstanceID == nil || s.Runtime.Cluster == nil || s.Runtime.Cluster.Model == nil ||
		s.Runtime.Cluster.Model.Properties == nil || s.Runtime.Cluster.Model.Properties.NodeResourceGroup == nil {
		return errors.New("refresh CVM model requires a VM instance and node resource group")
	}
	if config.Azure == nil || config.Azure.VMSS == nil || config.Azure.VMSSVM == nil {
		return errors.New("refresh CVM model requires Azure compute clients")
	}
	readCtx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	resourceGroup := *s.Runtime.Cluster.Model.Properties.NodeResourceGroup
	vmss, err := config.Azure.VMSS.Get(readCtx, resourceGroup, s.Runtime.VMSSName, nil)
	if err != nil {
		return fmt.Errorf("read fresh CVM VMSS model: %w", err)
	}
	vm, err := config.Azure.VMSSVM.Get(readCtx, resourceGroup, s.Runtime.VMSSName, *s.Runtime.VM.VM.InstanceID, nil)
	if err != nil {
		return fmt.Errorf("read fresh CVM VM instance: %w", err)
	}
	s.Runtime.VM.VMSS = &vmss.VirtualMachineScaleSet
	s.Runtime.VM.VM = &vm.VirtualMachineScaleSetVM
	return nil
}

func cvmAttestationObserved(err error) string {
	if err == nil {
		return "verified"
	}
	return err.Error()
}

func writeFailedCVMAttestationSummary(s *Scenario, verifiedSummary *cvmAttestationSummary, attestationErr error) error {
	record := map[string]cvmAttestationVerdict{}
	if verifiedSummary != nil {
		record = cvmAttestationVerdicts(verifiedSummary)[0]
	} else {
		for _, name := range []string{cvmCheckCompliance, cvmCheckAttestationType, cvmCheckSecureBoot, cvmCheckVTPM} {
			record[name] = cvmAttestationVerdict{
				Status: cvmStatusUnableToVerify,
				Reason: "no accepted attestation result is available",
			}
		}
		var failure *cvmAttestationError
		if errors.As(attestationErr, &failure) && failure.check != "" {
			record[failure.check] = cvmAttestationVerdict{
				Status:   cvmStatusFailed,
				Passed:   new(false),
				Observed: failure.observed,
				Reason:   failure.Error(),
			}
		}
	}
	record["attestation"] = cvmAttestationVerdict{
		Status: cvmAttestationStatus(attestationErr),
		Passed: new(false),
		Reason: attestationErr.Error(),
	}
	summaryJSON, err := json.MarshalIndent([]map[string]cvmAttestationVerdict{record}, "", "  ")
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
	model := readObservedCVMModel(s.Runtime.VM.VMSS)
	model.ExpectedImageID = string(s.Runtime.ResolvedVHD)
	if vm := s.Runtime.VM.VM; vm != nil && vm.Properties != nil && vm.Properties.StorageProfile != nil {
		model.InstanceImageReference = vm.Properties.StorageProfile.ImageReference
	}

	modelJSON, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize observed CVM VMSS model: %w", err)
	}
	if err = writeToFile(s.artifactName, evidencePath, string(modelJSON)+"\n"); err != nil {
		return nil, fmt.Errorf("write observed CVM VMSS model: %w", err)
	}
	diskJSON, err := json.MarshalIndent(map[string]any{
		"securityEncryptionType": model.SecurityEncryptionType,
		"diskEncryptionSetId":    model.DiskEncryptionSetID,
	}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize CVM disk security evidence: %w", err)
	}
	if err = writeToFile(s.artifactName, "cvm/disk-security.json", string(diskJSON)+"\n"); err != nil {
		return nil, fmt.Errorf("write CVM disk security evidence: %w", err)
	}
	encryptionJSON, err := json.MarshalIndent(map[string]any{"encryptionAtHost": model.EncryptionAtHost}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serialize Encryption at Host evidence: %w", err)
	}
	if err = writeToFile(s.artifactName, "cvm/encryption-at-host.json", string(encryptionJSON)+"\n"); err != nil {
		return nil, fmt.Errorf("write Encryption at Host evidence: %w", err)
	}

	expectedSKU := cvmSNPSKU
	if platform == cvmPlatformTDX {
		expectedSKU = cvmTDXSKU
	}
	checks := []cvmCheck{
		newCVMCheck("vmss.location", cvmLocation, model.Location, strings.EqualFold(model.Location, cvmLocation), evidencePath, "/location"),
		newCVMCheck("vmss.sku", expectedSKU, model.SKU, model.SKU == expectedSKU, evidencePath, "/sku"),
		newCVMCheck("vmss.imageReference", model.ExpectedImageID, model.ImageReference,
			cvmImageMatches(model.ExpectedImageID, model.ImageReference), evidencePath, "/imageReference"),
		newCVMCheck("vm.imageReference", model.ExpectedImageID, model.InstanceImageReference,
			cvmImageMatches(model.ExpectedImageID, model.InstanceImageReference), evidencePath, "/instanceImageReference"),
		newCVMCheck(
			"vmss.securityType", armcompute.SecurityTypesConfidentialVM, model.SecurityType,
			model.SecurityType == armcompute.SecurityTypesConfidentialVM, evidencePath, "/securityType",
		),
		newCVMCheck("vmss.secureBoot", true, model.SecureBootEnabled, model.SecureBootEnabled, evidencePath, "/secureBootEnabled"),
		newCVMCheck("vmss.vTPM", true, model.VTPMEnabled, model.VTPMEnabled, evidencePath, "/vTPMEnabled"),
		newCVMCheck("vmss.encryptionAtHost", true, model.EncryptionAtHost, model.EncryptionAtHost, evidencePath, "/encryptionAtHost"),
		newCVMCheck(
			"vmss.osDiskSecurity", securityEncryptionType, model.SecurityEncryptionType,
			model.SecurityEncryptionType == securityEncryptionType, "cvm/disk-security.json", "/securityEncryptionType",
		),
	}
	return checks, failedCVMChecks(checks)
}

func cvmImageMatches(expected string, observed *armcompute.ImageReference) bool {
	return expected != "" && !strings.HasSuffix(strings.ToLower(expected), "/versions/latest") &&
		observed != nil && strings.EqualFold(expected, stringValue(observed.ID))
}

func readObservedCVMModel(vmss *armcompute.VirtualMachineScaleSet) observedCVMModel {
	model := observedCVMModel{Location: stringValue(vmss.Location)}
	if vmss.SKU != nil {
		model.SKU = stringValue(vmss.SKU.Name)
	}
	if vmss.Properties == nil || vmss.Properties.VirtualMachineProfile == nil {
		return model
	}
	profile := vmss.Properties.VirtualMachineProfile
	if profile.SecurityProfile != nil {
		model.SecurityType = enumValue(profile.SecurityProfile.SecurityType)
		model.EncryptionAtHost = boolValue(profile.SecurityProfile.EncryptionAtHost)
		if profile.SecurityProfile.UefiSettings != nil {
			model.SecureBootEnabled = boolValue(profile.SecurityProfile.UefiSettings.SecureBootEnabled)
			model.VTPMEnabled = boolValue(profile.SecurityProfile.UefiSettings.VTpmEnabled)
		}
	}
	if profile.StorageProfile == nil {
		return model
	}
	model.ImageReference = profile.StorageProfile.ImageReference
	if profile.StorageProfile.OSDisk == nil || profile.StorageProfile.OSDisk.ManagedDisk == nil {
		return model
	}
	managedDisk := profile.StorageProfile.OSDisk.ManagedDisk
	if managedDisk.SecurityProfile != nil {
		model.SecurityEncryptionType = enumValue(managedDisk.SecurityProfile.SecurityEncryptionType)
	}
	if managedDisk.DiskEncryptionSet != nil {
		model.DiskEncryptionSetID = stringValue(managedDisk.DiskEncryptionSet.ID)
	}
	return model
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
set -euo pipefail
command -v tpm2_pcrread >/dev/null
sudo tpm2_pcrread sha256
`)
	pcrs, parseErr := parseCVMPCRs(pcrOutput)
	pcrEvidence := struct {
		Bank  string         `json:"bank"`
		PCRs  map[int]string `json:"pcrs"`
		Scope string         `json:"scope"`
	}{
		Bank: "sha256", PCRs: pcrs,
		Scope: "readability and format only; not a TPM quote or approved measured-boot baseline",
	}
	if parseErr != nil {
		validationErrors = append(validationErrors, fmt.Errorf("parse SHA-256 PCR evidence: %w", parseErr))
	} else {
		pcrJSON, err := json.MarshalIndent(pcrEvidence, "", "  ")
		if err == nil {
			err = writeToFile(s.artifactName, "cvm/measured-boot-pcrs.json", string(pcrJSON)+"\n")
		}
		if err != nil {
			pcrErr = errors.Join(pcrErr, fmt.Errorf("write structured PCR evidence: %w", err))
		}
	}
	checks = append(checks, newCVMCheck("guest.measuredBootPCRs",
		"24 readable SHA-256 PCRs (0-23), each 32 bytes; no integrity baseline comparison",
		pcrEvidence, pcrErr == nil && parseErr == nil, "cvm/measured-boot-pcrs.log", ""))
	if pcrErr != nil {
		validationErrors = append(validationErrors, pcrErr)
	}

	if err := failedCVMChecks(checks); err != nil {
		validationErrors = append(validationErrors, err)
	}
	return checks, errors.Join(validationErrors...)
}

func parseCVMPCRs(output string) (map[int]string, error) {
	var bank struct {
		SHA256 map[int]string `yaml:"sha256"`
	}
	decoder := yaml.NewDecoder(strings.NewReader(output))
	decoder.KnownFields(true)
	if err := decoder.Decode(&bank); err != nil {
		return nil, fmt.Errorf("decode PCR bank: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("PCR output must contain exactly one SHA-256 bank document")
	}
	if len(bank.SHA256) != 24 {
		return nil, fmt.Errorf("expected PCRs 0-23, got %d registers", len(bank.SHA256))
	}
	for index := range 24 {
		value, ok := bank.SHA256[index]
		if !ok {
			return nil, fmt.Errorf("missing PCR %d", index)
		}
		digest := strings.TrimPrefix(value, "0x")
		if len(digest) != 64 {
			return nil, fmt.Errorf("PCR %d must contain a 32-byte SHA-256 digest", index)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return nil, fmt.Errorf("PCR %d contains a non-hexadecimal digest: %w", index, err)
		}
		bank.SHA256[index] = strings.ToLower(digest)
	}
	return bank.SHA256, nil
}

func collectCVMEvidence(ctx context.Context, s *Scenario, artifactPath, script string) (string, error) {
	result, err := execScriptOnVMForScenario(ctx, s, script)
	var stdout, output string
	if result != nil {
		stdout = result.stdout
		output = result.String()
		if err == nil && result.exitCode != "0" {
			err = fmt.Errorf("command exited with code %s", result.exitCode)
		}
	} else if err == nil {
		err = errors.New("remote command returned no result")
	}
	if err != nil {
		output += "\ncollection error: " + err.Error() + "\n"
	}
	err = errors.Join(err, writeCVMDiagnosticArtifact(s, artifactPath, output))
	if err != nil {
		return stdout, fmt.Errorf("collect %s: %w", artifactPath, err)
	}
	return stdout, nil
}

func writeCVMDiagnosticArtifact(s *Scenario, artifactPath, output string) error {
	output = redactCVMDiagnostics(output)
	if len(output) > cvmEvidenceLimit {
		half := cvmEvidenceLimit / 2
		output = output[:half] + "\n[truncated]\n" + output[len(output)-half:]
	}
	if err := writeToFile(s.artifactName, artifactPath, output); err != nil {
		return fmt.Errorf("write %s: %w", artifactPath, err)
	}
	return nil
}

func newCVMCheck(name string, expected, observed any, passed bool, path, claim string) cvmCheck {
	evidence := cvmEvidence{Path: path, Claim: claim}
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
