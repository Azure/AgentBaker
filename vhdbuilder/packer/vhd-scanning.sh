#!/bin/bash
set -eux

source ./parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh

# Downstream publication must not inherit a stale output directory on skip.
echo "##vso[task.setvariable variable=KOMPLI_SCAN_OUTPUT_DIR;]"

# This variable is used to determine where we need to deploy the VM on which we'll run trivy.
# We must be sure this location matches the location used by packer when delivering the output image
# version to the staging gallery, as the particular image version will only have a single replica in this region.
if [ -z "$PACKER_BUILD_LOCATION" ]; then
    echo "PACKER_BUILD_LOCATION must be set to run VHD scanning"
    exit 1
fi

CURRENT_TIME=$(date +%s)

TRIVY_SCRIPT_PATH="trivy-scan.sh"
SCAN_RESOURCE_PREFIX="vhd-scanning"
SCAN_VM_NAME="$SCAN_RESOURCE_PREFIX-vm-$CURRENT_TIME-$RANDOM"
VHD_IMAGE="$MANAGED_SIG_ID"

SIG_CONTAINER_NAME="vhd-scans"
SCAN_VM_ADMIN_USERNAME="azureuser"

# shellcheck disable=SC3010
if [ "${ENVIRONMENT,,}" = "tme" ]; then
    ACCOUNT_NAME="$ACCOUNT_NAME_TME"
    KUSTO_DATABASE="$KUSTO_DATABASE_TME"
    KUSTO_TABLE="$KUSTO_TABLE_TME"
    KUSTO_ENDPOINT="$KUSTO_ENDPOINT_TME"
    UMSI_CLIENT_ID="$UMSI_CLIENT_ID_TME"
    UMSI_PRINCIPAL_ID="$UMSI_PRINCIPAL_ID_TME"
    UMSI_RESOURCE_ID="$UMSI_RESOURCE_ID_TME"
fi

RELEASE_NOTES_FILEPATH="$(pwd)/release-notes.txt"
if [ ! -f "${RELEASE_NOTES_FILEPATH}" ]; then
    echo "${RELEASE_NOTES_FILEPATH} does not exist"
    exit 1
fi

# we must create VMs in a vnet subnet which has access to the storage account, otherwise they will not be able to access the VHD blobs
SCANNING_SUBNET_ID="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${PACKER_VNET_RESOURCE_GROUP_NAME}/providers/Microsoft.Network/virtualNetworks/${PACKER_VNET_NAME}/subnets/scanning"
if [ -z "$(az network vnet subnet show --ids $SCANNING_SUBNET_ID | jq -r '.id')" ]; then
    echo "scanning subnet $SCANNING_SUBNET_ID seems to be missing, unable to create scanning VM"
    exit 1
fi

# Use the domain name from the classic blob URL to get the storage account name.
# If the CLASSIC_BLOB var is not set create a new var called BLOB_STORAGE_NAME in the pipeline.
BLOB_URL_REGEX="^https:\/\/.+\.blob\.core\.windows\.net\/vhd(s)?$"
# shellcheck disable=SC3010
if [[ $CLASSIC_BLOB =~ $BLOB_URL_REGEX ]]; then
    STORAGE_ACCOUNT_NAME=$(echo $CLASSIC_BLOB | sed -E 's|https://(.*)\.blob\.core\.windows\.net(:443)?/(.*)?|\1|')
else
    # Used in the 'AKS Linux VHD Build - PR check-in gate' pipeline.
    if [ -z "$BLOB_STORAGE_NAME" ]; then
        echo "BLOB_STORAGE_NAME is not set, please either set the CLASSIC_BLOB var or create a new var BLOB_STORAGE_NAME in the pipeline."
        exit 1
    fi
    STORAGE_ACCOUNT_NAME=${BLOB_STORAGE_NAME}
fi

set +x
# use a random, non-guessable admin password (prefix guarantees upper/lower/special to satisfy Azure VM complexity rules).
# openssl rand -hex avoids piping through tr: under `set -e` without `pipefail`, a pipeline's exit status is
# that of its last command, so a silent openssl failure would otherwise go undetected (tr succeeds on empty input).
SCAN_VM_ADMIN_PASSWORD="ScanVM@$(openssl rand -hex 24)"
set -x

RESOURCE_GROUP_NAME="$SCAN_RESOURCE_PREFIX-$CURRENT_TIME-$RANDOM"
az group create --name $RESOURCE_GROUP_NAME --location ${PACKER_BUILD_LOCATION} --tags "source=AgentBaker" "now=${CURRENT_TIME}" "branch=${GIT_BRANCH}"

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT
capture_benchmark "${SCRIPT_NAME}_set_variables_and_create_scan_resource_group"

VM_SIZE="Standard_D8ds_v5"
VM_OPTIONS="--size $VM_SIZE"
# shellcheck disable=SC3010
if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    # Ampere Altra (v5) doesn't support TrustedLaunch; Cobalt 100 (v6) does
    # TODO: remove once all relevant images have been updated to TrustedLaunchSupported and have corresponding TL-based AgentBaker E2E tests
    if [ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ]; then
        VM_SIZE="Standard_D8pds_v6"
    else
        VM_SIZE="Standard_D8pds_v5"
    fi
    VM_OPTIONS="--size $VM_SIZE"
fi

# TODO: remove once all relevant images have been updated to TrustedLaunchSupported and have corresponding TL-based AgentBaker E2E tests
if [ "${OS_TYPE}" = "Linux" ] && [ "${ENABLE_TRUSTED_LAUNCH,,}" = "true" ]; then
    # Unsigned dev ACL base images (aclBaseImageSource=acldevel) cannot boot with Secure Boot on.
    SECURE_BOOT="true"
    ACL_BASE_IMAGE_SIGNED="${ACL_BASE_IMAGE_SIGNED:-true}"
    if [ "${ACL_BASE_IMAGE_SIGNED,,}" = "false" ]; then
        SECURE_BOOT="false"
    fi
    VM_OPTIONS+=" --security-type TrustedLaunch --enable-secure-boot ${SECURE_BOOT} --enable-vtpm true"
