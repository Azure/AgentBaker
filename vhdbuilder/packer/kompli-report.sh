#!/bin/bash
set -eux

# kompli-report.sh
#
# Runs kompli (the compliance scanner) on the scan VM against one or more
# committed plans over one or more benchmark definitions, and uploads JSON,
# stderr and the engine exit status separately, per plan. This succeeds the
# CIS-CAT based cis-report.sh: the same kompli binary scans CIS today and
# STIG (and any future benchmark) tomorrow, driven by the committed plans and
# the *.benchmark.json definitions they reference. It emits JSON only (no
# HTML report yet).
#
# Execution model: standalone kompli, run as root (no komplid / --passthrough).
# It executes each COMMITTED plan file (`kompli run <plan>`) supplied by the
# caller, once each — plan generation (`kompli plan`) and the removed `kompli
# audit`/`remediate` subcommands are not used here. The reusable plumbing
# places the definitions and runs the plans; each plan (which rules/modes/
# params) is the consumer's committed, versioned policy. A failure running one
# plan does not stop the remaining plans from running.
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
#   PLAN_TAR_BLOB_NAME        Blob holding a tar of the committed plan files to
#                             run, one entry per plan named "<plan-id>.plan.json"
#   PLAN_NAMES                Comma-separated, sorted list of plan ids (each
#                             matching ^[a-z0-9-]+$) naming the PLAN_TAR_BLOB_NAME
#                             entries to run, in the order the RESULT_BLOB_NAMES/
#                             LOG_BLOB_NAMES/STATUS_BLOB_NAMES lists correlate to
#   DEFS_TAR_BLOB_NAME        Blob holding only the benchmark definitions any
#                             plan references (installed into /etc/kompli/definitions/)
#   RESULT_BLOB_NAMES         Comma-separated blob names to upload each plan's
#                             JSON result to, same order as PLAN_NAMES
#   LOG_BLOB_NAMES            Comma-separated blob names to upload each plan's
#                             kompli stderr log to, same order as PLAN_NAMES
#   STATUS_BLOB_NAMES         Comma-separated blob names for each plan's decimal
#                             kompli run exit code + newline, same order as PLAN_NAMES
#   STORAGE_ACCOUNT_NAME      Scanning storage account
#   SIG_CONTAINER_NAME        Container holding the blobs above
#   AZURE_MSI_RESOURCE_STRING User-assigned MSI resource ID for `az login`
#   ENABLE_TRUSTED_LAUNCH     "true" adds --allow-no-subscriptions to az login
#   TEST_VM_ADMIN_USERNAME    Admin user (kept for parity with cis-report.sh)
#   OS_SKU                    OS SKU (informational)

KOMPLI_BLOB_NAME=${KOMPLI_BLOB_NAME:-""}
PLAN_TAR_BLOB_NAME=${PLAN_TAR_BLOB_NAME:-""}
PLAN_NAMES=${PLAN_NAMES:-""}
DEFS_TAR_BLOB_NAME=${DEFS_TAR_BLOB_NAME:-""}
RESULT_BLOB_NAMES=${RESULT_BLOB_NAMES:-""}
LOG_BLOB_NAMES=${LOG_BLOB_NAMES:-""}
STATUS_BLOB_NAMES=${STATUS_BLOB_NAMES:-""}
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
# positional file (a definition, or a plan) unless the file and its parent
# directory are owned by root and are not group/world-writable.
#
# The work dir must ALSO be on an exec-permitted filesystem: CIS-hardened images
# mount /tmp (and often /var/tmp, /dev/shm) with 'noexec', so running kompli
# from the default mktemp location (/tmp) fails with exit code 126 ("cannot
# execute"). We therefore stage the binary/plans/results under /root — root's
# home on the root filesystem, which is exec-permitted and mode 0700 — which
# also satisfies kompli's root-owned, non-world-writable parent-directory
# checks. This script runs as root under RunShellScript, so /root is writable.
# Overridable (like KOMPLI_DEFS_DIR below) so specs can stage under an
# isolated directory instead of the real /root.
KOMPLI_WORK_ROOT="${KOMPLI_WORK_ROOT:-/root}"

