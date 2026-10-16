package scenario

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	cvmAttestationToolsCommit = "26755c86f5034605d41053bfe0e1a7f8ae68b23b"
	cvmAttestationIssuer      = "https://sharedwus.wus.attest.azure.net"
	cvmAttestationTokenMax    = 128 * 1024
)

var cvmAttestationHTTPClient = &http.Client{Timeout: 30 * time.Second}

type cvmAttestationSummary struct {
	Platform          cvmPlatform    `json:"platform"`
	Issuer            string         `json:"issuer"`
	AttestationType   string         `json:"attestationType"`
	ComplianceStatus  string         `json:"complianceStatus"`
	SecureBoot        bool           `json:"secureBoot"`
	VTPM              bool           `json:"vTPM"`
	IssuedAt          any            `json:"issuedAt"`
	NotBefore         any            `json:"notBefore"`
	ExpiresAt         any            `json:"expiresAt"`
	UserDataDigest    string         `json:"userDataDigest"`
	AttesterTCBStatus string         `json:"attesterTcbStatus,omitempty"`
	DebugStatus       any            `json:"debugStatus,omitempty"`
	PolicyClaims      map[string]any `json:"policyClaims"`
}

func validateCVMAttestation(platform cvmPlatform) validator {
	return func(ctx context.Context, s *Scenario) (retErr error) {
		if platform != cvmPlatformSEVSNP && platform != cvmPlatformTDX {
			return fmt.Errorf("unsupported CVM platform %q", platform)
		}

		nonce, err := randomHex(32)
		if err != nil {
			return fmt.Errorf("generate attestation nonce: %w", err)
		}
		remoteSuffix, err := randomHex(16)
		if err != nil {
			return fmt.Errorf("generate attestation workspace name: %w", err)
		}
		remoteDir := "/tmp/agentbaker-cvm-attestation-" + remoteSuffix
		tokenPath := remoteDir + "/token.jwt"
		defer func() {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
			defer cancel()
			result, cleanupErr := execScriptOnVMForScenario(cleanupCtx, s, "sudo rm -rf -- "+remoteDir)
			if cleanupErr == nil && result == nil {
				cleanupErr = errors.New("remote cleanup returned no result")
			} else if cleanupErr == nil && result.exitCode != "0" {
				cleanupErr = fmt.Errorf("remote cleanup exited with code %s: %s", result.exitCode, result.stderr)
			}
			retErr = errors.Join(retErr, cleanupErr)
		}()

		result, err := execScriptOnVMForScenario(ctx, s, cvmAttestationCollectorScript(platform, nonce, remoteDir))
		if result != nil {
			artifactErr := writeToFile(s.artifactName, "cvm/attestation-output-redacted.log", result.String())
			retErr = errors.Join(retErr, artifactErr)
		}
		if err != nil {
			return errors.Join(retErr, fmt.Errorf("run CVM attestation collector: %w", err))
		}
		if result.exitCode != "0" {
			return errors.Join(retErr, fmt.Errorf("CVM attestation collector exited with code %s; see redacted artifact", result.exitCode))
		}

		tokenBytes, err := copyFileFromVM(ctx, s, tokenPath, cvmAttestationTokenMax)
		if err != nil {
			return errors.Join(retErr, fmt.Errorf("retrieve CVM attestation token: %w", err))
		}
		defer clear(tokenBytes)
		token := strings.TrimSpace(string(tokenBytes))
		if token == "" {
			return errors.Join(retErr, errors.New("CVM attestation collector returned an empty token"))
		}

		summary, err := verifyCVMAttestationToken(ctx, cvmAttestationHTTPClient, token, cvmAttestationIssuer, platform, nonce)
		if err != nil {
			return errors.Join(retErr, fmt.Errorf("verify CVM attestation token: %w", err))
		}
		claimsJSON, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return errors.Join(retErr, fmt.Errorf("serialize verified CVM attestation claims: %w", err))
		}
		if err := writeToFile(s.artifactName, "cvm/attestation-claims.json", string(claimsJSON)+"\n"); err != nil {
			return errors.Join(retErr, fmt.Errorf("write verified CVM attestation claims: %w", err))
		}
		summaryJSON, err := json.MarshalIndent(cvmAttestationVerdicts(summary), "", "  ")
		if err != nil {
			return errors.Join(retErr, fmt.Errorf("serialize CVM attestation summary: %w", err))
		}
		if err := writeToFile(s.artifactName, "cvm-attestation-summary.json", string(summaryJSON)+"\n"); err != nil {
			return errors.Join(retErr, fmt.Errorf("write CVM attestation summary: %w", err))
		}
		return retErr
	}
}

func randomHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func cvmAttestationCollectorScript(platform cvmPlatform, nonce, remoteDir string) string {
	endpoint := cvmAttestationIssuer + "/attest/SevSnpVm?api-version=2022-08-01"
	if platform == cvmPlatformTDX {
		endpoint = cvmAttestationIssuer + "/attest/TdxVm?api-version=2023-04-01-preview"
	}
	replacer := strings.NewReplacer(
		"@@PLATFORM@@", string(platform),
		"@@NONCE@@", nonce,
		"@@REMOTE_DIR@@", remoteDir,
		"@@ENDPOINT@@", endpoint,
		"@@COMMIT@@", cvmAttestationToolsCommit,
	)
	return replacer.Replace(cvmAttestationCollectorScriptTemplate)
}

const cvmAttestationCollectorScriptTemplate = `#!/usr/bin/env bash
set -euo pipefail
umask 077

work_dir="@@REMOTE_DIR@@"
repo_dir="${work_dir}/repo"
deps_dir="${work_dir}/deps"
token_path="${work_dir}/token.jwt"
mkdir -p "${work_dir}"

if ! command -v git >/dev/null || ! command -v python3 >/dev/null || ! python3 -m pip --version >/dev/null 2>&1 || ! command -v tpm2_pcrread >/dev/null || ! command -v timeout >/dev/null; then
  if command -v apt-get >/dev/null; then
    sudo env DEBIAN_FRONTEND=noninteractive apt-get update -qq
    sudo env DEBIAN_FRONTEND=noninteractive apt-get install -y -qq git python3 python3-pip tpm2-tools ca-certificates coreutils
  elif command -v dnf >/dev/null; then
    sudo dnf install -y -q git python3 python3-pip tpm2-tools ca-certificates coreutils
  elif command -v tdnf >/dev/null; then
    sudo tdnf install -y -q git python3 python3-pip tpm2-tools ca-certificates coreutils
  else
    echo "no supported package manager found" >&2
    exit 1
  fi
fi

git init --quiet "${repo_dir}"
git -C "${repo_dir}" remote add origin https://github.com/Azure/cvm-attestation-tools.git
git -C "${repo_dir}" fetch --quiet --depth 1 origin "@@COMMIT@@"
git -C "${repo_dir}" checkout --quiet --detach FETCH_HEAD
python3 -m pip install --quiet --disable-pip-version-check --target "${deps_dir}" \
  'requests==2.31.0; python_version < "3.10"' \
  'requests==2.34.2; python_version >= "3.10"' \
  'PyJWT==2.8.0; python_version < "3.9"' \
  'PyJWT==2.15.1; python_version >= "3.9"' \
  'cryptography==42.0.8; python_version < "3.9"' \
  'cryptography==50.0.2; python_version >= "3.9"' \
  construct==2.10.70

sudo timeout --verbose --signal=TERM --kill-after=10s 180s env PYTHONPATH="${deps_dir}:${repo_dir}/cvm-attestation" python3 - "@@PLATFORM@@" "@@NONCE@@" "${token_path}" "@@ENDPOINT@@" <<'PY'
import json
import os
import sys

from src.attestation_provider import MAAProvider
from src.encoder import Encoder
from src.imds_client import ImdsClient
from src.isolation import IsolationType
from src.report_parser import ReportParser
from src.tpm_wrapper import TssWrapper


class QuietLogger:
    def debug(self, *_args, **_kwargs):
        pass

    def info(self, *_args, **_kwargs):
        pass

    def warning(self, *_args, **_kwargs):
        pass

    def error(self, *_args, **_kwargs):
        pass


def main():
    platform, nonce, token_path, endpoint = sys.argv[1:]
    isolation_type = {
        "sevsnp": IsolationType.SEV_SNP,
        "tdx": IsolationType.TDX,
    }[platform]
    claims = {"user-claims": {"nonce": nonce}}
    logger = QuietLogger()
    tss = TssWrapper(logger)
    hcl_report = tss.get_hcl_report(claims)
    report_type = ReportParser.extract_report_type(hcl_report)
    if report_type != isolation_type:
        raise RuntimeError(f"expected {isolation_type.name} report, got {report_type.name}")

    runtime_data = ReportParser.extract_runtimes_data(hcl_report)
    hardware_report = ReportParser.extract_hw_report(hcl_report)
    encoded_report = Encoder.base64url_encode(hardware_report)
    imds = ImdsClient(logger)
    if isolation_type == IsolationType.TDX:
        encoded_evidence = imds.get_td_quote(encoded_report)
    else:
        cert_chain = imds.get_vcek_certificate()
        snp_report = json.dumps({
            "SnpReport": encoded_report,
            "VcekCertChain": Encoder.base64url_encode(cert_chain),
        }).encode("utf-8")
        encoded_evidence = Encoder.base64url_encode(snp_report)

    provider = MAAProvider(logger, isolation_type, endpoint)
    token = provider.attest_platform(encoded_evidence, Encoder.base64url_encode(runtime_data))
    if not token:
        raise RuntimeError("attestation provider returned no token")

    fd = os.open(token_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as token_file:
        token_file.write(token)
        token_file.write("\n")


try:
    main()
except Exception as exc:
    print(f"attestation collector failed: {type(exc).__name__}: {exc}", file=sys.stderr)
    sys.exit(1)
PY

sudo chown -- "$(id -u):$(id -g)" "${token_path}"
printf 'attestation collector completed for %s\n' "@@PLATFORM@@"
`