fi

if [ "${OS_TYPE}" = "Linux" ] && grep -q "cvm" <<< "$FEATURE_FLAGS"; then
    VM_SIZE="${CVM_TEST_VM_SIZE:-Standard_DC8ads_v5}"
    # We completely re-assign the VM_OPTIONS string here to ensure that no artifacts from earlier conditionals are included
    VM_OPTIONS="--size $VM_SIZE --security-type ConfidentialVM --enable-secure-boot true --enable-vtpm true --os-disk-security-encryption-type VMGuestStateOnly"
    if [ "${OS_SKU:-}" != "AzureContainerLinux" ]; then
        VM_OPTIONS+=" --specialized true"
    fi
fi

# NVIDIA GB specific VM options for scanning (uses standard ARM64 VM for now)
if [ "${OS_TYPE}" = "Linux" ] && grep -q "NVIDIA_GB" <<< "$FEATURE_FLAGS"; then
    echo "NVIDIA GB: Using standard ARM64 VM options for scanning"
    # Additional NVIDIA GB-specific VM options can be added here when GB SKUs are available
fi

SCANNING_NIC_ID=$(az network nic create --resource-group $RESOURCE_GROUP_NAME --name "scanning${CURRENT_TIME}${RANDOM}" --subnet $SCANNING_SUBNET_ID | jq -r '.NewNIC.id')
if [ -z "$SCANNING_NIC_ID" ]; then
    echo "unable to create new NIC for scanning VM"
    exit 1
fi

# Create VM using appropriate method based on scenario
if [ "${OS_SKU}" = "Ubuntu" ] && [ "${OS_VERSION}" = "22.04" ] && [ "$(printf %s "${ENABLE_FIPS}" | tr '[:upper:]' '[:lower:]')" = "true" ]; then
    # Source the FIPS helper functions
    FULL_PATH=$(realpath $0)
    CDIR=$(dirname $FULL_PATH)
    source "$CDIR/fips-helper.sh"

    # Register FIPS feature and create VM using REST API. Exit if any step fails.
    ensure_fips_feature_registered || exit $?
    create_fips_vm \
        "$VM_SIZE" \
        "$SCAN_VM_NAME" \
        "$SCAN_VM_ADMIN_USERNAME" \
        SCAN_VM_ADMIN_PASSWORD \
        "$VHD_IMAGE" \
        "$SCANNING_NIC_ID" \
        "$UMSI_RESOURCE_ID" \
        "$RESOURCE_GROUP_NAME" || exit $?
else
    echo "Creating VM using standard az vm create command..."

    # Disable tracing to prevent password from appearing in logs
    set +x
    # Use the standard VM creation approach for all other scenarios
    az vm create --resource-group $RESOURCE_GROUP_NAME \
        --name $SCAN_VM_NAME \
        --image $VHD_IMAGE \
        --nics $SCANNING_NIC_ID \
        --admin-username $SCAN_VM_ADMIN_USERNAME \
        --admin-password $SCAN_VM_ADMIN_PASSWORD \
        --os-disk-size-gb 60 \
        ${VM_OPTIONS} \
        --assign-identity "${UMSI_RESOURCE_ID}"

    # Check for errors in the az vm create command
    AZ_VM_CREATE_EXIT_CODE=$?
    # Re-enable tracing after sensitive command
    set -x
    if [ $AZ_VM_CREATE_EXIT_CODE -ne 0 ]; then
        echo "Error: Failed to create VM" >&2
        exit 1
    fi
fi

capture_benchmark "${SCRIPT_NAME}_create_scan_vm"
set +x

# for scanning storage account/container upload access
az vm identity assign -g $RESOURCE_GROUP_NAME --name $SCAN_VM_NAME --identities $AZURE_MSI_RESOURCE_STRING

FULL_PATH=$(realpath $0)
CDIR=$(dirname $FULL_PATH)
TRIVY_SCRIPT_PATH="$CDIR/$TRIVY_SCRIPT_PATH"

TIMESTAMP=$(date +%s%3N)
TRIVY_UPLOAD_REPORT_NAME="trivy-report-${BUILD_ID}-${TIMESTAMP}.json"
TRIVY_UPLOAD_TABLE_NAME="trivy-table-${BUILD_ID}-${TIMESTAMP}.txt"
CVE_DIFF_UPLOAD_REPORT_NAME="cve-diff-${BUILD_ID}-${TIMESTAMP}.txt"
CVE_LIST_UPLOAD_REPORT_NAME="cve-list-${BUILD_ID}-${TIMESTAMP}.txt"

# Extract date, revision from build number
BUILD_RUN_NUMBER=$(echo $BUILD_RUN_NUMBER | cut -d_ -f 1)

# set image version locally, if it is not set in environment variable
if [ -z "${IMAGE_VERSION:-}" ]; then
    IMAGE_VERSION=$(date +%Y%m.%d.0)
    echo "IMAGE_VERSION was not set, setting it to ${IMAGE_VERSION} for trivy scan and Kusto ingestion"
fi

