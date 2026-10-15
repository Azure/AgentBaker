#!/bin/bash

Describe 'publish-generalized-cvm-if-needed'
  SCRIPT="./vhdbuilder/packer/publish-generalized-cvm-if-needed.sh"
  ORIGINAL_PATH="${PATH}"

  setup() {
    TEST_ROOT="$(mktemp -d)"
    AZ_CALLS="${TEST_ROOT}/az-calls"
    AZ_VERSION_STATE_FILE="${TEST_ROOT}/version-state"
    touch "${AZ_CALLS}"

    export AZ_CALLS
    export AZ_VERSION_STATE_FILE
    export MOCK_DEFINITION_EXISTS=false
    export MOCK_CREATED_VERSION_STATE=Succeeded

    export OS_SKU=Ubuntu
    export FEATURE_FLAGS=cvm
    export AZURE_RESOURCE_GROUP_NAME=resource-group
    export SIG_GALLERY_NAME=gallery
    export SKU_NAME=2404gen2CVMcontainerd
    export CAPTURED_SIG_VERSION=202610.15.0
    export PACKER_BUILD_LOCATION=eastus
    export BUILD_BUILDID=12345

    cat >"${TEST_ROOT}/az" <<'EOF'
#!/usr/bin/env bash

printf '<%s>\n' "$*" >>"${AZ_CALLS}"

case "$1 $2" in
    "sig image-definition")
        case "$3" in
            show)
                if [[ "$*" == *"--query id"* ]]; then
                    echo "/subscriptions/test/resourceGroups/resource-group/providers/Microsoft.Compute/galleries/gallery/images/2404gen2CVMcontainerd"
                    exit 0
                fi
                if [[ "${MOCK_DEFINITION_EXISTS}" == "true" ]]; then
                    echo '{}'
                    exit 0
                fi
                exit 1
                ;;
            create)
                exit 0
                ;;
        esac
        ;;
    "sig image-version")
        case "$3" in
            show)
                if [[ -s "${AZ_VERSION_STATE_FILE}" ]]; then
                    cat "${AZ_VERSION_STATE_FILE}"
                    exit 0
                fi
                exit 1
                ;;
            create)
                printf '%s' "${MOCK_CREATED_VERSION_STATE}" >"${AZ_VERSION_STATE_FILE}"
                exit 0
                ;;
            delete)
                rm -f "${AZ_VERSION_STATE_FILE}"
                exit 0
                ;;
            wait)
                exit 0
                ;;
        esac
        ;;
    "image create")
        echo "/subscriptions/test/resourceGroups/resource-group/providers/Microsoft.Compute/images/temporary-image"
        exit 0
        ;;
    "image show")
        exit 1
        ;;
    "image delete"|"resource update")
        exit 0
        ;;
esac

echo "Unexpected az invocation: $*" >&2
exit 1
EOF

    cat >"${TEST_ROOT}/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

    chmod +x "${TEST_ROOT}/az" "${TEST_ROOT}/sleep"
    export PATH="${TEST_ROOT}:${ORIGINAL_PATH}"
  }

  cleanup() {
    PATH="${ORIGINAL_PATH}"
    rm -rf "${TEST_ROOT}"
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'does nothing for a non-CVM build'
    FEATURE_FLAGS=""

    When run "${SCRIPT}" "https://storage.example/vhd.vhd"
    The status should be success
    The output should include "Generalized CVM publication is not required"
    The contents of file "${AZ_CALLS}" should equal ""
  End

  It 'does nothing for Azure Container Linux CVM'
    OS_SKU=AzureContainerLinux

    When run "${SCRIPT}" "https://storage.example/vhd.vhd"
    The status should be success
    The output should include "Generalized CVM publication is not required"
    The contents of file "${AZ_CALLS}" should equal ""
  End

  It 'publishes an Ubuntu CVM staging VHD'
    When run "${SCRIPT}" "https://storage.example/ubuntu.vhd"
    The status should be success
    The output should include "provisioned successfully"
    The contents of file "${AZ_CALLS}" should include "<sig image-definition create"
    The contents of file "${AZ_CALLS}" should include "--os-state Generalized"
    The contents of file "${AZ_CALLS}" should include "SecurityType=ConfidentialVMSupported"
    The contents of file "${AZ_CALLS}" should include "<image create"
    The contents of file "${AZ_CALLS}" should include "--source https://storage.example/ubuntu.vhd"
    The contents of file "${AZ_CALLS}" should include "<sig image-version create"
    The contents of file "${AZ_CALLS}" should include "--target-regions eastus westeurope"
  End

  It 'publishes an Azure Linux CVM staging VHD'
    OS_SKU=AzureLinux
    SKU_NAME=V3gen2CVM

    When run "${SCRIPT}" "https://storage.example/azurelinux.vhd"
    The status should be success
    The output should include "V3gen2CVM/202610.15.0 provisioned successfully"
    The contents of file "${AZ_CALLS}" should include "--gallery-image-definition V3gen2CVM"
    The contents of file "${AZ_CALLS}" should include "--source https://storage.example/azurelinux.vhd"
  End

  It 'reuses an existing successfully provisioned version'
    MOCK_DEFINITION_EXISTS=true
    printf '%s' "Succeeded" >"${AZ_VERSION_STATE_FILE}"

    When run "${SCRIPT}" "https://storage.example/ubuntu.vhd"
    The status should be success
    The output should include "is already provisioned"
    The contents of file "${AZ_CALLS}" should not include "<image create"
    The contents of file "${AZ_CALLS}" should not include "<sig image-version create"
  End

  It 'deletes and replaces a failed image version'
    MOCK_DEFINITION_EXISTS=true
    printf '%s' "Failed" >"${AZ_VERSION_STATE_FILE}"

    When run "${SCRIPT}" "https://storage.example/ubuntu.vhd"
    The status should be success
    The output should include "Deleting failed image version"
    The contents of file "${AZ_CALLS}" should include "<sig image-version delete"
    The contents of file "${AZ_CALLS}" should include "<sig image-version wait"
    The contents of file "${AZ_CALLS}" should include "<image create"
    The contents of file "${AZ_CALLS}" should include "<sig image-version create"
  End
End