func verifyCVMAttestationToken(
	ctx context.Context,
	client *http.Client,
	encodedToken string,
	expectedIssuer string,
	platform cvmPlatform,
	nonce string,
) (*cvmAttestationSummary, error) {
	issuerURL, err := url.Parse(expectedIssuer)
	if err != nil || issuerURL.Scheme != "https" || issuerURL.Host == "" || issuerURL.Path != "" {
		return nil, fmt.Errorf("invalid expected attestation issuer %q", expectedIssuer)
	}

	unverifiedClaims := jwt.MapClaims{}
	unverifiedToken, _, err := jwt.NewParser().ParseUnverified(encodedToken, unverifiedClaims)
	if err != nil {
		return nil, fmt.Errorf("parse token header: %w", err)
	}
	if unverifiedToken.Method.Alg() != jwt.SigningMethodRS256.Alg() {
		return nil, fmt.Errorf("unexpected signing algorithm %q", unverifiedToken.Method.Alg())
	}
	keyID, ok := unverifiedToken.Header["kid"].(string)
	if !ok || keyID == "" {
		return nil, errors.New("attestation token has no signing key ID")
	}
	expectedJWKSURL := expectedIssuer + "/certs"
	if jku, ok := unverifiedToken.Header["jku"].(string); !ok || jku != expectedJWKSURL {
		return nil, fmt.Errorf("unexpected attestation signing keys URL")
	}

	publicKey, err := fetchMAASigningKey(ctx, client, expectedIssuer, expectedJWKSURL, keyID)
	if err != nil {
		return nil, err
	}
	claims := jwt.MapClaims{}
	parsedToken, err := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithIssuer(expectedIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(5*time.Minute),
	).ParseWithClaims(encodedToken, claims, func(*jwt.Token) (any, error) {
		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("validate signature and registered claims: %w", err)
	}
	if !parsedToken.Valid {
		return nil, errors.New("attestation token is invalid")
	}

	return validateCVMAttestationClaims(claims, platform, nonce)
}

func fetchMAASigningKey(
	ctx context.Context,
	client *http.Client,
	expectedIssuer string,
	expectedJWKSURL string,
	keyID string,
) (*rsa.PublicKey, error) {
	var metadata struct {
		Issuer                     string   `json:"issuer"`
		JWKSURI                    string   `json:"jwks_uri"`
		SigningAlgorithmsSupported []string `json:"id_token_signing_alg_values_supported"`
	}
	metadataURL := expectedIssuer + "/.well-known/openid-configuration?api-version=2025-06-01"
	if err := getJSON(ctx, client, metadataURL, &metadata); err != nil {
		return nil, fmt.Errorf("get MAA OpenID metadata: %w", err)
	}
	if metadata.Issuer != expectedIssuer {
		return nil, fmt.Errorf("OpenID metadata issuer %q does not match %q", metadata.Issuer, expectedIssuer)
	}
	if metadata.JWKSURI != expectedJWKSURL {
		return nil, fmt.Errorf("OpenID metadata signing keys URL %q does not match %q", metadata.JWKSURI, expectedJWKSURL)
	}
	if !slicesContains(metadata.SigningAlgorithmsSupported, jwt.SigningMethodRS256.Alg()) {
		return nil, errors.New("OpenID metadata does not advertise RS256")
	}

	var keySet struct {
		Keys []struct {
			KeyID   string   `json:"kid"`
			KeyType string   `json:"kty"`
			X5C     []string `json:"x5c"`
		} `json:"keys"`
	}
	if err := getJSON(ctx, client, expectedJWKSURL, &keySet); err != nil {
		return nil, fmt.Errorf("get MAA signing keys: %w", err)
	}
	for _, key := range keySet.Keys {
		if key.KeyID != keyID {
			continue
		}
		if key.KeyType != "RSA" || len(key.X5C) == 0 {
			return nil, fmt.Errorf("MAA signing key %q has no RSA certificate", keyID)
		}
		certificateDER, err := base64.StdEncoding.DecodeString(key.X5C[0])
		if err != nil {
			return nil, fmt.Errorf("decode MAA signing certificate: %w", err)
		}
		certificate, err := x509.ParseCertificate(certificateDER)
		if err != nil {
			return nil, fmt.Errorf("parse MAA signing certificate: %w", err)
		}
		now := time.Now()
		if now.Before(certificate.NotBefore) || now.After(certificate.NotAfter) {
			return nil, fmt.Errorf("MAA signing certificate %q is not currently valid", keyID)
		}
		publicKey, ok := certificate.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("MAA signing certificate %q does not contain an RSA key", keyID)
		}
		return publicKey, nil
	}
	return nil, fmt.Errorf("MAA signing key %q was not found", keyID)
}