az vm run-command invoke \
    --command-id RunShellScript \
    --name $SCAN_VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$TRIVY_SCRIPT_PATH \
    --parameters "OS_SKU=${OS_SKU}" \
        "OS_VERSION=${OS_VERSION}" \
        "SCAN_VM_ADMIN_USERNAME=${SCAN_VM_ADMIN_USERNAME}" \
        "ARCHITECTURE=${ARCHITECTURE}" \
        "SIG_CONTAINER_NAME"=${SIG_CONTAINER_NAME} \
        "STORAGE_ACCOUNT_NAME"=${STORAGE_ACCOUNT_NAME} \
        "VHD_ARTIFACT_NAME"=${VHD_ARTIFACT_NAME} \
        "SKU_NAME"=${SKU_NAME} \
        "KUSTO_ENDPOINT"=${KUSTO_ENDPOINT} \
        "KUSTO_DATABASE"=${KUSTO_DATABASE} \
        "KUSTO_TABLE"=${KUSTO_TABLE} \
        "TRIVY_UPLOAD_REPORT_NAME"=${TRIVY_UPLOAD_REPORT_NAME} \
        "TRIVY_UPLOAD_TABLE_NAME"=${TRIVY_UPLOAD_TABLE_NAME} \
        "ACCOUNT_NAME"=${ACCOUNT_NAME} \
        "BLOB_URL"=${BLOB_URL} \
        "SEVERITY"=${SEVERITY} \
        "MODULE_VERSION"=${MODULE_VERSION} \
        "UMSI_PRINCIPAL_ID"=${UMSI_PRINCIPAL_ID} \
        "UMSI_CLIENT_ID"=${UMSI_CLIENT_ID} \
        "AZURE_MSI_RESOURCE_STRING"=${AZURE_MSI_RESOURCE_STRING} \
        "BUILD_RUN_NUMBER"=${BUILD_RUN_NUMBER} \
        "BUILD_REPOSITORY_NAME"=${BUILD_REPOSITORY_NAME} \
        "BUILD_SOURCEBRANCH"=${GIT_BRANCH} \
        "BUILD_SOURCEVERSION"=${BUILD_SOURCEVERSION} \
        "SYSTEM_COLLECTIONURI"=${SYSTEM_COLLECTIONURI} \
        "SYSTEM_TEAMPROJECT"=${SYSTEM_TEAMPROJECT} \
        "BUILDID"=${BUILD_ID} \
        "IMAGE_VERSION"=${IMAGE_VERSION} \
        "CVE_DIFF_UPLOAD_REPORT_NAME"=${CVE_DIFF_UPLOAD_REPORT_NAME} \
        "CVE_LIST_UPLOAD_REPORT_NAME"=${CVE_LIST_UPLOAD_REPORT_NAME} \
        "SCAN_RESOURCE_PREFIX"=${SCAN_RESOURCE_PREFIX}

capture_benchmark "${SCRIPT_NAME}_run_az_scan_command"

az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_UPLOAD_REPORT_NAME} --file trivy-report.json --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${TRIVY_UPLOAD_TABLE_NAME} --file  trivy-images-table.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${CVE_DIFF_UPLOAD_REPORT_NAME} --file  cve-diff.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login
az storage blob download --container-name ${SIG_CONTAINER_NAME} --name  ${CVE_LIST_UPLOAD_REPORT_NAME} --file  cve-list.txt --account-name ${STORAGE_ACCOUNT_NAME} --auth-mode login

