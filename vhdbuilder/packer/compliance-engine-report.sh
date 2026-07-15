#!/bin/bash
set -eux

# compliance-engine-report.sh
#
# Runs the Compliance Engine assessor (compliance-engine-assessor) against a
# single benchmark MOF on the scan VM and uploads the native JSON result to the
# scanning storage account. This is the benchmark-agnostic successor to the
# CIS-CAT based cis-report.sh: the same assessor binary audits CIS today and
# STIG (and any future benchmark) tomorrow, driven entirely by the MOF it is
# given. It emits JSON only (no HTML report yet).
#
# It is invoked ON the scan VM (as root) via `az vm run-command invoke
# --command-id RunShellScript` from vhd-scanning.sh, mirroring how cis-report.sh
# is invoked. Parameters arrive as environment variables (the `KEY=VALUE` form
# passed to --parameters), matching cis-report.sh.
#
# This scan is intentionally NON-BLOCKING: a NonCompliant audit result still
# produces a JSON report and the script exits 0. Only the caller decides how to
# surface findings; here we never fail the build on compliance content.
#
# Parameters (environment variables):
#   ASSESSOR_BLOB_NAME         Blob holding the compliance-engine-assessor binary
#   MOF_BLOB_NAME              Blob holding the benchmark .mof to audit
#   RESULT_BLOB_NAME          Blob name to upload the JSON result to
#   LOG_BLOB_NAME             Blob name to upload the assessor --log-file output to
#   STORAGE_ACCOUNT_NAME      Scanning storage account
#   SIG_CONTAINER_NAME        Container holding the blobs above
#   AZURE_MSI_RESOURCE_STRING User-assigned MSI resource ID for `az login`
#   ENABLE_TRUSTED_LAUNCH     "true" adds --allow-no-subscriptions to az login
#   TEST_VM_ADMIN_USERNAME    Admin user (kept for parity with cis-report.sh)
#   OS_SKU                    OS SKU (informational)

ASSESSOR_BLOB_NAME=${ASSESSOR_BLOB_NAME:-""}
MOF_BLOB_NAME=${MOF_BLOB_NAME:-""}
RESULT_BLOB_NAME=${RESULT_BLOB_NAME:-""}
LOG_BLOB_NAME=${LOG_BLOB_NAME:-""}
STORAGE_ACCOUNT_NAME=${STORAGE_ACCOUNT_NAME:-""}
SIG_CONTAINER_NAME=${SIG_CONTAINER_NAME:-""}
AZURE_MSI_RESOURCE_STRING=${AZURE_MSI_RESOURCE_STRING:-""}
ENABLE_TRUSTED_LAUNCH=${ENABLE_TRUSTED_LAUNCH:-""}
TEST_VM_ADMIN_USERNAME=${TEST_VM_ADMIN_USERNAME:-"azureuser"}
OS_SKU=${OS_SKU:-""}

# Azure login helper (mirrors cis-report.sh)
login_with_user_assigned_managed_identity() {
    local TYPE_FLAG="$1"
    local ID=$2
    LOGIN_FLAGS="--identity $TYPE_FLAG $ID"
    if [ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ]; then
        LOGIN_FLAGS="$LOGIN_FLAGS --allow-no-subscriptions"
    fi
    echo "logging into azure with flags: $LOGIN_FLAGS"
    az login $LOGIN_FLAGS
}

if [ -z "$AZURE_MSI_RESOURCE_STRING" ]; then
    echo "AZURE_MSI_RESOURCE_STRING must be set for az login"
    exit 1
fi
login_with_user_assigned_managed_identity "--resource-id" "$AZURE_MSI_RESOURCE_STRING"

# The assessor's input hardening (azure-osconfig InputSecurity.cpp) refuses to
# read the MOF, and refuses to write the --log-file, unless the file and its
# parent directory are owned by root and are not group/world-writable.
#
# The work dir must ALSO be on an exec-permitted filesystem: CIS-hardened images
# mount /tmp (and often /var/tmp, /dev/shm) with 'noexec', so running the
# assessor from the default mktemp location (/tmp) fails with exit code 126
# ("cannot execute"). We therefore stage under /root — root's home on the root
# filesystem, which is exec-permitted and mode 0700 — which also satisfies the
# assessor's root-owned, non-world-writable parent-directory checks. This script
# runs as root under RunShellScript, so /root is writable.
WORK_DIR="$(mktemp -d /root/compliance-engine.XXXXXX)"
chmod 0700 "$WORK_DIR"
cleanup() {
    rm -rf "$WORK_DIR" || true
}
trap cleanup EXIT

