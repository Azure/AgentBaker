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

# Run the audit. `audit` emits the canonical result JSON on stdout. No --format
# flag: --format is render-only and is rejected on audit by current assessors;
# JUnit is rendered later on the agent from this JSON. --log-file captures all
# assessor logging (and disables console logging) so stdout stays parseable.
# --continue-on-error keeps the audit going when an individual rule's procedure
# fails: without it the assessor aborts the whole MOF on the first rule error
# (returns 1 and emits no JSON, since the JSON formatter only flushes at the
# end), which would drop every result. For a shadow, non-blocking scan we want
# the partial JSON plus a logged error rather than nothing.
# Capture the exit code but never abort: a NonCompliant result is expected and
# must not fail the (shadow, non-blocking) scan.
audit_rc=0
set +e
"$ASSESSOR_BIN" --verbose --continue-on-error --log-file "$LOG_PATH" audit "$MOF_PATH" > "$RESULT_PATH" 2> "${WORK_DIR}/stderr.log"
audit_rc=$?
set -e
echo "compliance-engine-assessor exited with code: ${audit_rc}"

# Surface a little context in the run-command output for debugging.
echo "----- compliance-engine-assessor stderr -----"
cat "${WORK_DIR}/stderr.log" 2>/dev/null || true
echo "---------------------------------------------"

# The assessor disables console logging when --log-file is set, so on a failure
# or an empty result the reason (e.g. "benchmark is not applicable for the
# current distribution", or a specific rule's procedure error) lives ONLY in the
# log file. Surface the error lines and the tail inline so the pipeline output
# shows the cause without having to open the published log artifact.
if [ "${audit_rc}" -ne 0 ] || [ ! -s "$RESULT_PATH" ]; then
    echo "----- compliance-engine-assessor log: error lines -----"
    grep -iE 'error|abort|not applicable|exception|fatal' "$LOG_PATH" 2>/dev/null | tail -n 40 || true
    echo "----- compliance-engine-assessor log: tail -----"
    tail -n 40 "$LOG_PATH" 2>/dev/null || true
    echo "------------------------------------------------------"
fi

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
