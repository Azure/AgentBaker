#!/bin/bash
set -eux

# kompli-report.sh
#
# Runs kompli (the compliance scanner) on the scan VM against a committed
# plan over one or more benchmark definitions, and uploads the native JSON result
# to the scanning storage account. This is the benchmark-agnostic successor to
# the CIS-CAT based cis-report.sh: the same kompli binary scans CIS today and
# STIG (and any future benchmark) tomorrow, driven by the committed plan and the
# *.benchmark.json definitions it references. It emits JSON only (no HTML report
# yet).
#
# Execution model: standalone kompli, run as root (no komplid / --passthrough).
# It executes a COMMITTED plan file (`kompli run <plan>`) supplied by the caller
# — plan generation (`kompli plan`) and the removed `kompli audit`/`remediate`
# subcommands are not used here. See ADR-0001: the reusable plumbing places the
# definitions and runs the plan; the plan (which rules/modes/params) is the
# consumer's committed, versioned policy.
#
# It is invoked ON the scan VM (as root) via `az vm run-command invoke
# --command-id RunShellScript` from vhd-scanning.sh, mirroring how cis-report.sh
# is invoked. Parameters arrive as environment variables (the `KEY=VALUE` form
# passed to --parameters), matching cis-report.sh.
#
# This scan is intentionally NON-BLOCKING: a NonCompliant result still produces a
# JSON report and the script exits 0. Only the caller decides how to surface
# findings; here we never fail the build on compliance content.
#
# Parameters (environment variables):
#   KOMPLI_BLOB_NAME          Blob holding the kompli binary
#   PLAN_BLOB_NAME            Blob holding the committed kompli plan file to run
#   DEFS_TAR_BLOB_NAME        Blob holding a tar of the benchmark definitions the
#                             plan references (installed into /etc/kompli/definitions/)
#   RESULT_BLOB_NAME          Blob name to upload the JSON result to
#   LOG_BLOB_NAME             Blob name to upload the kompli stderr log to
#   STORAGE_ACCOUNT_NAME      Scanning storage account
#   SIG_CONTAINER_NAME        Container holding the blobs above
#   AZURE_MSI_RESOURCE_STRING User-assigned MSI resource ID for `az login`
#   ENABLE_TRUSTED_LAUNCH     "true" adds --allow-no-subscriptions to az login
#   TEST_VM_ADMIN_USERNAME    Admin user (kept for parity with cis-report.sh)
#   OS_SKU                    OS SKU (informational)

KOMPLI_BLOB_NAME=${KOMPLI_BLOB_NAME:-""}
PLAN_BLOB_NAME=${PLAN_BLOB_NAME:-""}
DEFS_TAR_BLOB_NAME=${DEFS_TAR_BLOB_NAME:-""}
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

# kompli's input hardening (azure-osconfig InputSecurity.cpp) refuses to read a
# positional file (the definition, and the generated plan) unless the file and
# its parent directory are owned by root and are not group/world-writable.
#
# The work dir must ALSO be on an exec-permitted filesystem: CIS-hardened images
# mount /tmp (and often /var/tmp, /dev/shm) with 'noexec', so running kompli
# from the default mktemp location (/tmp) fails with exit code 126 ("cannot
# execute"). We therefore stage the binary/plan/result under /root — root's home
# on the root filesystem, which is exec-permitted and mode 0700 — which also
# satisfies kompli's root-owned, non-world-writable parent-directory checks. This
# script runs as root under RunShellScript, so /root is writable.
#
# The benchmark DEFINITIONS the plan references must live in
# /etc/kompli/definitions/: a plan records only each definition's BARE FILENAME,
# and `kompli run` re-resolves those against the fixed /etc/kompli/definitions/
# tree (ADR-0004) — a definition staged only in the work dir would not be found
# at run time. We create the tree root-owned (no `kompli` group exists on the
# scan VM without komplid, so root:root; the parent stays non-group/world-writable,
# satisfying the input hardening) and install the definitions there.
KOMPLI_DEFS_DIR="/etc/kompli/definitions"
WORK_DIR="$(mktemp -d /root/kompli.XXXXXX)"
chmod 0700 "$WORK_DIR"
cleanup() {
    rm -rf "$WORK_DIR" || true
    # Remove only the definitions this run installed (leave the tree itself).
    rm -rf "${KOMPLI_DEFS_DIR:?}/"* || true
}
trap cleanup EXIT

