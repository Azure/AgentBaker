#!/bin/bash
set -eux

source ./parts/linux/cloud-init/artifacts/cse_benchmark_functions.sh

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
SCAN_VM_ADMIN_PASSWORD="ScanVM@$CURRENT_TIME"
set -x

RESOURCE_GROUP_NAME="$SCAN_RESOURCE_PREFIX-$CURRENT_TIME-$RANDOM"
az group create --name $RESOURCE_GROUP_NAME --location ${PACKER_BUILD_LOCATION} --tags "source=AgentBaker" "now=${CURRENT_TIME}" "branch=${GIT_BRANCH}"

function cleanup() {
    echo "Deleting resource group ${RESOURCE_GROUP_NAME}"
    az group delete --name $RESOURCE_GROUP_NAME --yes --no-wait
}
trap cleanup EXIT
capture_benchmark "${SCRIPT_NAME}_set_variables_and_create_scan_resource_group"

VM_OPTIONS="--size Standard_D8ds_v5"
# shellcheck disable=SC3010
if [[ "${ARCHITECTURE,,}" == "arm64" ]]; then
    VM_OPTIONS="--size Standard_D8pds_v5"
fi

if [ "${OS_TYPE}" = "Linux" ] && [ "${ENABLE_TRUSTED_LAUNCH}" = "True" ]; then
    VM_OPTIONS+=" --security-type TrustedLaunch --enable-secure-boot true --enable-vtpm true"
fi

if [ "${OS_TYPE}" = "Linux" ] && grep -q "cvm" <<< "$FEATURE_FLAGS"; then
    # We completely re-assign the VM_OPTIONS string here to ensure that no artifacts from earlier conditionals are included
    VM_OPTIONS="--size Standard_DC8ads_v5 --security-type ConfidentialVM --enable-secure-boot true --enable-vtpm true --os-disk-security-encryption-type VMGuestStateOnly --specialized true"
fi

# GB200 specific VM options for scanning (uses standard ARM64 VM for now)
if [ "${OS_TYPE}" = "Linux" ] && grep -q "GB200" <<< "$FEATURE_FLAGS"; then
    echo "GB200: Using standard ARM64 VM options for scanning"
    # Additional GB200-specific VM options can be added here when GB200 SKUs are available
fi

SCANNING_NIC_ID=$(az network nic create --resource-group $RESOURCE_GROUP_NAME --name "scanning${CURRENT_TIME}${RANDOM}" --subnet $SCANNING_SUBNET_ID | jq -r '.NewNIC.id')
if [ -z "$SCANNING_NIC_ID" ]; then
    echo "unable to create new NIC for scanning VM"
    exit 1
fi

az vm create --resource-group $RESOURCE_GROUP_NAME \
    --name $SCAN_VM_NAME \
    --image $VHD_IMAGE \
    --nics $SCANNING_NIC_ID \
    --admin-username $SCAN_VM_ADMIN_USERNAME \
    --admin-password $SCAN_VM_ADMIN_PASSWORD \
    --os-disk-size-gb 50 \
    ${VM_OPTIONS} \
    --assign-identity "${UMSI_RESOURCE_ID}"
    
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
        "ENABLE_TRUSTED_LAUNCH"=${ENABLE_TRUSTED_LAUNCH} \
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

    # Only 22.04+ are supported
    if [ "$os" = "Ubuntu" ] && { [ "$version" = "18.04" ] || [ "$version" = "20.04" ]; }; then
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

SKIP_CIS=${SKIP_CIS:-true}
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
        current_rule_status=${current_status["$rule_id"]}
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
CIS_REPORT_TXT_NAME="cis-report-${BUILD_ID}-${TIMESTAMP}.txt"
CIS_REPORT_HTML_NAME="cis-report-${BUILD_ID}-${TIMESTAMP}.html"

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
        "CIS_REPORT_TXT_NAME=${CIS_REPORT_TXT_NAME}" \
        "CIS_REPORT_HTML_NAME=${CIS_REPORT_HTML_NAME}" \
        "TEST_VM_ADMIN_USERNAME=${SCAN_VM_ADMIN_USERNAME}" \
        "OS_SKU=${OS_SKU}"
)
echo "$ret"
msg=$(echo -E "$ret" | jq -r '.value[].message')
echo "$msg"

# Download CIS report files to working directory
az storage blob download --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_TXT_NAME}" --file cis-report.txt --account-name "${STORAGE_ACCOUNT_NAME}" --auth-mode login
az storage blob download --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_HTML_NAME}" --file cis-report.html --account-name "${STORAGE_ACCOUNT_NAME}" --auth-mode login

# Remove CIS report blobs from storage
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_TXT_NAME}" --auth-mode login
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CIS_REPORT_HTML_NAME}" --auth-mode login
# Remove CIS assessor tarball blob from storage
az storage blob delete --account-name "${STORAGE_ACCOUNT_NAME}" --container-name "${SIG_CONTAINER_NAME}" --name "${CISASSESSOR_BLOB_NAME}" --auth-mode login

echo -e "CIS Report Script Completed\n\n\n"
capture_benchmark "${SCRIPT_NAME}_cis_report_upload_and_download"

echo -e "Comparing CIS report against baseline"
compare_cis_with_baseline

capture_benchmark "${SCRIPT_NAME}_overall" true
process_benchmarks
