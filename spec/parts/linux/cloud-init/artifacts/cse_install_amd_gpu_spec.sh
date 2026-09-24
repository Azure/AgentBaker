#!/bin/bash

Describe 'dedicated AMD GPU image provisioning'
    Include ./parts/linux/cloud-init/artifacts/cse_amd_gpu.sh
    ERR_AMD_GPU_UNSUPPORTED=244
    ERR_AMD_GPU_VALIDATE_FAIL=245
    OS=UBUNTU
    UBUNTU_OS_NAME=UBUNTU
    OS_VERSION=24.04
    GPU_NODE=false
    get_compute_sku() { echo Standard_ND96isr_MI300X_v5; }
    uname() {
        case "$1" in
            -m) echo x86_64 ;;
            -r) echo 6.17.0-test-azure ;;
        esac
    }

    Describe 'nodePrep opt-in dispatch'
        run_amd_node_prep() {
            # Execute the real dispatch block, restricted to nodePrep so moving
            # it into basePrep (which PIS skips) also fails the enabled test.
            eval "$(awk '
                /^function nodePrep/ { node_prep = 1 }
                node_prep && /if .*AMD_GPU_NODE/ { block = 1 }
                block { print }
                block && /^[[:space:]]*fi$/ { exit }
            ' parts/linux/cloud-init/artifacts/cse_main.sh)"
        }
        source() { test "$1" = /opt/azure/containers/amd-gpu-validate.sh; }
        logs_to_events() { shift; "$@"; }
        ensureAmdGpuDrivers() { echo amd-driver-validated; }

        It 'runs hardware checks in nodePrep with both explicit flags'
            AMD_GPU_NODE=true
            CONFIG_GPU_DRIVER_IF_NEEDED=true
            When run run_amd_node_prep
            The status should be success
            The output should equal amd-driver-validated
        End

        It 'fails the AMD node if its dedicated image validator is missing'
            AMD_GPU_NODE=true
            CONFIG_GPU_DRIVER_IF_NEEDED=true
            source() { return 1; }
            When run run_amd_node_prep
            The status should equal 245
            The output should equal ''
        End

        It 'honors driver configuration opt-out'
            AMD_GPU_NODE=true
            CONFIG_GPU_DRIVER_IF_NEEDED=false
            When run run_amd_node_prep
            The status should be success
            The output should equal ''
        End

        It 'does nothing on existing nodes without AMD enablement'
            unset AMD_GPU_NODE
            CONFIG_GPU_DRIVER_IF_NEEDED=true
            When run run_amd_node_prep
            The status should be success
            The output should equal ''
        End
    End

    Describe 'ensureAmdGpuDrivers'
        validateAmdGpuDriver() { echo driver-validated; }

        It 'validates the AMD image without installing anything'
            When call ensureAmdGpuDrivers
            The status should be success
            The output should equal driver-validated
        End

        It 'rejects another GPU SKU'
            get_compute_sku() { echo Standard_ND96asr_v4; }
            When call ensureAmdGpuDrivers
            The status should equal 244
            The output should include 'Unsupported AMD GPU VM SKU'
        End

        It 'rejects mixed NVIDIA and AMD configuration'
            GPU_NODE=true
            When call ensureAmdGpuDrivers
            The status should equal 244
            The output should include 'exclusive AMD configuration'
        End

        It 'rejects unsupported Ubuntu versions'
            OS_VERSION=22.04
            When call ensureAmdGpuDrivers
            The status should equal 244
            The output should include 'Ubuntu 24.04'
        End

        It 'fails closed when the baked driver is missing or invalid'
            validateAmdGpuDriver() { return 1; }
            When call ensureAmdGpuDrivers
            The status should equal 245
            The output should include 'use a qualified AMD GPU VHD'
        End
    End

    Describe 'validateAmdGpuDriver and active devices'
        setup() {
            AMD_TEST_ROOT=$(mktemp -d)
            export AMD_TEST_ROOT
            mkdir -p "${AMD_TEST_ROOT}/nodes" "${AMD_TEST_ROOT}/module"
            echo 7.1.3.test > "${AMD_TEST_ROOT}/module/version"
            echo '{"schema_version":1,"package_version":"1:driver-test","firmware_package_version":"firmware-test","module_version":"7.1.3.test","kernel_version":"older-bake-kernel"}' > "${AMD_TEST_ROOT}/driver.json"
            local i
            for i in {0..8}; do
                mkdir -p "${AMD_TEST_ROOT}/nodes/${i}"
                echo "${i}" > "${AMD_TEST_ROOT}/nodes/${i}/gpu_id"
            done
            eval "$(sed -n '/^validateAmdGpuDriver()/,/^}/p; /^validateAmdGpuDevices()/,/^}/p' parts/linux/cloud-init/artifacts/cse_amd_gpu.sh |
                sed "s|/opt/azure/amd-gpu|${AMD_TEST_ROOT}|g; s|/sys/module/amdgpu|${AMD_TEST_ROOT}/module|g; s|/sys/class/kfd/kfd/topology/nodes|${AMD_TEST_ROOT}/nodes|g")"
        }
        cleanup() { rm -rf "${AMD_TEST_ROOT}"; }
        BeforeEach setup
        AfterEach cleanup

        dpkg-query() {
            case "${*: -1}" in
                amdgpu-dkms) echo 'install ok installed 1:driver-test' ;;
                amdgpu-dkms-firmware) echo 'install ok installed firmware-test' ;;
            esac
        }
        modinfo() {
            case "$4" in
                filename) echo /lib/modules/6.17.0-test-azure/updates/dkms/amdgpu.ko.zst ;;
                version) echo 7.1.3.test ;;
            esac
        }
        retrycmd_if_failure() {
            shift 3
            case "$1" in
                modprobe) return 0 ;;
                test) return 0 ;;
                *) "$@" ;;
            esac
        }

        It 'accepts a rebuilt pinned driver on a newer kernel and ignores the CPU KFD node'
            When call validateAmdGpuDriver
            The status should be success
            The output should include 'eight GPUs detected'
        End

        It 'rejects the inbox module'
            modinfo() { echo /lib/modules/6.17.0-test-azure/kernel/drivers/gpu/drm/amd/amdgpu/amdgpu.ko.zst; }
            When call validateAmdGpuDriver
            The status should be failure
            The output should equal ''
        End

        It 'rejects a different already-loaded module'
            echo old-driver > "${AMD_TEST_ROOT}/module/version"
            When call validateAmdGpuDriver
            The status should be failure
            The output should equal ''
        End

        It 'rejects removed packages with leftover dpkg records'
            dpkg-query() { echo 'deinstall ok config-files 1:driver-test'; }
            When call validateAmdGpuDriver
            The status should be failure
            The output should equal ''
        End

        It 'rejects an unknown marker format'
            echo '{"schema_version":2}' > "${AMD_TEST_ROOT}/driver.json"
            When call validateAmdGpuDriver
            The status should be failure
            The output should equal ''
        End

        It 'rejects missing GPUs'
            echo 0 > "${AMD_TEST_ROOT}/nodes/8/gpu_id"
            When call validateAmdGpuDriver
            The status should be failure
            The output should equal ''
        End

        It 'rejects malformed GPU topology rather than counting it as a GPU'
            echo invalid > "${AMD_TEST_ROOT}/nodes/8/gpu_id"
            When call validateAmdGpuDevices
            The status should be failure
        End
    End
End