func getJSON(ctx context.Context, client *http.Client, endpoint string, output any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
		return fmt.Errorf("GET %s returned %s", endpoint, response.Status)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1024*1024))
	if err := decoder.Decode(output); err != nil {
		return fmt.Errorf("decode response from %s: %w", endpoint, err)
	}
	return nil
}

func validateCVMAttestationClaims(claims jwt.MapClaims, platform cvmPlatform, nonce string) (*cvmAttestationSummary, error) {
	expectedType := "sevsnpvm"
	if platform == cvmPlatformTDX {
		expectedType = "tdxvm"
	}
	attestationType, err := requiredStringClaim(claims, "x-ms-attestation-type")
	if err != nil {
		return nil, err
	}
	if attestationType != expectedType {
		return nil, fmt.Errorf("expected attestation type %q, got %q", expectedType, attestationType)
	}
	complianceStatus, err := requiredStringClaim(claims, "x-ms-compliance-status")
	if err != nil {
		return nil, err
	}
	if complianceStatus != "azure-compliant-cvm" {
		return nil, fmt.Errorf("unexpected compliance status %q", complianceStatus)
	}

	runtimeData, ok := claims["x-ms-runtime"].(map[string]any)
	if !ok {
		return nil, errors.New("attestation token has no x-ms-runtime object")
	}
	userDataDigest, ok := runtimeData["user-data"].(string)
	if !ok || userDataDigest == "" {
		return nil, errors.New("attestation token has no x-ms-runtime.user-data digest")
	}
	expectedDigest := cvmNonceDigest(nonce)
	if !strings.EqualFold(userDataDigest, expectedDigest) {
		return nil, errors.New("attestation token user-data digest does not match the scenario nonce")
	}
	vmConfiguration, ok := runtimeData["vm-configuration"].(map[string]any)
	if !ok {
		return nil, errors.New("attestation token has no x-ms-runtime.vm-configuration object")
	}
	secureBoot, ok := vmConfiguration["secure-boot"].(bool)
	if !ok || !secureBoot {
		return nil, errors.New("attestation token does not assert Secure Boot is enabled")
	}
	vtpm, ok := vmConfiguration["tpm-enabled"].(bool)
	if !ok || !vtpm {
		return nil, errors.New("attestation token does not assert vTPM is enabled")
	}

	summary := &cvmAttestationSummary{
		Platform:         platform,
		Issuer:           claimString(claims, "iss"),
		AttestationType:  attestationType,
		ComplianceStatus: complianceStatus,
		SecureBoot:       secureBoot,
		VTPM:             vtpm,
		IssuedAt:         claims["iat"],
		NotBefore:        claims["nbf"],
		ExpiresAt:        claims["exp"],
		UserDataDigest:   strings.ToUpper(userDataDigest),
		PolicyClaims:     make(map[string]any),
	}
	switch platform {
	case cvmPlatformSEVSNP:
		debuggable, ok := claims["x-ms-sevsnpvm-is-debuggable"].(bool)
		if !ok || debuggable {
			return nil, errors.New("SEV-SNP attestation token does not assert a non-debuggable VM")
		}
		if vmpl, err := requiredNumberClaim(claims, "x-ms-sevsnpvm-vmpl"); err != nil || vmpl != 0 {
			return nil, fmt.Errorf("SEV-SNP attestation token must assert VMPL 0")
		}
		migrationAllowed, ok := claims["x-ms-sevsnpvm-migration-allowed"].(bool)
		if !ok || migrationAllowed {
			return nil, errors.New("SEV-SNP attestation token does not assert migration is disabled")
		}
		if _, ok := claims["x-ms-sevsnpvm-smt-allowed"].(bool); !ok {
			return nil, errors.New("SEV-SNP attestation token has no SMT policy claim")
		}
		for _, claim := range []string{
			"x-ms-sevsnpvm-bootloader-svn",
			"x-ms-sevsnpvm-guestsvn",
			"x-ms-sevsnpvm-microcode-svn",
			"x-ms-sevsnpvm-snpfw-svn",
			"x-ms-sevsnpvm-tee-svn",
		} {
			if _, err := requiredNumberClaim(claims, claim); err != nil {
				return nil, err
			}
			summary.PolicyClaims[claim] = claims[claim]
		}
		for _, claim := range []string{"x-ms-sevsnpvm-launchmeasurement", "x-ms-sevsnpvm-reportdata"} {
			if _, err := requiredStringClaim(claims, claim); err != nil {
				return nil, err
			}
			summary.PolicyClaims[claim] = claims[claim]
		}
		summary.PolicyClaims["x-ms-sevsnpvm-vmpl"] = claims["x-ms-sevsnpvm-vmpl"]
		summary.PolicyClaims["x-ms-sevsnpvm-migration-allowed"] = migrationAllowed
		summary.PolicyClaims["x-ms-sevsnpvm-smt-allowed"] = claims["x-ms-sevsnpvm-smt-allowed"]
		summary.PolicyClaims["x-ms-sevsnpvm-is-debuggable"] = debuggable
		summary.DebugStatus = debuggable
	case cvmPlatformTDX:
		tcbStatus, err := requiredStringClaim(claims, "attester_tcb_status")
		if err != nil {
			return nil, err
		}
		if tcbStatus != "UpToDate" {
			return nil, fmt.Errorf("unexpected TDX attester TCB status %q", tcbStatus)
		}
		debugStatus, err := requiredStringClaim(claims, "dbgstat")
		if err != nil {
			return nil, err
		}
		if debugStatus != "disabled" {
			return nil, fmt.Errorf("unexpected TDX debug status %q", debugStatus)
		}
		if debugAttribute, ok := claims["tdx_td_attributes_debug"].(bool); !ok || debugAttribute {
			return nil, errors.New("TDX attestation token does not assert the TD debug attribute is disabled")
		}
		for _, claim := range []string{
			"tdx_tee_tcb_svn",
			"tdx_mrseam",
			"tdx_mrtd",
			"tdx_report_data",
			"tdx_rtmr0",
			"tdx_rtmr1",
			"tdx_rtmr2",
			"tdx_rtmr3",
		} {
			if _, err := requiredStringClaim(claims, claim); err != nil {
				return nil, err
			}
			summary.PolicyClaims[claim] = claims[claim]
		}
		summary.PolicyClaims["attester_tcb_status"] = tcbStatus
		summary.PolicyClaims["dbgstat"] = debugStatus
		summary.PolicyClaims["tdx_td_attributes_debug"] = false
		summary.AttesterTCBStatus = tcbStatus
		summary.DebugStatus = debugStatus
	default:
		return nil, fmt.Errorf("unsupported CVM platform %q", platform)
	}

	return summary, nil
}