ASSESSOR_BIN="${WORK_DIR}/compliance-engine-assessor"
MOF_PATH="${WORK_DIR}/benchmark.mof"
RESULT_PATH="${WORK_DIR}/result.json"
LOG_PATH="${WORK_DIR}/compliance-engine-assessor.log"

# Download the assessor binary and the benchmark MOF from the scanning storage
# account. Both were staged there by vhd-scanning.sh running on the agent.
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$ASSESSOR_BLOB_NAME" --file "$ASSESSOR_BIN" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$MOF_BLOB_NAME" --file "$MOF_PATH" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login

chmod 0755 "$ASSESSOR_BIN"
chmod 0644 "$MOF_PATH"

# Diagnostics: exit code 126 ("cannot execute") is almost always an architecture
# mismatch (binary built for a different arch than this VM) or the work dir being
# on a 'noexec' mount. Log enough to tell the two apart from the pipeline output.
echo "----- assessor exec diagnostics -----"
echo "VM arch (uname -m): $(uname -m)"
echo "assessor permissions/owner:"; stat -c '  %A  %U:%G  %s bytes  %n' "$ASSESSOR_BIN" 2>/dev/null || echo "  (stat failed)"
echo "assessor file type:"; file "$ASSESSOR_BIN" 2>/dev/null || echo "  (file(1) not available)"
# ELF header e_machine lives at byte offset 18 (0x3e=x86-64, 0xb7=AArch64).
echo "assessor ELF magic + e_machine (offset 0 and 18):"
od -An -tx1 -N 20 "$ASSESSOR_BIN" 2>/dev/null || true
# ldd reveals missing shared libraries ("=> not found") or, on an arch mismatch,
# prints "not a dynamic executable" / an error — useful either way.
echo "assessor dynamic dependencies (ldd):"
ldd "$ASSESSOR_BIN" 2>&1 | sed 's/^/  /' || true
echo "work dir mount options:"; findmnt -no TARGET,OPTIONS --target "$WORK_DIR" 2>/dev/null || mount 2>/dev/null | grep -E " on / " || true
# noexec self-test: if a trivial script here cannot run, the fs is noexec; if it
# runs but the assessor does not, the assessor is the wrong arch/format.
printf '#!/bin/sh\nexit 0\n' > "${WORK_DIR}/.exec-test.sh"
chmod 0755 "${WORK_DIR}/.exec-test.sh"
if "${WORK_DIR}/.exec-test.sh" 2>/dev/null; then
    echo "noexec self-test: PASS (work dir is exec-permitted)"
else
    echo "noexec self-test: FAIL (work dir appears to be mounted noexec)"
fi
rm -f "${WORK_DIR}/.exec-test.sh"
echo "-------------------------------------"

# Run the audit. stdout is the pure JSON result; --log-file captures all
# assessor logging (and disables console logging) so stdout stays parseable.
# Capture the exit code but never abort: a NonCompliant result is expected and
# must not fail the (shadow, non-blocking) scan.
audit_rc=0
set +e
"$ASSESSOR_BIN" --verbose --log-file "$LOG_PATH" --format json audit "$MOF_PATH" > "$RESULT_PATH" 2> "${WORK_DIR}/stderr.log"
audit_rc=$?
set -e
echo "compliance-engine-assessor exited with code: ${audit_rc}"

# Surface a little context in the run-command output for debugging.
echo "----- compliance-engine-assessor stderr -----"
cat "${WORK_DIR}/stderr.log" 2>/dev/null || true
echo "---------------------------------------------"

# Upload the JSON result and the assessor log even if the audit reported
# NonCompliant or failed, so the agent always has something to publish.
if [ -s "$RESULT_PATH" ]; then
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$RESULT_PATH" --name "$RESULT_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
else
    echo "WARNING: assessor produced no JSON result on stdout"
fi
if [ -s "$LOG_PATH" ]; then
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$LOG_PATH" --name "$LOG_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
fi

echo "compliance-engine report script completed for MOF blob ${MOF_BLOB_NAME}"
exit 0