az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_UPLOAD_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${TRIVY_UPLOAD_TABLE_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${CVE_DIFF_UPLOAD_REPORT_NAME} --auth-mode login
az storage blob delete --account-name ${STORAGE_ACCOUNT_NAME} --container-name ${SIG_CONTAINER_NAME} --name ${CVE_LIST_UPLOAD_REPORT_NAME} --auth-mode login

capture_benchmark "${SCRIPT_NAME}_download_and_delete_blobs"

echo "=== CVEs fixed in version: ${IMAGE_VERSION}" >> ${RELEASE_NOTES_FILEPATH}
cat cve-diff.txt >> ${RELEASE_NOTES_FILEPATH}

# error if cve-list.txt non-empty
if [ -s cve-list.txt ]; then
    printf "##vso[task.logissue type=error]Error: cve-list.txt is not empty. Please address the listed CVEs.\n%s\n" "$(cat cve-list.txt)"
    echo "##vso[task.complete result=SucceededWithIssues;]"
fi

echo -e "Trivy Scan Script Completed\n\n\n"

capture_benchmark "${SCRIPT_NAME}_cis_report_start"

# --- CIS Report Generation and Upload ---

# Check if OS requires CIS scan
isMarinerOrAzureLinux() {
    local os="$1"
    if [ "$os" = "CBLMariner" ] || [ "$os" = "AzureLinux" ]; then
        return 0
    fi
    return 1
}
isCISUnsupportedUbuntu() {
    local os="$1"
    local version="$2"

    # Only 22.04+ are supported.
    if [ "$os" = "Ubuntu" ] && [ "$version" = "20.04" ]; then
        return 0
    fi

    # No CIS benchmarks yet available for Ubuntu 26.04 (resolute)
    # TODO(2604): enable 26.04 CIS scanning when support is added by upstream CIS.
    if [ "$os" = "Ubuntu" ] && [ "$version" = "26.04" ]; then
        return 0
    fi

    return 1
}
isFlatcar() {
    local os="$1"

    if [ "$os" = "Flatcar" ]; then
        return 0
    fi
    return 1
}
isACL() {
    local os="$1"

    if [ "$os" = "AzureContainerLinux" ]; then
        return 0
    fi
    return 1
}
isAzureLinuxOSGuard() {
    local os="$1"

    if [ "$os" = "AzureLinuxOSGuard" ]; then
        return 0
    fi
    return 1
}
isUbuntuCVM() {
    local os="$1"
    local feature_flags="$2"

    if [ "$os" = "Ubuntu" ] && grep -q "cvm" <<< "$feature_flags"; then
        return 0
    fi
    return 1
}
requiresCISScan() {
    local os="$1"
    local version="$2"

    if isMarinerOrAzureLinux "$os"; then
        return 1
    fi
    if isCISUnsupportedUbuntu "$os" "$version"; then
        return 1
    fi
    if isFlatcar "$os"; then
        return 1
    fi
    if isACL "$os"; then
        return 1
    fi
    if isAzureLinuxOSGuard "$os"; then
        return 1
    fi
    return 0 # Requires scan
}

# First check if this OS requires CIS scanning
if ! requiresCISScan "${OS_SKU}" "${OS_VERSION}"; then
    echo "CIS scan not required for ${OS_SKU} ${OS_VERSION}"
    capture_benchmark "${SCRIPT_NAME}_cis_report_skipped"
    capture_benchmark "${SCRIPT_NAME}_overall" true
    process_benchmarks
    exit 0
fi

# --- kompli scan (shadow, non-blocking) ---
# Runs the benchmark-agnostic kompli on the same scan VM,
# alongside (not instead of) the CIS-CAT scan below. Returns JSON, log and
# engine status per plan; CI renders each plan's JUnit only after that plan's
# operationally successful run. This runs BEFORE the CIS-CAT block on
# purpose, so the CIS-CAT skip/license early-exits do not suppress the kompli
# results. It is fully non-blocking: compliance findings never fail the
# build, and any operational error here is logged as a failure but cannot
# abort the CIS-CAT scan that follows.
#
# The reusable inputs are laid down on the agent by
# .pipelines/templates/.kompli-scan-template.yaml at:
#   vhdbuilder/kompli/kompli
#   vhdbuilder/kompli/definitions/ubuntu-<version>-<level>.benchmark.json
# The per-consumer POLICY is every committed plan (which rules/modes/params)
# under this repo's per-OS-version directory:
#   vhdbuilder/packer/kompli-plans/ubuntu-<version>/<plan-name>.plan.json
# Every plan found there runs exactly once per invocation, each with its own
# result/log/status/JUnit; an empty or missing directory skips the scan
# (non-blocking), same as before.
KOMPLI_SCRIPT_PATH="$CDIR/kompli-report.sh"
KOMPLI_BIN_LOCAL="$CDIR/../kompli/kompli"
KOMPLI_DEF_DIR="$CDIR/../kompli/definitions"
KOMPLI_PLAN_DIR="$CDIR/kompli-plans"

# Validate the selected plan against manifest keys and acquired definitions
# before any blob upload/VM action. Emit only its approved flat filenames.
kompli_plan_definitions() {
    local plan="$1" manifest="$2" def_dir="$3" path name key
    local -a paths=() names=()
    if ! jq -e -s 'length == 1 and (.[0] | type == "object" and (.benchmarks | type == "array" and length > 0) and all(.benchmarks[]; type == "object" and (.file | type == "string")))' "$plan" >/dev/null 2>&1; then
        echo "kompli: invalid committed plan" >&2
        return 1
    fi
    mapfile -t paths < <(jq -r '.benchmarks[].file' "$plan")
    for path in "${paths[@]}"; do
        if [[ ! "$path" =~ ^/etc/kompli/definitions/([A-Za-z0-9][A-Za-z0-9._-]*\.benchmark\.json)$ ]]; then
            echo "kompli: unsafe definition path: ${path}" >&2
            return 1
        fi
        name="${BASH_REMATCH[1]}"
        if [[ "$name" == *..* ]]; then
            echo "kompli: unsafe definition name: ${name}" >&2
            return 1
        fi
        for key in "${names[@]}"; do
            if [ "$key" = "$name" ]; then
                echo "kompli: duplicate plan definition: ${name}" >&2
                return 1
            fi
        done
        key="${name%.benchmark.json}"
        if ! jq -e --arg key "$key" '.benchmarks | type == "object" and has($key)' "$manifest" >/dev/null 2>&1; then
            echo "kompli: no approved manifest mapping for ${name}" >&2
            return 1
        fi
        if [ -L "${def_dir}/${name}" ] || [ ! -f "${def_dir}/${name}" ]; then
            echo "kompli: missing or unsafe acquired definition: ${name}" >&2
            return 1
        fi
        names+=("$name")
    done
    printf '%s\n' "${names[@]}"
}

# Extract and validate this plan's rule ids, so rendering can be restricted
# to exactly the rules the plan executed — without this, every rule outside
# the plan appears as a skipped test in that plan's JUnit.
kompli_plan_rule_ids() {
    local plan="$1" id
    local -a ids=()
    if ! jq -e -s 'length == 1 and (.[0] | type == "object" and (.benchmarks | type == "array" and length > 0) and all(.benchmarks[]; .rules | type == "object"))' "$plan" >/dev/null 2>&1; then
        echo "kompli: invalid committed plan (rules)" >&2
        return 1
    fi
    mapfile -t ids < <(jq -r '[.benchmarks[].rules | keys[]] | unique[]' "$plan")
    if [ "${#ids[@]}" -eq 0 ]; then
        echo "kompli: committed plan selects no rules" >&2
        return 1
    fi
    for id in "${ids[@]}"; do
        if [[ ! "$id" =~ ^[0-9A-Za-z._-]+$ ]]; then
            echo "kompli: unsafe rule id: ${id}" >&2
            return 1
        fi
    done
    printf '%s\n' "${ids[@]}"
}

run_kompli_scan() {
    local key="ubuntu-${OS_VERSION}"
    local skip="${SKIP_KOMPLI:-false}"
    if [ "${skip,,}" = "true" ]; then
        echo "Skipping kompli scan (SKIP_KOMPLI=true)"
        return 0
    fi
    local plan_dir="${KOMPLI_PLAN_DIR}/${key}"
    local -a plan_files=() plan_ids=()
    local plan_file pid
    if [ -d "$plan_dir" ]; then
        mapfile -t plan_files < <(find "$plan_dir" -maxdepth 1 -type f -name '*.plan.json' -printf '%f\n' | LC_ALL=C sort)
    fi
    if [ "${#plan_files[@]}" -eq 0 ]; then
        echo "No committed kompli plans at ${plan_dir}; skipping (non-blocking)"
        return 0
    fi
    for plan_file in "${plan_files[@]}"; do
        pid="${plan_file%.plan.json}"
        if [[ ! "$pid" =~ ^[a-z0-9-]+$ ]]; then
            echo "kompli: unsafe plan name: ${plan_file}" >&2
            return 1
        fi
        plan_ids+=("$pid")
    done
    if [ ! -x "$KOMPLI_BIN_LOCAL" ]; then
        echo "kompli not found at ${KOMPLI_BIN_LOCAL}" >&2
        return 1
    fi

    # Keep one private input snapshot and all this invocation's outputs
    # together. The ADO publisher is pointed only at this fresh directory.
    local output_dir plans_snapshot_dir defs_snapshot_dir selected verified
    local -a def_names=() uploaded_blobs=()
    local vm_invoked=false
    output_dir=$(mktemp -d "$PWD/kompli-scan.XXXXXXXX") || return 1
    echo "##vso[task.setvariable variable=KOMPLI_SCAN_OUTPUT_DIR;]${output_dir}"
    plans_snapshot_dir="$output_dir/plans"
    defs_snapshot_dir="$output_dir/definitions"
    trap 'rm -rf -- "$plans_snapshot_dir" "$defs_snapshot_dir"' RETURN
    mkdir -m 0700 "$plans_snapshot_dir" || return 1
    mkdir -m 0700 "$defs_snapshot_dir" || return 1
    local def_name known existing
    for pid in "${plan_ids[@]}"; do
        cp -- "${plan_dir}/${pid}.plan.json" "$plans_snapshot_dir/${pid}.plan.json" || return 1
        selected=$(kompli_plan_definitions "$plans_snapshot_dir/${pid}.plan.json" "$CDIR/kompli-scan.json" "$KOMPLI_DEF_DIR") || return 1
        while IFS= read -r def_name; do
            [ -n "$def_name" ] || continue
            known=false
            for existing in "${def_names[@]}"; do
                if [ "$existing" = "$def_name" ]; then
                    known=true
                    break
                fi
            done
            if ! "$known"; then
                def_names+=("$def_name")
                # Preserve symlinks so validation of the snapshot rejects any swap.
                cp -P -- "$KOMPLI_DEF_DIR/$def_name" "$defs_snapshot_dir/$def_name" || return 1
            fi
        done <<< "$selected"
        verified=$(kompli_plan_definitions "$plans_snapshot_dir/${pid}.plan.json" "$CDIR/kompli-scan.json" "$defs_snapshot_dir") || return 1
        [ "$verified" = "$selected" ] || return 1
    done

    local kompli_ts run_id kompli_blob plans_tar_blob defs_tar_blob
    kompli_ts=$(date +%s%3N)
    run_id="${kompli_ts}-${output_dir##*/}"
    kompli_blob="kompli-${BUILD_ID}-${run_id}"
    plans_tar_blob="kompli-plans-${key}-${BUILD_ID}-${run_id}.tar"
    defs_tar_blob="kompli-defs-${key}-${BUILD_ID}-${run_id}.tar"

    local -a result_blobs=() log_blobs=() status_blobs=()
    local -a result_locals=() log_locals=() status_locals=() junit_locals=() render_tmps=() render_logs=()
    for pid in "${plan_ids[@]}"; do
        result_blobs+=("kompli-result-${key}-${pid}-${BUILD_ID}-${run_id}.json")
        log_blobs+=("kompli-log-${key}-${pid}-${BUILD_ID}-${run_id}.log")
        status_blobs+=("kompli-status-${key}-${pid}-${BUILD_ID}-${run_id}.status")
        result_locals+=("$output_dir/kompli-${key}-${pid}.json")
        log_locals+=("$output_dir/kompli-${key}-${pid}.log")
        status_locals+=("$output_dir/kompli-${key}-${pid}.status")
        junit_locals+=("$output_dir/kompli-${key}-${pid}.junit.xml")
        render_tmps+=("$output_dir/.kompli-${key}-${pid}.junit.part")
        render_logs+=("$output_dir/kompli-${key}-${pid}.render.log")
    done

    local plans_tar defs_tar cleanup_name
    plans_tar="$output_dir/plans.tar"
    defs_tar="$output_dir/definitions.tar"
    # RETURN runs while this invocation's local blob names are in scope.
    trap 'for cleanup_name in "${uploaded_blobs[@]}"; do
        az storage blob delete --account-name "$STORAGE_ACCOUNT_NAME" --container-name "$SIG_CONTAINER_NAME" --name "$cleanup_name" --auth-mode login || true
    done
    if "$vm_invoked"; then
        for cleanup_i in "${!plan_ids[@]}"; do
            for cleanup_name in "${result_blobs[cleanup_i]}" "${log_blobs[cleanup_i]}" "${status_blobs[cleanup_i]}"; do
                az storage blob delete --account-name "$STORAGE_ACCOUNT_NAME" --container-name "$SIG_CONTAINER_NAME" --name "$cleanup_name" --auth-mode login || true
            done
        done
    fi
    rm -f -- "$defs_tar" "$plans_tar"
    rm -rf -- "$plans_snapshot_dir" "$defs_snapshot_dir"' RETURN
    local -a plan_tar_entries=()
    for pid in "${plan_ids[@]}"; do
        plan_tar_entries+=("${pid}.plan.json")
    done
    tar -cf "$plans_tar" -C "$plans_snapshot_dir" "${plan_tar_entries[@]}" || return 1
    tar -cf "$defs_tar" -C "$defs_snapshot_dir" "${def_names[@]}" || return 1

    echo "Uploading kompli, ${#plan_ids[@]} plan(s) (${key}) and ${#def_names[@]} referenced definitions"
    # Blob names are random for this invocation. Register before each attempt:
    # an upload can create a blob yet report an error to its caller.
    uploaded_blobs+=("$kompli_blob")
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$KOMPLI_BIN_LOCAL" --name "$kompli_blob" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login || return 1
    uploaded_blobs+=("$plans_tar_blob")
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$plans_tar" --name "$plans_tar_blob" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login || return 1
    uploaded_blobs+=("$defs_tar_blob")
    az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$defs_tar" --name "$defs_tar_blob" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login || return 1

    local ret invoke_rc=0
    local plan_names_csv result_blobs_csv log_blobs_csv status_blobs_csv
    plan_names_csv=$(IFS=,; echo "${plan_ids[*]}")
    result_blobs_csv=$(IFS=,; echo "${result_blobs[*]}")
    log_blobs_csv=$(IFS=,; echo "${log_blobs[*]}")
    status_blobs_csv=$(IFS=,; echo "${status_blobs[*]}")
    echo "kompli: running ${#plan_ids[@]} committed plan(s): ${plan_ids[*]}"
    vm_invoked=true
    ret=$(az vm run-command invoke \
        --command-id RunShellScript \
        --name "$SCAN_VM_NAME" \
        --resource-group "$RESOURCE_GROUP_NAME" \
        --scripts @"$KOMPLI_SCRIPT_PATH" \
        --parameters "KOMPLI_BLOB_NAME=${kompli_blob}" \
            "PLAN_TAR_BLOB_NAME=${plans_tar_blob}" \
            "PLAN_NAMES=${plan_names_csv}" \
            "DEFS_TAR_BLOB_NAME=${defs_tar_blob}" \
            "RESULT_BLOB_NAMES=${result_blobs_csv}" \
            "LOG_BLOB_NAMES=${log_blobs_csv}" \
            "STATUS_BLOB_NAMES=${status_blobs_csv}" \
            "STORAGE_ACCOUNT_NAME=${STORAGE_ACCOUNT_NAME}" \
            "SIG_CONTAINER_NAME=${SIG_CONTAINER_NAME}" \
            "AZURE_MSI_RESOURCE_STRING=${AZURE_MSI_RESOURCE_STRING}" \
            "ENABLE_TRUSTED_LAUNCH=${ENABLE_TRUSTED_LAUNCH}" \
            "TEST_VM_ADMIN_USERNAME=${SCAN_VM_ADMIN_USERNAME}" \
            "OS_SKU=${OS_SKU}") || invoke_rc=$?
    echo "$ret"

    local overall_rc=0 pi
    local -a outcomes=()
    for pi in "${!plan_ids[@]}"; do
        pid="${plan_ids[$pi]}"
        local result_blob="${result_blobs[$pi]}" log_blob="${log_blobs[$pi]}" status_blob="${status_blobs[$pi]}"
        local result_local="${result_locals[$pi]}" log_local="${log_locals[$pi]}" status_local="${status_locals[$pi]}"
        local junit_local="${junit_locals[$pi]}" render_tmp="${render_tmps[$pi]}" render_log="${render_logs[$pi]}"
        local result_rc=0 log_rc=0 status_rc=0 run_rc

        az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$result_blob" --file "$result_local" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login || result_rc=$?
        az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$log_blob" --file "$log_local" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login || log_rc=$?
        az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$status_blob" --file "$status_local" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login || status_rc=$?

        # Partial JSON from a failed engine is diagnostic, never a successful scan.
        # The VM withholds (or removes) the status blob unless the result and
        # log for this plan both uploaded successfully, so a present, valid
        # status blob is also this plan's proof of complete transfer.
        if [ "$status_rc" -ne 0 ] || [ ! -s "$status_local" ] || ! [[ $(<"$status_local") =~ ^[0-9]+$ ]]; then
            echo "kompli ${key} ${pid}: operational failure (missing or invalid engine status)" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (status)")
            continue
        fi
        run_rc=$(<"$status_local")
        if [ "$run_rc" -ne 0 ]; then
            echo "kompli ${key} ${pid}: operational failure (engine exit ${run_rc}); JSON/log retained" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (engine exit ${run_rc})")
            continue
        fi
        if [ "$invoke_rc" -ne 0 ] || [ "$result_rc" -ne 0 ] || [ "$log_rc" -ne 0 ]; then
            echo "kompli ${key} ${pid}: operational failure (run-command or result/log transfer); diagnostics retained" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (transfer)")
            continue
        fi
        if [ ! -s "$result_local" ] || ! jq -e -s 'length == 1 and (.[0] | type == "object")' "$result_local" >/dev/null 2>&1; then
            echo "kompli ${key} ${pid}: operational failure (missing or invalid JSON); diagnostics retained" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (invalid JSON)")
            continue
        fi
        # Restrict rendering to exactly this plan's rules: without --section,
        # every rule outside the plan appears as a skipped test.
        local rule_ids
        rule_ids=$(kompli_plan_rule_ids "$plans_snapshot_dir/${pid}.plan.json") || {
            echo "kompli ${key} ${pid}: operational failure (rule id extraction); JSON/log retained" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (rule ids)")
            continue
        }
        local -a sections=()
        mapfile -t sections <<< "$rule_ids"
        local -a render_args=()
        local rule_id
        for rule_id in "${sections[@]}"; do
            render_args+=(--section="$rule_id")
        done
        if ! "$KOMPLI_BIN_LOCAL" render --format junit --suite-name "kompli ${key} ${pid}" "${render_args[@]}" "$result_local" > "$render_tmp" 2> "$render_log" || [ ! -s "$render_tmp" ]; then
            rm -f -- "$render_tmp"
            echo "kompli ${key} ${pid}: operational failure (JUnit render); JSON/log retained" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (render)")
            continue
        fi
        if ! mv -- "$render_tmp" "$junit_local"; then
            echo "kompli ${key} ${pid}: operational failure (JUnit promotion)" >&2
            overall_rc=1
            outcomes+=("${pid}: failed (promotion)")
            continue
        fi
        [ -s "$render_log" ] || rm -f -- "$render_log"
        echo "kompli ${key} ${pid}: completed scan (engine exit 0); JUnit at ${junit_local}"
        outcomes+=("${pid}: ok")
    done
    echo "kompli ${key}: plan outcomes: ${outcomes[*]}"
    return "$overall_rc"
}

# Preserve the non-blocking shadow policy while reporting real scan failure.
( set +e; run_kompli_scan ) || echo "kompli: FAILED operational scan (shadow policy: continuing CIS-CAT)" >&2
capture_benchmark "${SCRIPT_NAME}_kompli_scan"

# Set this pipeline variable to true if CIS scanning is broken
SKIP_CIS=${SKIP_CIS:-false}
if [ "${SKIP_CIS,,}" = "true" ]; then
    # For artifacts
    touch cis-report.txt
    touch cis-report.html
    echo "Skipping CIS assessment as SKIP_CIS is set to true"
    capture_benchmark "${SCRIPT_NAME}_cis_report_skipped"
    capture_benchmark "${SCRIPT_NAME}_overall" true
    process_benchmarks
    exit 0
fi

# Compare current cis-report.txt against stored baseline for Ubuntu 22.04 / 24.04.
# A regression is when a rule that previously "pass" now has any other result.
assemble_cis_report() {
    local l1_file="$1"
    local l2_file="$2"
    local output_file="$3"

    if [ ! -f "${l1_file}" ]; then
        printf '##vso[task.logissue type=error]Missing L1 CIS report file: %s\n' "$l1_file"
        return 1
    fi
    if [ ! -f "${l2_file}" ]; then
        printf '##vso[task.logissue type=error]Missing L2 CIS report file: %s\n' "$l2_file"
        return 1
    fi

    local -A l1_rules
    while IFS= read -r line; do
        case "$line" in
            pass:*|fail:*|manual:*|error:*|unknown:*)
                local rest
                rest=$(printf '%s' "$line" | cut -d':' -f2-)
                rest="${rest# }"
                local rule_id=${rest%% *}
                if [ -n "$rule_id" ]; then
                    l1_rules["$rule_id"]=1
                fi
                ;;
        esac
    done < "${l1_file}"

    while IFS= read -r line; do
        case "$line" in
            pass:*|fail:*|manual:*|error:*|unknown:*)
                local status rest rule_id desc level
                status=$(printf '%s' "$line" | cut -d':' -f1)
                rest=$(printf '%s' "$line" | cut -d':' -f2-)
                rest="${rest# }"
                rule_id=${rest%% *}
                desc="${rest#"$rule_id"}"
                level="L2"
                if [ -n "${l1_rules[$rule_id]-}" ]; then
                    level="L1"
                fi
                printf '%s: %s [%s]%s\n' "$status" "$rule_id" "$level" "$desc"
                ;;
            *)
                printf '%s\n' "$line"
                ;;
        esac
    done < "${l2_file}" > "${output_file}"
}

