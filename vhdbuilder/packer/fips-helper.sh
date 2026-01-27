#!/bin/bash
# FIPS Helper Functions for VHD Scanning

# FIPS 140-3 encryption is not automatically supported in Linux VMs.
# Because not all extensions are onboarded to FIPS 140-3 yet, subscriptions must register the Microsoft.Compute/OptInToFips1403Compliance feature.
# After registering the feature, the VM must be created via Azure REST API calls to enable support for FIPS 140-3.
# There is currently no ETA for when FIPS 140-3 encryption is natively supported, but all information can be found here: https://learn.microsoft.com/en-us/azure/virtual-machines/extensions/agent-linux-fips

# This script contains functions related to FIPS 140-3 compliance for Ubuntu 22.04

# Function to ensure FIPS 140-3 compliance feature is registered
ensure_fips_feature_registered() {
    echo "Detected Ubuntu 22.04 + FIPS scenario, enabling FIPS 140-3 compliance..."

    # Enable FIPS 140-3 compliance feature if not already enabled
    echo "Checking FIPS 140-3 compliance feature registration..."
    FIPS_FEATURE_STATE=$(az feature show --namespace Microsoft.Compute --name OptInToFips1403Compliance --query 'properties.state' -o tsv 2>/dev/null || echo "NotRegistered")

    if [ "$FIPS_FEATURE_STATE" != "Registered" ]; then
        echo "Registering FIPS 140-3 compliance feature..."
        az feature register --namespace Microsoft.Compute --name OptInToFips1403Compliance
        local az_register_exit_code=$?
        if [ "$az_register_exit_code" -ne 0 ]; then
            echo "Error: Failed to register FIPS 140-3 compliance feature (exit code: $az_register_exit_code)" >&2
            return "$az_register_exit_code"
        fi

        # Poll until registered (timeout after 5 minutes)
        local TIMEOUT=300
        local ELAPSED=0
        while [ "$FIPS_FEATURE_STATE" != "Registered" ] && [ $ELAPSED -lt $TIMEOUT ]; do
            sleep 10
            ELAPSED=$((ELAPSED + 10))
            FIPS_FEATURE_STATE=$(az feature show --namespace Microsoft.Compute --name OptInToFips1403Compliance --query 'properties.state' -o tsv)
            echo "Feature state: $FIPS_FEATURE_STATE (waited ${ELAPSED}s)"
        done

        if [ "$FIPS_FEATURE_STATE" != "Registered" ]; then
            echo "Error: FIPS 140-3 feature registration timed out after ${TIMEOUT}s" >&2
            return 1
        fi

        echo "FIPS 140-3 feature registered successfully. Refreshing provider..."
        az provider register -n Microsoft.Compute
    else
        echo "FIPS 140-3 compliance feature already registered"
    fi
}

# Function to build FIPS-enabled VM request body
build_fips_vm_body() {
    local location="$1"
    local vm_name="$2"
    local admin_username="$3"
    local admin_password="$4"
    local image_id="$5"
    local nic_id="$6"
    local umsi_resource_id="$7"
    local vm_size="$8"

    cat <<EOF
{
  "location": "$location",
  "identity": {
    "type": "UserAssigned",
    "userAssignedIdentities": {
      "$umsi_resource_id": {}
    }
  },
  "properties": {
    "additionalCapabilities": {
      "enableFips1403Encryption": true
    },
    "hardwareProfile": {
      "vmSize": "$vm_size"
    },
    "osProfile": {
      "computerName": "$vm_name",
      "adminUsername": "$admin_username",
      "adminPassword": "$admin_password"
    },
    "storageProfile": {
      "imageReference": {
        "id": "$image_id"
      },
      "osDisk": {
        "createOption": "FromImage",
        "diskSizeGB": 50,
        "managedDisk": {
          "storageAccountType": "Premium_LRS"
        }
      }
    },
    "networkProfile": {
      "networkInterfaces": [
        {
          "id": "$nic_id"
        }
      ]
    }
  }
}
EOF
}

# Function to create FIPS-enabled VM using REST API
create_fips_vm() {
    local vm_size="$1"
    echo "Creating VM with FIPS 140-3 encryption using REST API..."

    # Disable tracing to prevent password from appearing in logs
    set +x
    # Build the VM request body for FIPS scenario
    local VM_BODY=$(build_fips_vm_body \
        "$PACKER_BUILD_LOCATION" \
        "$SCAN_VM_NAME" \
        "$SCAN_VM_ADMIN_USERNAME" \
        "$SCAN_VM_ADMIN_PASSWORD" \
        "$VHD_IMAGE" \
        "$SCANNING_NIC_ID" \
        "$UMSI_RESOURCE_ID" \
        "$vm_size")

    # Create the VM using REST API
    az rest \
        --method put \
        --url "https://management.azure.com/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP_NAME}/providers/Microsoft.Compute/virtualMachines/${SCAN_VM_NAME}?api-version=2024-11-01" \
        --body "$VM_BODY"

    # Check for errors in the REST API call
    local az_rest_exit_code=$?
    # Re-enable tracing after sensitive command
    set -x
    if [ "$az_rest_exit_code" -ne 0 ]; then
        echo "Error: Failed to create VM with FIPS 140-3 encryption via REST API (exit code: $az_rest_exit_code)" >&2
        return "$az_rest_exit_code"
    fi

    # Wait for VM to be ready (timeout after 10 minutes)
    echo "Waiting for VM to be ready..."
    az vm wait --created --name $SCAN_VM_NAME --resource-group $RESOURCE_GROUP_NAME --timeout 600

    # Check for errors in the az wait command
    local az_wait_exit_code=$?
    if [ "$az_wait_exit_code" -ne 0 ]; then
        echo "Error: Failed to await VM readiness (exit code: $az_wait_exit_code)" >&2
        return "$az_wait_exit_code"
    fi
}
