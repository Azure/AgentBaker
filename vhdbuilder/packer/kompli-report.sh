#!/bin/bash
set -eux

# kompli-report.sh
#
# Runs kompli (the compliance scanner) on the scan VM against a committed
# plan over one or more benchmark definitions, and uploads JSON, stderr and
# the engine exit status separately. This succeeds the CIS-CAT based cis-report.sh: the same kompli binary scans CIS today and
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
# JSON report with engine exit 0. An engine error (including partial JSON) is
# recorded as nonzero status, not mistaken for completed compliance findings.
#
# Parameters (environment variables):
#   KOMPLI_BLOB_NAME          Blob holding the kompli binary
#   PLAN_BLOB_NAME            Blob holding the committed kompli plan file to run
#   DEFS_TAR_BLOB_NAME        Blob holding only the benchmark definitions the
#                             plan references (installed into /etc/kompli/definitions/)
#   RESULT_BLOB_NAME          Blob name to upload the JSON result to
#   LOG_BLOB_NAME             Blob name to upload the kompli stderr log to
#   STATUS_BLOB_NAME          Blob name for decimal kompli run exit code + newline
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
STATUS_BLOB_NAME=${STATUS_BLOB_NAME:-""}
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
# The plan records absolute definition paths under /etc/kompli/definitions/.
# Only its selected flat entries are installed there; no existing definition or
# directory permissions are changed. All installed files are removed at exit.
KOMPLI_DEFS_DIR="/etc/kompli/definitions"
WORK_DIR="$(mktemp -d /root/kompli.XXXXXX)"
chmod 0700 "$WORK_DIR"
installed_defs=()
installed_inodes=()
cleanup() {
    local index target current
    for index in "${!installed_defs[@]}"; do
        target="${KOMPLI_DEFS_DIR}/${installed_defs[index]}"
        # Never delete a replacement created by another run. Identity was
        # recorded from our open descriptor immediately after exclusive create.
        current=$(stat -c %d:%i -- "$target" 2>/dev/null) || continue
        if [ ! -L "$target" ] && [ "$current" = "${installed_inodes[index]}" ]; then
            rm -f -- "$target" || true
        else
            echo "kompli: leaving replaced definition: $target" >&2
        fi
    done
    rm -rf -- "$WORK_DIR" || true
}
trap cleanup EXIT

KOMPLI_BIN="${WORK_DIR}/kompli"
PLAN_PATH="${WORK_DIR}/plan.json"
DEFS_TAR="${WORK_DIR}/definitions.tar"
STAGE_DIR="${WORK_DIR}/definitions"
RESULT_PATH="${WORK_DIR}/result.json"
LOG_PATH="${WORK_DIR}/kompli.log"
STATUS_PATH="${WORK_DIR}/run.status"

az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$KOMPLI_BLOB_NAME" --file "$KOMPLI_BIN" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$PLAN_BLOB_NAME" --file "$PLAN_PATH" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$DEFS_TAR_BLOB_NAME" --file "$DEFS_TAR" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
chmod 0755 "$KOMPLI_BIN"

# Validate the downloaded plan and flat archive again on the target: neither
# a changed blob nor tar metadata may place files outside the approved tree.
if ! jq -e -s 'length == 1 and (.[0] | type == "object" and (.benchmarks | type == "array" and length > 0) and all(.[]; type == "object" and (.file | type == "string")))' "$PLAN_PATH" >/dev/null 2>&1; then
    echo "kompli: invalid downloaded plan" >&2
    exit 1
fi
mapfile -t plan_paths < <(jq -r '.benchmarks[].file' "$PLAN_PATH")
plan_names=()
for path in "${plan_paths[@]}"; do
    if [[ ! "$path" =~ ^/etc/kompli/definitions/([A-Za-z0-9][A-Za-z0-9._-]*\.benchmark\.json)$ ]]; then
        echo "kompli: unsafe downloaded plan path: ${path}" >&2
        exit 1
    fi
    name="${BASH_REMATCH[1]}"
    if [[ "$name" == *..* ]]; then
        echo "kompli: unsafe downloaded plan name: ${name}" >&2
        exit 1
    fi
    plan_names+=("$name")