compare_cis_with_baseline() {
    local baseline_file="vhdbuilder/packer/cis/baselines/${OS_SKU,,}/${OS_VERSION}.txt"
    local current_file="cis-report.txt"
    local regressions_file="cis-regressions.txt"

    if [ ! -f "${baseline_file}" ]; then
        printf '##vso[task.logissue type=error]Missing baseline file: %s\n' "$baseline_file"
        echo "Baseline file ${baseline_file} not found; skipping comparison"
        return 0
    fi
    if [ ! -f "${current_file}" ]; then
        printf '##vso[task.logissue type=error]Missing cis-report file: %s\n' "$current_file"
        return 0
    fi

    # Build associative arrays of baseline pass statuses and current statuses
    # shellcheck disable=SC2034
    declare -A baseline_pass
    declare -A current_status

    # Extract pass rule IDs from baseline (status line format: "pass: <RULE_ID> <Description>")
    # Accept rule IDs composed of digits and dots.
    while IFS= read -r line; do
        # quick filter
        case "$line" in
            pass:*) ;;
            *) continue ;;
        esac
        # shellcheck disable=SC2001
        rule_id=$(echo "$line" | sed -E 's/^pass: ([0-9][0-9.]*).*$/\1/')
        if [ -n "$rule_id" ]; then
            # We can't modify bootloader configuration for CVM so need to skip this rule
            if isUbuntuCVM "${OS_SKU}" "$FEATURE_FLAGS" && [ "$rule_id" = "1.3.1.2" ]; then
                continue
            fi
            baseline_pass["$rule_id"]=1
        fi
    done < "${baseline_file}"

    # Capture any status line in current report and map rule id -> status
    while IFS= read -r line; do
        # Expected prefixes: pass: fail: manual: (others ignored)
        case "$line" in
            pass:*|fail:*|manual:*|error:*|unknown:*) ;;
            *) continue ;;
        esac
        # status is token before colon
        status_token=${line%%:*}
        # Extract rule id if present
        # shellcheck disable=SC2001
        rule_id=$(echo "$line" | sed -E 's/^[a-zA-Z]+: ([0-9][0-9.]*).*$/\1/')
        if [ -n "$rule_id" ]; then
            current_status["$rule_id"]=$status_token
        fi
    done < "${current_file}"

    : > "${regressions_file}"
    local regression_count=0
    for rule_id in "${!baseline_pass[@]}"; do
        baseline_status="pass"
        current_rule_status=${current_status["$rule_id"]-}
        if [ -z "$current_rule_status" ]; then
            # Missing rule considered regression
            printf '%s|%s->MISSING\n' "$rule_id" "$baseline_status" >> "${regressions_file}"
            regression_count=$((regression_count+1))
        elif [ "$current_rule_status" != "pass" ]; then
            printf '%s|%s->%s\n' "$rule_id" "$baseline_status" "$current_rule_status" >> "${regressions_file}"
            regression_count=$((regression_count+1))
        fi
    done

    if [ $regression_count -gt 0 ]; then
        echo "CIS regressions detected: $regression_count"
        echo "Regression details (rule_id|baseline->current):"
        cat "${regressions_file}"
        # Azure DevOps error log issue so it surfaces clearly
        printf '##vso[task.logissue type=error]CIS regressions detected (%d). See cis-regressions.txt for details.\n' "$regression_count"
        echo "##vso[task.complete result=SucceededWithIssues;]"
    else
        echo "No CIS regressions detected against baseline ${baseline_file}"
        rm -f "${regressions_file}"
    fi
}

