#!/bin/bash

# Run in a disposable Linux container with DKMS installed, or set DKMS_TEST_BIN
# to a DKMS script. All driver state is a fixture; no NVIDIA module is compiled.
Describe 'CUDA prebake with real DKMS'
    Include ./parts/linux/cloud-init/artifacts/cse_config_gpu.sh
    DKMS_TEST_BIN="${DKMS_TEST_BIN:-$(command -v dkms || true)}"
    Skip if 'requires root in a disposable Linux test environment' [ "$(id -u)" -ne 0 ]
    Skip if 'requires DKMS or DKMS_TEST_BIN' [ ! -r "${DKMS_TEST_BIN}" ]

    setup() {
        TEST_DIR="$(mktemp -d)"
        DRIVER_VERSION=580.126.09
        RUNNING_KERNEL=6.8.0-prebaked
        DKMS_TREE="${TEST_DIR}/dkms"
        MODULE_TREE="${TEST_DIR}/modules"
        SOURCE_TREE="${TEST_DIR}/sources"
        mkdir -p "${DKMS_TREE}" "${SOURCE_TREE}/nvidia-${DRIVER_VERSION}" \
            "${TEST_DIR}/headers/include" "${MODULE_TREE}/${RUNNING_KERNEL}/updates/dkms"
        cat > "${SOURCE_TREE}/nvidia-${DRIVER_VERSION}/dkms.conf" <<EOF
PACKAGE_NAME="nvidia"
PACKAGE_VERSION="${DRIVER_VERSION}"
AUTOINSTALL="yes"
BUILT_MODULE_NAME[0]="nvidia"
DEST_MODULE_LOCATION[0]="/updates/dkms"
MAKE[0]="touch '${TEST_DIR}/build-attempted'; false"
CLEAN="true"
EOF
        echo '/* fixture source: no real module is compiled */' > "${SOURCE_TREE}/nvidia-${DRIVER_VERSION}/nvidia.c"
        echo 'precompiled-module-fixture' > "${MODULE_TREE}/${RUNNING_KERNEL}/updates/dkms/nvidia.ko"
    }
    cleanup() { rm -rf "${TEST_DIR}"; }
    BeforeEach setup
    AfterEach cleanup

    uname() { case "$1" in -r) echo "${RUNNING_KERNEL}" ;; -m) echo x86_64 ;; esac; }
    modinfo() { echo "${DRIVER_VERSION}"; }
    dkms() {
        bash "${DKMS_TEST_BIN}" --dkmstree "${DKMS_TREE}" \
            --sourcetree "${SOURCE_TREE}" --installtree "${MODULE_TREE}" \
            --kernelsourcedir "${TEST_DIR}/headers" "$@"
    }

    # Create the files and links that DKMS status uses for an installed record.
    # This verifies status parsing/discovery, not a real NVIDIA installation.
    installed_fixture() {
        dkms add -m nvidia -v "${DRIVER_VERSION}" > "${TEST_DIR}/add.log" 2>&1 || return 1
        local built="${DKMS_TREE}/nvidia/${DRIVER_VERSION}/${RUNNING_KERNEL}/x86_64"
        mkdir -p "${built}/module"
        cp "${MODULE_TREE}/${RUNNING_KERNEL}/updates/dkms/nvidia.ko" "${built}/module/nvidia.ko"
        ln -s "${built}" "${DKMS_TREE}/nvidia/kernel-${RUNNING_KERNEL}-x86_64"
    }

    failed_cleanup_then_update() {
        cleanup_driver() { return 1; }
        cleanup_driver || true
        dkms autoinstall -k 6.8.0-security-update
    }

    It 'does not select NVIDIA after failed CPU cleanup even when module and source files remain'
        When call failed_cleanup_then_update
        The status should be success
        The output should not include 'nvidia'
        The path "${DKMS_TREE}/nvidia" should not be exist
        The path "${TEST_DIR}/build-attempted" should not be exist
        The path "${MODULE_TREE}/${RUNNING_KERNEL}/updates/dkms/nvidia.ko" should be file
    End

    It 'rejects a source-only added record'
        dkms add -m nvidia -v "${DRIVER_VERSION}" > "${TEST_DIR}/add.log" 2>&1
        When call isNvidiaDKMSInstalledForCurrentKernel
        The status should be failure
        The output should equal ''
        The path "${TEST_DIR}/build-attempted" should not be exist
    End

    It 'accepts an installed record for the current kernel'
        installed_fixture
        When call isNvidiaDKMSInstalledForCurrentKernel
        The status should be success
        The output should equal ''
    End

    It 'accepts an installed record with a backup of the original module'
        installed_fixture
        mkdir -p "${DKMS_TREE}/nvidia/original_module/${RUNNING_KERNEL}/x86_64"
        When call isNvidiaDKMSInstalledForCurrentKernel
        The status should be success
        The output should equal ''
    End

    It 'rejects installation for another kernel'
        installed_fixture
        RUNNING_KERNEL=6.8.0-security-update
        When call isNvidiaDKMSInstalledForCurrentKernel
        The status should be failure
        The output should equal ''
    End

    It 'rejects a mismatch between the DKMS cache and installed module'
        installed_fixture
        echo 'different-module' > "${MODULE_TREE}/${RUNNING_KERNEL}/updates/dkms/nvidia.ko"
        When call isNvidiaDKMSInstalledForCurrentKernel
        The status should be failure
        The output should equal ''
    End

    It 'does not build again for the already-installed kernel'
        installed_fixture
        When call dkms autoinstall -k "${RUNNING_KERNEL}"
        The status should be success
        The output should not include 'Building module'
        The path "${TEST_DIR}/build-attempted" should not be exist
    End

    attempt_kernel_update() {
        # The fixture deliberately fails compilation. DKMS versions differ in
        # their exit status here; the build sentinel is the assertion.
        dkms autoinstall -k 6.8.0-security-update > "${TEST_DIR}/update.log" 2>&1 || true
    }

    It 'selects NVIDIA for a new kernel after full installed state exists'
        installed_fixture
        When call attempt_kernel_update
        The status should be success
        The output should equal ''
        The path "${TEST_DIR}/build-attempted" should be file
    End
End