# Each plan records absolute definition paths under /etc/kompli/definitions/.
# Only the union of all plans' selected flat entries is installed there; no
# existing definition or directory permissions are changed. All installed
# files are removed at exit. Overridable (like the parameters above) so specs
# can point installation at an isolated directory instead of the real system path.
KOMPLI_DEFS_DIR="${KOMPLI_DEFS_DIR:-/etc/kompli/definitions}"
WORK_DIR="$(mktemp -d "${KOMPLI_WORK_ROOT}/kompli.XXXXXX")"
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
PLAN_TAR="${WORK_DIR}/plans.tar"
PLAN_STAGE_DIR="${WORK_DIR}/plans"
DEFS_TAR="${WORK_DIR}/definitions.tar"
STAGE_DIR="${WORK_DIR}/definitions"

az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$KOMPLI_BLOB_NAME" --file "$KOMPLI_BIN" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$PLAN_TAR_BLOB_NAME" --file "$PLAN_TAR" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$DEFS_TAR_BLOB_NAME" --file "$DEFS_TAR" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
chmod 0755 "$KOMPLI_BIN"

# Split the comma-separated per-plan parameter lists. Plan ids are validated
# below; the blob-name lists are used only verbatim in upload/download calls
# within this same trusted agent-to-VM channel, same as the singular blob
# names already in use above.
IFS=',' read -r -a plan_names <<< "$PLAN_NAMES"
IFS=',' read -r -a result_blob_names <<< "$RESULT_BLOB_NAMES"
IFS=',' read -r -a log_blob_names <<< "$LOG_BLOB_NAMES"
IFS=',' read -r -a status_blob_names <<< "$STATUS_BLOB_NAMES"
if [ "${#plan_names[@]}" -eq 0 ]; then
    echo "kompli: no plan names supplied" >&2
    exit 1
fi
if [ "${#result_blob_names[@]}" -ne "${#plan_names[@]}" ] || [ "${#log_blob_names[@]}" -ne "${#plan_names[@]}" ] ||
    [ "${#status_blob_names[@]}" -ne "${#plan_names[@]}" ]; then
    echo "kompli: plan/result/log/status name list length mismatch" >&2
    exit 1
fi
for name in "${plan_names[@]}"; do
    if [[ ! "$name" =~ ^[a-z0-9-]+$ ]]; then
        echo "kompli: unsafe plan name: ${name}" >&2
        exit 1
    fi
done

# Validate the downloaded plan archive again on the target: neither a changed
# blob nor tar metadata may place files outside the approved tree.
expected_plan_names=$(printf '%s\n' "${plan_names[@]}" | LC_ALL=C sort -u)
if [ "$(printf '%s\n' "${plan_names[@]}" | wc -l)" -ne "$(printf '%s\n' "$expected_plan_names" | wc -l)" ]; then
    echo "kompli: duplicate plan names" >&2
    exit 1
fi
expected_plan_entries=$(printf '%s\n' "$expected_plan_names" | sed 's/$/.plan.json/')
plan_archive_names=$(tar -tf "$PLAN_TAR") || exit 1
if [ "$(printf '%s\n' "$plan_archive_names" | LC_ALL=C sort)" != "$expected_plan_entries" ]; then
    echo "kompli: plan archive entries do not match supplied plan names" >&2
    exit 1
fi
mkdir -m 0700 "$PLAN_STAGE_DIR"
tar --no-same-owner --no-same-permissions -xf "$PLAN_TAR" -C "$PLAN_STAGE_DIR"
# Refuse symlinks, directories, special files, and repeated archive entries.
if [ "$(find "$PLAN_STAGE_DIR" -mindepth 1 | wc -l)" -ne "${#plan_names[@]}" ]; then
    echo "kompli: unexpected extracted plan entries" >&2
    exit 1