CIS_SCRIPT_PATH="$CDIR/cis-report.sh"
CIS_REPORT_L1_TXT_NAME="cis-report-l1-${BUILD_ID}-${TIMESTAMP}.txt"
CIS_REPORT_L2_TXT_NAME="cis-report-l2-${BUILD_ID}-${TIMESTAMP}.txt"
CIS_REPORT_HTML_NAME="cis-report-${BUILD_ID}-${TIMESTAMP}.html"
CIS_REPORT_L1_LOCAL="cis-report-l1.txt"
CIS_REPORT_L2_LOCAL="cis-report-l2.txt"

# Upload cisassessor tarball to storage account
if [ "${ARCHITECTURE,,}" = "arm64" ]; then
    CISASSESSOR_LOCAL_PATH="$CDIR/../cisassessor-arm64.tar.gz"
else
    CISASSESSOR_LOCAL_PATH="$CDIR/../cisassessor-amd64.tar.gz"
fi
CISASSESSOR_BLOB_NAME="cisassessor-${BUILD_ID}-${TIMESTAMP}.tar.gz"
az storage blob upload --container-name "${SIG_CONTAINER_NAME}" --file "${CISASSESSOR_LOCAL_PATH}" --name "${CISASSESSOR_BLOB_NAME}" --account-name "${STORAGE_ACCOUNT_NAME}" --auth-mode login

