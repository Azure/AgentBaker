#!/bin/bash
set -eux

# Parameters (passed as environment variables or arguments)
CISASSESSOR_TARBALL_PATH="/tmp/cisassessor.tar.gz"
CISASSESSOR_BLOB_NAME=${CISASSESSOR_BLOB_NAME:-""}
STORAGE_ACCOUNT_NAME=${STORAGE_ACCOUNT_NAME:-""}
SIG_CONTAINER_NAME=${SIG_CONTAINER_NAME:-""}
AZURE_MSI_RESOURCE_STRING=${AZURE_MSI_RESOURCE_STRING:-""}
ENABLE_TRUSTED_LAUNCH=${ENABLE_TRUSTED_LAUNCH:-""}
CIS_REPORT_L1_TXT_NAME=${CIS_REPORT_L1_TXT_NAME:-"cis-report-l1.txt"}
CIS_REPORT_L2_TXT_NAME=${CIS_REPORT_L2_TXT_NAME:-"cis-report-l2.txt"}
CIS_REPORT_HTML_NAME=${CIS_REPORT_HTML_NAME:-"cis-report.html"}
OS_SKU=${OS_SKU:-""}
TEST_VM_ADMIN_USERNAME=${TEST_VM_ADMIN_USERNAME:-"azureuser"}

if [ "$OS_SKU" = "Flatcar" ]; then
    # The venv with azure-cli is created in trivy-scan.sh but PATH changes are
    # not preserved across scripts.
    export PATH="/home/$TEST_VM_ADMIN_USERNAME/venv/bin:$PATH"
fi

# Azure login helper
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
login_with_umsi_resource_id() {
    login_with_user_assigned_managed_identity "--resource-id" "$1"
}

# Main logic - OS check is now done in vhd-scanning.sh before calling this script

# Login to Azure before blob download
if [ -n "$AZURE_MSI_RESOURCE_STRING" ]; then
    login_with_umsi_resource_id "$AZURE_MSI_RESOURCE_STRING"
else
    echo "AZURE_MSI_RESOURCE_STRING must be set for az login"
    exit 1
fi

# Fetch cisassessor tarball from storage account
az storage blob download --container-name "$SIG_CONTAINER_NAME" --name "$CISASSESSOR_BLOB_NAME" --file "$CISASSESSOR_TARBALL_PATH" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login

if [ ! -f "$CISASSESSOR_TARBALL_PATH" ]; then
    echo "CIS assessor tarball not found at $CISASSESSOR_TARBALL_PATH"
    exit 1
fi
pushd "$(dirname "$CISASSESSOR_TARBALL_PATH")" || exit 1

# Disable GuestConfig agent to avoid interference with CIS checks
systemctl disable --now gcd.service || true
# Fix permissions of log files
find /var/log -type f -exec chmod 640 {} \;

tar xzf "$CISASSESSOR_TARBALL_PATH"

# Run L1 and L2 and upload both text reports. L2 HTML is used to assist in fixing issues.
REPORT_DIR="cisassessor/lib/app/reports"
latest_report() {
    local pattern="$1"
    find "$REPORT_DIR" -name "$pattern" -printf '%T@ %p\n' | sort -n | tail -n1 | cut -d' ' -f2-
}

LEVEL=1 cisassessor/launch-cis.sh
L1_TXT_REPORT=$(latest_report "*.txt")
if [ -z "$L1_TXT_REPORT" ] || [ ! -f "$L1_TXT_REPORT" ]; then
    echo "No CIS L1 text report found in ${REPORT_DIR}"
    exit 1
fi

LEVEL=2 cisassessor/launch-cis.sh
L2_TXT_REPORT=$(latest_report "*.txt")
if [ -z "$L2_TXT_REPORT" ] || [ ! -f "$L2_TXT_REPORT" ]; then
    echo "No CIS L2 text report found in ${REPORT_DIR}"
    exit 1
fi
L2_HTML_REPORT=$(latest_report "*.html")
if [ -z "$L2_HTML_REPORT" ] || [ ! -f "$L2_HTML_REPORT" ]; then
    echo "No CIS L2 HTML report found in ${REPORT_DIR}"
    exit 1
fi

# Upload reports to storage account
az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$L1_TXT_REPORT" --name "$CIS_REPORT_L1_TXT_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$L2_TXT_REPORT" --name "$CIS_REPORT_L2_TXT_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login
az storage blob upload --container-name "$SIG_CONTAINER_NAME" --file "$L2_HTML_REPORT" --name "$CIS_REPORT_HTML_NAME" --account-name "$STORAGE_ACCOUNT_NAME" --auth-mode login

popd || exit 1