fi
for name in "${plan_names[@]}"; do
    if [ -L "$PLAN_STAGE_DIR/$name.plan.json" ] || [ ! -f "$PLAN_STAGE_DIR/$name.plan.json" ]; then
        echo "kompli: unsafe extracted plan: ${name}" >&2
        exit 1
    fi
done

# Validate each downloaded plan's structure and collect the union of
# definitions any plan references. Multiple plans referencing the same
# definition are expected and collapse to one entry; a duplicate reference
# WITHIN one plan remains rejected.
def_names=()
for name in "${plan_names[@]}"; do
    plan_path="$PLAN_STAGE_DIR/$name.plan.json"
    if ! jq -e -s 'length == 1 and (.[0] | type == "object" and (.benchmarks | type == "array" and length > 0) and all(.benchmarks[]; type == "object" and (.file | type == "string")))' "$plan_path" >/dev/null 2>&1; then
        echo "kompli: invalid downloaded plan: ${name}" >&2
        exit 1
    fi
    mapfile -t plan_def_paths < <(jq -r '.benchmarks[].file' "$plan_path")
    plan_def_names=()
    for path in "${plan_def_paths[@]}"; do
        if [[ ! "$path" =~ ^/etc/kompli/definitions/([A-Za-z0-9][A-Za-z0-9._-]*\.benchmark\.json)$ ]]; then
            echo "kompli: unsafe downloaded plan path (${name}): ${path}" >&2
            exit 1
        fi
        def_name="${BASH_REMATCH[1]}"
        if [[ "$def_name" == *..* ]]; then
            echo "kompli: unsafe downloaded plan definition name (${name}): ${def_name}" >&2
            exit 1
        fi
        for existing in "${plan_def_names[@]}"; do
            if [ "$existing" = "$def_name" ]; then
                echo "kompli: duplicate downloaded plan definition (${name}): ${def_name}" >&2
                exit 1
            fi
        done
        plan_def_names+=("$def_name")
        known=false
        for existing in "${def_names[@]}"; do
            if [ "$existing" = "$def_name" ]; then
                known=true
                break
            fi
        done
        if ! "$known"; then
            def_names+=("$def_name")
        fi
    done
done
expected=$(printf '%s\n' "${def_names[@]}" | LC_ALL=C sort -u)
archive_names=$(tar -tf "$DEFS_TAR") || exit 1
if [ "$(printf '%s\n' "$archive_names" | LC_ALL=C sort)" != "$expected" ]; then
    echo "kompli: archive entries do not match referenced plan definitions" >&2
    exit 1
fi
mkdir -m 0700 "$STAGE_DIR"
tar --no-same-owner --no-same-permissions -xf "$DEFS_TAR" -C "$STAGE_DIR"
# Refuse symlinks, directories, special files, and repeated archive entries.
if [ "$(find "$STAGE_DIR" -mindepth 1 | wc -l)" -ne "${#def_names[@]}" ]; then
    echo "kompli: unexpected extracted definition entries" >&2
    exit 1
fi
for name in "${def_names[@]}"; do
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
# Ancestor paths are overridable (like KOMPLI_DEFS_DIR above) so specs can
# substitute an isolated tree instead of the real /etc.
KOMPLI_ETC_DIR="${KOMPLI_ETC_DIR:-/etc}"
KOMPLI_ETC_KOMPLI_DIR="${KOMPLI_ETC_KOMPLI_DIR:-/etc/kompli}"
kompli_check_definition_dir "$KOMPLI_ETC_DIR" false
kompli_check_definition_dir "$KOMPLI_ETC_KOMPLI_DIR"
kompli_check_definition_dir "$KOMPLI_DEFS_DIR"
# Cleanup checks identity before unlinking, but a check-then-remove race with
# a competing root actor on this dedicated scan VM remains accepted.
# The directory is not writable by untrusted users, but a second root scan
# could still choose the same name. Noclobber uses exclusive creation, unlike
# install(1), and the held descriptor keeps writes on our own inode. Mode 0600
# under umask 077 is root-owned and never group/world-writable.
umask 077
for name in "${def_names[@]}"; do
    target="$KOMPLI_DEFS_DIR/$name"
    set -C
    if exec {definition_fd}>"$target"; then
        set +C
    else
        set +C
        echo "kompli: definition already exists: $name" >&2
        exit 1
    fi
    inode=$(stat -Lc %d:%i "/dev/fd/$definition_fd") || exit 1
    installed_defs+=("$name")
    installed_inodes+=("$inode")
    cat -- "$STAGE_DIR/$name" >&"$definition_fd"
    exec {definition_fd}>&-