# Run CIS report script on VM (pass storage info)
ret=$(az vm run-command invoke \
    --command-id RunShellScript \
    --name $SCAN_VM_NAME \
    --resource-group $RESOURCE_GROUP_NAME \
    --scripts @$CIS_SCRIPT_PATH \
    --parameters "CISASSESSOR_BLOB_NAME=${CISASSESSOR_BLOB_NAME}" \
        "STORAGE_ACCOUNT_NAME=${STORAGE_ACCOUNT_NAME}" \
        "SIG_CONTAINER_NAME=${SIG_CONTAINER_NAME}" \
        "AZURE_MSI_RESOURCE_STRING=${AZURE_MSI_RESOURCE_STRING}" \
        "ENABLE_TRUSTED_LAUNCH=${ENABLE_TRUSTED_LAUNCH}" \
        "CIS_REPORT_L1_TXT_NAME=${CIS_REPORT_L1_TXT_NAME}" \
        "CIS_REPORT_L2_TXT_NAME=${CIS_REPORT_L2_TXT_NAME}" \
        "CIS_REPORT_HTML_NAME=${CIS_REPORT_HTML_NAME}" \
        "TEST_VM_ADMIN_USERNAME=${SCAN_VM_ADMIN_USERNAME}" \
        "OS_SKU=${OS_SKU}"
)
echo "$ret"
msg=$(echo -E "$ret" | jq -r '.value[].message')
echo "$msg"