KOMPLI_BIN="${WORK_DIR}/kompli"
PLAN_PATH="${WORK_DIR}/plan.json"
DEFS_TAR="${WORK_DIR}/definitions.tar"
RESULT_PATH="${WORK_DIR}/result.json"
LOG_PATH="${WORK_DIR}/kompli.log"

# Download the kompli binary, the committed plan, and the tar of definitions the
# plan references — all staged by vhd-scanning.sh running on the agent.
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$KOMPLI_BLOB_NAME" --file "$KOMPLI_BIN" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$PLAN_BLOB_NAME" --file "$PLAN_PATH" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$DEFS_TAR_BLOB_NAME" --file "$DEFS_TAR" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login

chmod 0755 "$KOMPLI_BIN"

# Install the definitions into the fixed /etc/kompli/definitions/ tree, root-owned
# and non-group/world-writable, where `kompli run` resolves each plan block's
# bare filename. Extract without restoring archived ownership/modes, then set the
# ADR-0004-compatible modes (root:root here — no `kompli` group on a komplid-less
# scan VM).
install -d -o root -g root -m 0755 "$KOMPLI_DEFS_DIR"
tar --no-same-owner --no-same-permissions -xf "$DEFS_TAR" -C "$KOMPLI_DEFS_DIR"
chown -R root:root "$KOMPLI_DEFS_DIR"
find "$KOMPLI_DEFS_DIR" -type f -exec chmod 0644 {} +

# Execute the committed plan. `kompli run` re-resolves each block's definition in
# /etc/kompli/definitions/, re-validates applicability against this host, and
# emits the canonical result JSON on stdout. kompli logs to stderr (there is no
# --log-file), captured to the log. --continue-on-error keeps `run` going past a
# single rule's procedure error so a partial JSON is still emitted. A
# NonCompliant result is expected and must never fail the (shadow, non-blocking)
# scan. Plan GENERATION is out of scope here — the plan is a committed artifact
# (ADR-0001).
run_rc=0
if [ -s "$PLAN_PATH" ]; then
    set +e
    "$KOMPLI_BIN" --verbose --continue-on-error run "$PLAN_PATH" > "$RESULT_PATH" 2> "$LOG_PATH"
    run_rc=$?
    set -e
    echo "kompli run exited with code: ${run_rc}"
else
    echo "WARNING: no committed plan file downloaded; skipping run"
fi

# Surface the reason for a failure or empty result inline (the log carries
# "benchmark is not applicable for the current distribution", a rule's procedure
# error, etc.) so the pipeline output shows the cause without opening the
# published log artifact.
if [ "${run_rc}" -ne 0 ] || [ ! -s "$RESULT_PATH" ]; then
    echo "----- kompli log: error lines -----"
    grep -iE 'error|abort|not applicable|exception|fatal' "$LOG_PATH" 2>/dev/null | tail -n 40 || true
    echo "----- kompli log: tail -----"
    tail -n 40 "$LOG_PATH" 2>/dev/null || true
    echo "------------------------------------------------------"
fi

# Upload the JSON result and the kompli log even if the scan reported
# NonCompliant or failed, so the agent always has something to publish.
if [ -s "$RESULT_PATH" ]; then
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$RESULT_PATH" --name "$RESULT_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
else
    echo "WARNING: kompli produced no JSON result on stdout"
fi
if [ -s "$LOG_PATH" ]; then
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$LOG_PATH" --name "$LOG_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
fi

echo "kompli report script completed for plan blob ${PLAN_BLOB_NAME}"
exit 0