func cvmAttestationVerdicts(summary *cvmAttestationSummary) []map[string]any {
	claimsPath := "cvm/attestation-claims.json"
	return []map[string]any{{
		"maaCompliance": map[string]any{
			"passed":   summary.ComplianceStatus == "azure-compliant-cvm",
			"observed": summary.ComplianceStatus,
			"evidence": map[string]any{"path": claimsPath, "claim": "/complianceStatus"},
		},
		"attestationType": map[string]any{
			"passed":   summary.AttestationType == expectedCVMAttestationType(summary.Platform),
			"observed": summary.AttestationType,
			"evidence": map[string]any{"path": claimsPath, "claim": "/attestationType"},
		},
		"secureBoot": map[string]any{
			"passed":   summary.SecureBoot,
			"observed": summary.SecureBoot,
			"evidence": map[string]any{"path": claimsPath, "claim": "/secureBoot"},
		},
		"vTPM": map[string]any{
			"passed":   summary.VTPM,
			"observed": summary.VTPM,
			"evidence": map[string]any{"path": claimsPath, "claim": "/vTPM"},
		},
	}}
}

func expectedCVMAttestationType(platform cvmPlatform) string {
	if platform == cvmPlatformTDX {
		return "tdxvm"
	}
	return "sevsnpvm"
}

func cvmNonceDigest(nonce string) string {
	serializedClaims := fmt.Sprintf(`{"user-claims": {"nonce": "%s"}}`, nonce)
	digest := sha512.Sum512([]byte(serializedClaims))
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

func requiredStringClaim(claims jwt.MapClaims, name string) (string, error) {
	value, ok := claims[name].(string)
	if !ok || value == "" {
		return "", fmt.Errorf("attestation token has no %s claim", name)
	}
	return value, nil
}

func requiredNumberClaim(claims jwt.MapClaims, name string) (float64, error) {
	value, ok := claims[name].(float64)
	if !ok {
		return 0, fmt.Errorf("attestation token has no numeric %s claim", name)
	}
	return value, nil
}

func claimString(claims jwt.MapClaims, name string) string {
	value, _ := claims[name].(string)
	return value
}

func slicesContains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