if printf '%s\n' "$msg" | grep -q "The selected assessment content is limited by the terms of the associated CIS license key" && \
    printf '%s\n' "$msg" | grep -q "Assessment 1 Exit Value: 122"; then
    printf '##vso[task.logissue type=warning]Skipping CIS report comparison because CIS-CAT license does not allow the selected assessment content.\n'
    printf 'CIS assessment skipped: CIS-CAT license does not allow the selected assessment content.\n' > cis-report.txt
    printf '<html><body>CIS assessment skipped: CIS-CAT license does not allow the selected assessment content.</body></html>\n' > cis-report.html
    az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CISASSESSOR_BLOB_NAME}" --auth-mode login
    capture_benchmark "${SCRIPT_NAME}_cis_report_license_limited"
    capture_benchmark "${SCRIPT_NAME}_overall" true
    process_benchmarks
    exit 0
fi

# Download CIS report files to working directory
az storage blob download --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_L1_TXT_NAME}" --file "${CIS_REPORT_L1_LOCAL}" --account-name "${STORAGE_ACCOUNT_NAME}" --auth-mode login
az storage blob download --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_L2_TXT_NAME}" --file "${CIS_REPORT_L2_LOCAL}" --account-name "${STORAGE_ACCOUNT_NAME}" --auth-mode login
az storage blob download --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_HTML_NAME}" --file cis-report.html --account-name "${STORAGE_ACCOUNT_NAME}" --auth-mode login

assemble_cis_report "${CIS_REPORT_L1_LOCAL}" "${CIS_REPORT_L2_LOCAL}" "cis-report.txt"

# Remove CIS report blobs from storage
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_L1_TXT_NAME}" --auth-mode login
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_L2_TXT_NAME}" --auth-mode login
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_HTML_NAME}" --auth-mode login
# Remove CIS kompli tarball blob from storage
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CISASSESSOR_BLOB_NAME}" --auth-mode login

echo -e "CIS Report Script Completed\n\n\n"
capture_benchmark "${SCRIPT_NAME}_cis_report_upload_and_download"

echo -e "Comparing CIS report against baseline"
compare_cis_with_baseline

capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
