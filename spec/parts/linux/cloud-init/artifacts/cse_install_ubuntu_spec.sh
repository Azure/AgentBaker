#!/bin/bash

Describe 'cse_install_ubuntu.sh'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

    Describe 'cleanUpPrebakedGPUDriver'
        It 'is a no-op when the prebake marker is absent'
            GPU_DKMS_MARKER_FILE="$(mktemp)"; rm -f "${GPU_DKMS_MARKER_FILE}"
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should equal ""
        End

        It 'deregisters the nvidia DKMS module and removes baked artifacts (libs, binaries, marker) when present'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }  # no nvidia module loaded
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "Removing pre-baked NVIDIA driver"
            # deregisters via the DKMS source tree + built module removal (no slow dkms remove)
            The output should include "mock rm -rf /var/lib/dkms/nvidia"
            The output should include "mock rm -f /lib/modules"
            # relocated userspace libs
            The output should include "mock rm -rf /usr/bin/lib64"
            # driver userspace binaries so nvidia-smi becomes "command not found" on non-GPU nodes
            The output should include "mock rm -f /usr/bin/nvidia-smi"
            The output should include "mock ldconfig"
            # the slow per-version dkms remove --all must NOT be on the critical path anymore
            The output should not include "dkms remove"
            # stage-1 observability: a structured outcome line is emitted. Here rm is mocked, so the
            # marker is left in place and the cleanup correctly reports an incomplete (security-gap) result.
            The output should include "AKS_GPU_PREBAKE event=teardown"
            The output should include "status=incomplete"
        End

        It 'reports status=cleaned once the marker and DKMS state are actually gone'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }  # no nvidia module loaded (grid-style prebake)
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "AKS_GPU_PREBAKE event=teardown"
            The output should include "status=cleaned"
            The output should include "marker_after=false"
            # the setuid nvidia-modprobe is part of the security-coverage check
            The output should include "modprobe_after=false"
            The output should include "module_before=false"
            The output should include "module_after=false"
        End

        It 'unloads an idle prebaked nvidia module that auto-loaded at boot (cuda/cuda-lts SKUs)'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            # simulate a loaded-but-idle module: lsmod shows nvidia until rmmod is "run"
            _nvidia_loaded=true
            lsmod() { if [ "${_nvidia_loaded}" = true ]; then echo "nvidia 104165376 0"; else echo ""; fi; }
            cat() { echo "0"; }        # /sys/module/nvidia/refcnt = 0 (idle)
            ls() { return 1; }          # no /dev/nvidia* device nodes
            rmmod() { _nvidia_loaded=false; echo "mock rmmod $*"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "mock rmmod nvidia"
            The output should include "module_before=true"
            The output should include "module_after=false"
            The output should include "status=cleaned"
        End

        It 'keeps the marker (incomplete) when a busy nvidia module cannot be unloaded'
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo "nvidia 104165376 2"; }  # stays loaded (refcnt shows in-use)
            cat() { echo "2"; }                       # refcnt != 0 -> do not rmmod
            ls() { return 1; }
            rmmod() { echo "mock rmmod $*"; }         # should NOT be called
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should not include "mock rmmod"
            The output should include "module_before=true"
            The output should include "module_after=true"
            The output should include "status=incomplete"
        End
    End

    Describe 'AMD ROCm CSE install'
        setup_amd_rocm_test() {
            AMD_ROCM_TEST_DIR="$(mktemp -d)"
            AMD_ROCM_MARKER_FILE="${AMD_ROCM_TEST_DIR}/amd-rocm/version"
            OS="UBUNTU"
            UBUNTU_OS_NAME="UBUNTU"
            UBUNTU_RELEASE="24.04"
            ERR_AMD_ROCM_UNSUPPORTED_OS=244
            ERR_AMD_ROCM_GPG_KEY_DOWNLOAD_TIMEOUT=245
            ERR_AMD_ROCM_INSTALL_TIMEOUT=246
            ERR_AMD_ROCM_VALIDATE_FAIL=247
            ERR_AMD_ROCM_REPO_CONFIG_INVALID=248
        }

        cleanup_amd_rocm_test() {
            /bin/rm -rf "${AMD_ROCM_TEST_DIR:-}"
            unset AMD_ROCM_MARKER_FILE
            unset AMD_ROCM_REPO_BASE_URL
            unset AMD_ROCM_GPG_KEY_URL
            unset AMD_ROCM_APT_PIN_ORIGIN
        }

        BeforeEach 'setup_amd_rocm_test'
        AfterEach 'cleanup_amd_rocm_test'

        It 'accepts the supported MI300X RDMA SKU'
            get_compute_sku() { echo "Standard_ND96isr_MI300X_v5"; }

            When call isAmdRocmSupportedSku

            The status should be success
        End

        It 'accepts the supported MI300X non-RDMA SKU case-insensitively'
            get_compute_sku() { echo "standard_nd96is_mi300x_v5"; }

            When call isAmdRocmSupportedSku

            The status should be success
        End

        It 'rejects unsupported GPU SKUs'
            get_compute_sku() { echo "Standard_NC24ads_A100_v4"; }

            When call isAmdRocmSupportedSku

            The status should be failure
            The output should include "AMD ROCm CSE install is not supported for VM SKU 'Standard_NC24ads_A100_v4'"
        End

        It 'normalizes an HTTPS mirror URL'
            AMD_ROCM_REPO_BASE_URL="https://packages.aks.azure.com/amd-rocm/"

            When call amdRocmRepoBaseUrl

            The status should be success
            The output should equal "https://packages.aks.azure.com/amd-rocm"
        End

        It 'rejects a non-HTTPS package source'
            AMD_ROCM_REPO_BASE_URL="http://example.test/amd-rocm"

            When call amdRocmRepoBaseUrl

            The status should equal 248
            The output should include "AMD_ROCM_REPO_BASE_URL must be an https URL"
        End

        It 'cleans only AMD repository state'
            rm() { echo "rm $*"; }

            When call removeAmdRocmAptRepos

            The status should be success
            The output should include "/etc/apt/sources.list.d/rocm.list"
            The output should include "repo.radeon.com"
            The output should not include "packages.microsoft.com"
            The output should not include "packages.aks.azure.com"
        End

        It 'rejects unsupported Ubuntu versions before repository setup'
            UBUNTU_RELEASE="22.04"
            uname() { echo "6.8.0-1024-azure"; }
            isARM64() { echo 0; }
            getCPUArch() { echo "amd64"; }
            setupAmdRocmAptRepos() { echo "unexpected repo setup"; }

            When run ensureAmdGpuDrivers

            The status should equal 244
            The output should include "only supported on Ubuntu 24.04 amd64"
            The output should not include "unexpected repo setup"
        End

        It 'rejects unsupported SKUs before repository setup'
            uname() { echo "6.8.0-1024-azure"; }
            isARM64() { echo 0; }
            get_compute_sku() { echo "Standard_NC24ads_A100_v4"; }
            setupAmdRocmAptRepos() { echo "unexpected repo setup"; }

            When run ensureAmdGpuDrivers

            The status should equal 244
            The output should include "AMD ROCm CSE install is not supported for VM SKU 'Standard_NC24ads_A100_v4'"
            The output should not include "unexpected repo setup"
        End

        It 'skips installation when an existing driver validates successfully'
            mkdir -p "$(dirname "${AMD_ROCM_MARKER_FILE}")"
            touch "${AMD_ROCM_MARKER_FILE}"
            uname() { echo "6.8.0-1024-azure"; }
            isARM64() { echo 0; }
            isAmdRocmSupportedSku() { return 0; }
            ensureAmdRocmModuleAutoload() { echo "configured module autoload"; }
            validateAmdRocmDriver() { return 0; }
            setupAmdRocmAptRepos() { echo "unexpected repo setup"; }

            When call ensureAmdGpuDrivers

            The status should be success
            The output should include "AMD ROCm driver is already installed and validated"
            The output should not include "unexpected repo setup"
        End

        It 'installs and records the validated driver versions'
            uname() { echo "6.8.0-1024-azure"; }
            isARM64() { echo 0; }
            isAmdRocmSupportedSku() { return 0; }
            ensureAmdRocmModuleAutoload() { echo "configured module autoload"; }
            setupAmdRocmAptRepos() { echo "setup repos $*"; }
            apt_get_install() { echo "install packages $*"; }
            ldconfig() { echo "refresh linker cache"; }
            validateAmdRocmDriver() { echo "validated driver"; return 0; }
            removeAmdRocmAptRepos() { echo "cleanup repos"; }

            When call ensureAmdGpuDrivers

            The status should be success
            The output should include "setup repos 7.2.4 30.30.4"
            The output should include "validated driver"
            The output should include "cleanup repos"
            The path "${AMD_ROCM_MARKER_FILE}" should be file
            The contents of file "${AMD_ROCM_MARKER_FILE}" should include "install_mode=cse"
            The contents of file "${AMD_ROCM_MARKER_FILE}" should include "rocm_version=7.2.4"
            The contents of file "${AMD_ROCM_MARKER_FILE}" should include "kernel=6.8.0-1024-azure"
        End

        It 'cleans repository state and leaves no marker when package installation fails'
            uname() { echo "6.8.0-1024-azure"; }
            isARM64() { echo 0; }
            isAmdRocmSupportedSku() { return 0; }
            ensureAmdRocmModuleAutoload() { return 0; }
            setupAmdRocmAptRepos() { echo "setup repos"; }
            apt_get_install() { echo "package install failed"; return 1; }
            removeAmdRocmAptRepos() { echo "cleanup repos"; }

            When run ensureAmdGpuDrivers

            The status should equal 246
            The output should include "package install failed"
            The output should include "cleanup repos"
            The path "${AMD_ROCM_MARKER_FILE}" should not be exist
        End

        It 'writes no success marker when driver validation fails'
            uname() { echo "6.8.0-1024-azure"; }
            isARM64() { echo 0; }
            isAmdRocmSupportedSku() { return 0; }
            ensureAmdRocmModuleAutoload() { return 0; }
            setupAmdRocmAptRepos() { return 0; }
            apt_get_install() { return 0; }
            ldconfig() { return 0; }
            validateAmdRocmDriver() { echo "driver validation failed"; return 1; }
            removeAmdRocmAptRepos() { echo "cleanup repos"; }

            When run ensureAmdGpuDrivers

            The status should equal 247
            The output should include "driver validation failed"
            The output should include "cleanup repos"
            The path "${AMD_ROCM_MARKER_FILE}" should not be exist
        End
    End
End