done

# Each committed plan executes once. A failure with partial JSON still
# carries its nonzero status; diagnostics and status are transferred as
# independent blobs. A failure (engine or upload) for one plan does not stop
# the remaining plans: this script's own exit code stays 0 regardless, same
# as the single-plan behavior this replaces — real status flows only through
# each plan's status blob.
#
# The status blob is the agent's only proof that a plan's result/log fully
# reached blob storage: it is uploaded LAST, and only once the result and log
# uploads both report success. `az storage blob upload` can create its blob
# yet still report a nonzero exit to its caller (seen in practice), so a
# result/log upload failure must withhold status even though the blob might
# otherwise look fine downstream — the agent already treats a missing or
# invalid status as an operational failure and moves on to the next plan
# without rendering, so withholding status is sufficient; no agent-side
# change is needed. If the status upload itself reports failure, best-effort
# delete whatever it might have created, for the same reason.
plan_failures=0
for i in "${!plan_names[@]}"; do
    name="${plan_names[$i]}"
    plan_path="$PLAN_STAGE_DIR/$name.plan.json"
    result_path="${WORK_DIR}/${name}.result.json"
    log_path="${WORK_DIR}/${name}.log"
    status_path="${WORK_DIR}/${name}.status"
    plan_failed=false

    set +e
    "$KOMPLI_BIN" --verbose --continue-on-error run "$plan_path" > "$result_path" 2> "$log_path"
    run_rc=$?
    set -e
    printf '%s\n' "$run_rc" > "$status_path"
    echo "kompli run (${name}) exited with code: ${run_rc}"
    if [ "$run_rc" -ne 0 ] || [ ! -s "$result_path" ]; then
        plan_failed=true
        echo "----- kompli log (${name}): tail -----"
        tail -n 40 "$log_path" || true
        echo "----------------------------"
    fi
    # Upload even empty stdout/stderr so transfer failure is distinguishable
    # from a valid empty stream; only CI decides whether a result is
    # publishable. One plan's upload failure must not block the next plan.
    set +e
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$result_path" --name "${result_blob_names[$i]}" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
    result_upload_rc=$?
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$log_path" --name "${log_blob_names[$i]}" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
    log_upload_rc=$?
    set -e
    if [ "$result_upload_rc" -ne 0 ] || [ "$log_upload_rc" -ne 0 ]; then
        plan_failed=true
        echo "kompli (${name}): upload failure for result/log; withholding status" >&2
    else
        set +e
        az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$status_path" --name "${status_blob_names[$i]}" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login --overwrite
        status_upload_rc=$?
        set -e
        if [ "$status_upload_rc" -ne 0 ]; then
            plan_failed=true
            echo "kompli (${name}): upload failure for status; removing any partial status blob" >&2
            az storage blob delete --account-name "$STORAGE_ACCOUNT_NAME" --container-name "$SIG_CONTAINER_NAME" --name "${status_blob_names[$i]}" --auth-mode login || true
        fi
    fi
    if "$plan_failed"; then
        plan_failures=$((plan_failures + 1))
    fi
done
echo "kompli: ${#plan_names[@]} plan(s) run, ${plan_failures} with failures"
exit 0