done
expected=$(printf '%s\n' "${plan_names[@]}" | LC_ALL=C sort -u)
if [ "$(printf '%s\n' "${plan_names[@]}" | wc -l)" -ne "$(printf '%s\n' "$expected" | wc -l)" ]; then
    echo "kompli: duplicate downloaded plan paths" >&2
    exit 1
fi
archive_names=$(tar -tf "$DEFS_TAR") || exit 1
if [ "$(printf '%s\n' "$archive_names" | LC_ALL=C sort)" != "$expected" ]; then
    echo "kompli: archive entries do not match referenced plan definitions" >&2
    exit 1
fi
mkdir -m 0700 "$STAGE_DIR"
tar --no-same-owner --no-same-permissions -xf "$DEFS_TAR" -C "$STAGE_DIR"
# Refuse symlinks, directories, special files, and repeated archive entries.
if [ "$(find "$STAGE_DIR" -mindepth 1 | wc -l)" -ne "${#plan_names[@]}" ]; then
    echo "kompli: unexpected extracted definition entries" >&2
    exit 1
fi
for name in "${plan_names[@]}"; do
    if [ -L "$STAGE_DIR/$name" ] || [ ! -f "$STAGE_DIR/$name" ]; then
        echo "kompli: unsafe extracted definition: ${name}" >&2
        exit 1
    fi
done

# Check each parent before installing a definition. Never chmod/chown an
# existing directory. InputSecurity requires root ownership and no group/world
# write permission throughout the target path.
kompli_check_definition_dir() {
    local dir="$1" create_missing="${2:-true}" owner mode
    if [ -L "$dir" ]; then
        echo "kompli: definition path is a symlink: $dir" >&2
        return 1
    fi
    if [ ! -e "$dir" ] && [ "$create_missing" = true ]; then
        install -d -o root -g root -m 0755 "$dir" || return 1
    fi
    if [ ! -d "$dir" ]; then
        echo "kompli: definition path is not a real directory: $dir" >&2
        return 1
    fi
    owner=$(stat -c %u -- "$dir") || return 1
    mode=$(stat -c %a -- "$dir") || return 1
    if [ "$owner" -ne 0 ] || (( (8#$mode & 8#022) != 0 )); then
        echo "kompli: definition path is not root-owned and non-writable: $dir" >&2
        return 1
    fi
}
kompli_check_definition_dir /etc false
kompli_check_definition_dir /etc/kompli
kompli_check_definition_dir "$KOMPLI_DEFS_DIR"
# Cleanup checks identity before unlinking, but a check-then-remove race with
# a competing root actor on this dedicated scan VM remains accepted.
# The directory is not writable by untrusted users, but a second root scan
# could still choose the same name. Noclobber uses exclusive creation, unlike
# install(1), and the held descriptor keeps writes on our own inode. Mode 0600
# under umask 077 is root-owned and never group/world-writable.
umask 077
for name in "${plan_names[@]}"; do
    target="$KOMPLI_DEFS_DIR/$name"
    set -C
    if exec {definition_fd}>"$target"; then
        set +C
    else
        set +C
        echo "kompli: definition already exists: $name" >&2
        exit 1
    fi
    inode=$(stat -Lc %d:%i "/proc/$$/fd/$definition_fd") || exit 1
    installed_defs+=("$name")
    installed_inodes+=("$inode")
    cat -- "$STAGE_DIR/$name" >&"$definition_fd"
    exec {definition_fd}>&-
done

# One committed-plan execution. A failure with partial JSON still carries its
# nonzero status; diagnostics and status are transferred as independent blobs.
set +e
"$KOMPLI_BIN" --verbose --continue-on-error run "$PLAN_PATH" > "$RESULT_PATH" 2> "$LOG_PATH"
run_rc=$?
set -e
printf '%s\n' "$run_rc" > "$STATUS_PATH"
echo "kompli run exited with code: ${run_rc}"
if [ "$run_rc" -ne 0 ] || [ ! -s "$RESULT_PATH" ]; then
    echo "----- kompli log: tail -----"
    tail -n 40 "$LOG_PATH" || true
    echo "----------------------------"
fi
# Upload even empty stdout/stderr so transfer failure is distinguishable from
# a valid empty stream; only CI decides whether a result is publishable.
az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$RESULT_PATH" --name "$RESULT_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$LOG_PATH" --name "$LOG_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$STATUS_PATH" --name "$STATUS_BLOB_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
exit 0
