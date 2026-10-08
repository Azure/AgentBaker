#!/bin/bash

# Mock functions that the ACL script depends on
oras() {
    echo "mock oras $*" >&2
}

ln() {
    echo "mock ln $*" >&2
}

systemd-sysext() {
    echo "mock systemd-sysext $*" >&2
}

timeout() {
    shift # remove timeout duration
    "$@" # execute the command
}

mkdir() {
    echo "mock mkdir $*" >&2
}

getSystemdArch() {
    echo "x86-64"
}

getCPUArch() {
    echo "amd64"
}

sleep() {
    echo "sleeping $1 seconds" >&2
}

find() {
    echo "mock find $*" >&2
}

CSE_STARTTIME_SECONDS=$(date +%s)

Describe 'cse_install_acl.sh'
    Include "./parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh"
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    Describe 'installKubeletKubectlFromPkg'
        It 'masks the kubelet sysext Upholds drop-in before activating the sysext'
            MASK_CREATED=false
            ln() {
                if [ "$1" = "-sfn" ] && [ "$2" = "/dev/null" ] && [ "$3" = "/etc/systemd/system/multi-user.target.d/10-kubelet-kubelet.conf" ]; then
                    MASK_CREATED=true
                fi
                echo "mock ln $*" >&2
            }
            mergeSysexts() {
                if [ "$MASK_CREATED" != "true" ]; then
                    echo "mergeSysexts called before the kubelet sysext Upholds drop-in was masked" >&2
                    return 1
                fi
                echo "mock mergeSysexts $*" >&2
            }
            When call installKubeletKubectlFromPkg "1.33"
            The error should include "mock mkdir -p /etc/systemd/system/multi-user.target.d"
            The error should include "mock ln -sfn /dev/null /etc/systemd/system/multi-user.target.d/10-kubelet-kubelet.conf"
            The error should include "mock mergeSysexts kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext 1.33 kubectl mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext 1.33"
            The error should include "mock ln -snf /usr/bin/kubelet /usr/bin/kubectl /opt/bin/"
            The error should not include "mergeSysexts called before"
            The status should be success
        End

        It 'fails installation when the sysext Upholds drop-in cannot be masked'
            mkdir() {
                return 1
            }
            mergeSysexts() {
                echo "unexpected mergeSysexts" >&2
            }
            installKubeletKubectlFromURL() {
                echo "unexpected installKubeletKubectlFromURL" >&2
            }
            When run installKubeletKubectlFromPkg "1.33"
            The error should include "Failed to create kubelet sysext systemd drop-in directory /etc/systemd/system/multi-user.target.d"
            The error should not include "unexpected mergeSysexts"
            The error should not include "unexpected installKubeletKubectlFromURL"
            The status should equal "$ERR_K8S_INSTALL_ERR"
        End
    End

    Describe 'installSecureTLSBootstrapClientSysext'
        It 'calls mergeSysexts with correct URL and creates symlink on success'
            mergeSysexts() {
                echo "mock mergeSysexts $*" >&2
            }
            ln() {
                echo "mock ln $*" >&2
            }
            When call installSecureTLSBootstrapClientSysext "1.1.3"
            The error should include "mock mergeSysexts aks-secure-tls-bootstrap-client mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext 1.1.3"
            The error should include "mock ln -snf /usr/bin/aks-secure-tls-bootstrap-client /opt/bin/aks-secure-tls-bootstrap-client"
            The status should be success
        End

        It 'uses custom registry when provided'
            mergeSysexts() {
                echo "mock mergeSysexts $*" >&2
            }
            ln() {
                echo "mock ln $*" >&2
            }
            When call installSecureTLSBootstrapClientSysext "1.1.3" "custom.registry.io"
            The error should include "mock mergeSysexts aks-secure-tls-bootstrap-client custom.registry.io/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext 1.1.3"
            The status should be success
        End

        It 'returns ERR_ORAS_PULL_SYSEXT_FAIL when mergeSysexts fails'
            mergeSysexts() {
                return 1
            }
            ERR_ORAS_PULL_SYSEXT_FAIL=231
            When call installSecureTLSBootstrapClientSysext "1.1.3"
            The output should include "Failed to install aks-secure-tls-bootstrap-client sysext"
            The status should be failure
        End

        It 'strips a leading v from the version before passing to mergeSysexts'
            mergeSysexts() {
                echo "mock mergeSysexts $*" >&2
            }
            ln() {
                echo "mock ln $*" >&2
            }
            When call installSecureTLSBootstrapClientSysext "v1.1.3-2-azlinux3"
            The error should include "mock mergeSysexts aks-secure-tls-bootstrap-client mcr.microsoft.com/aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext 1.1.3-2-azlinux3"
            The error should not include "vv1.1.3"
            The status should be success
        End
    End

    Describe 'installACLSysext'
        getACLVersionID() { echo "3.0.20260809"; }
        mergeSysexts() {
            echo "mergeSysexts $*"
            return "${MERGE_RC:-0}"
        }

        It 'resolves the streaming extension against the booted ACL version'
            MCR_REPOSITORY_BASE="mcr.microsoft.com"
            When call installACLSysext artifact-streaming amd64
            The output should equal "mergeSysexts artifact-streaming mcr.microsoft.com/azurelinux/3.0/azure-container-linux/artifact-streaming 3.0.20260809-amd64"
            The status should be success
        End

        It 'selects the ARM64 streaming artifact instead of the AMD64 bare tag'
            MCR_REPOSITORY_BASE="mcr.microsoft.com"
            When call installACLSysext artifact-streaming arm64
            The output should equal "mergeSysexts artifact-streaming mcr.microsoft.com/azurelinux/3.0/azure-container-linux/artifact-streaming 3.0.20260809-arm64"
            The status should be success
        End

        It 'preserves the legacy GPU tag when no architecture suffix is requested'
            MCR_REPOSITORY_BASE="mcr.microsoft.com"
            When call installACLSysext nvidia-driver-vgpu
            The output should equal "mergeSysexts nvidia-driver-vgpu mcr.microsoft.com/azurelinux/3.0/azure-container-linux/nvidia-driver-vgpu 3.0.20260809"
            The status should be success
        End

        It 'preserves the configured registry'
            MCR_REPOSITORY_BASE="mirror.example.test/"
            When call installACLSysext artifact-streaming
            The output should equal "mergeSysexts artifact-streaming mirror.example.test/azurelinux/3.0/azure-container-linux/artifact-streaming 3.0.20260809"
            The status should be success
        End

        It 'returns the sysext error instead of enabling missing services'
            MERGE_RC=1
            MCR_REPOSITORY_BASE="mcr.microsoft.com"
            When call installACLSysext artifact-streaming
            The output should include "mergeSysexts artifact-streaming"
            The status should equal "$ERR_ORAS_PULL_SYSEXT_FAIL"
        End

        It 'rejects a missing ACL version before fetching an extension'
            getACLVersionID() { return "$ERR_SYSEXT_VERSION_ID_NOT_FOUND"; }
            When call installACLSysext artifact-streaming
            The output should equal ""
            The status should equal "$ERR_SYSEXT_VERSION_ID_NOT_FOUND"
        End
    End

    Describe 'matchLocalSysext OEM cache'
        setup_local_cache() {
            OPT_CACHE_FILE=$(mktemp)
            VERSIONED_MATCH=""
            LOCAL_MATCH=""
            OEM_CACHE_STATUS=0
        }
        cleanup_local_cache() { rm -f "${OPT_CACHE_FILE}"; }
        BeforeEach 'setup_local_cache'
        AfterEach 'cleanup_local_cache'
        find() {
            if [[ "${5}" == artifact-streaming-v* ]]; then
                printf '%s\n' "${VERSIONED_MATCH}"
            else
                printf '%s\n' "${LOCAL_MATCH}"
            fi
        }
        test() {
            if [[ "$#" -eq 2 && "$1" == "-f" && "$2" == "/oem/aks-sysext-cache/artifact-streaming.raw" ]]; then
                return "${OEM_CACHE_STATUS}"
            fi
            builtin test "$@"
        }

        It 'preserves the versioned opt cache as the first choice'
            VERSIONED_MATCH="${OPT_CACHE_FILE}"
            When call matchLocalSysext artifact-streaming 3.0.20261008-amd64 x86-64
            The output should equal "${OPT_CACHE_FILE}"
            The status should be success
        End

        It 'preserves the plain opt cache ahead of the OEM cache'
            LOCAL_MATCH="${OPT_CACHE_FILE}"
            When call matchLocalSysext artifact-streaming 3.0.20261008-amd64 x86-64
            The output should equal "${OPT_CACHE_FILE}"
            The status should be success
        End

        It 'finds the preloaded OEM payload when the opt cache is absent'
            When call matchLocalSysext artifact-streaming 3.0.20261008-amd64 x86-64
            The output should equal "/oem/aks-sysext-cache/artifact-streaming.raw"
            The status should be success
        End

        It 'preserves remote lookup when neither cache has a payload'
            OEM_CACHE_STATUS=1
            When call matchLocalSysext artifact-streaming 3.0.20261008-amd64 x86-64
            The output should equal ""
            The status should be success
        End
    End

    Describe 'matchRemoteSysext published streaming tags'
        BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER=""
        retrycmd_silent() {
            printf '%s\n' 3.0.20261007 3.0.20261007-amd64 3.0.20261007-arm64
        }

        It 'resolves the published AMD64 tag'
            When call matchRemoteSysext example.test/artifact-streaming 3.0.20261007-amd64 x86-64
            The output should equal "3.0.20261007-amd64"
            The status should be success
        End

        It 'resolves the published ARM64 tag rather than the bare AMD64 manifest'
            When call matchRemoteSysext example.test/artifact-streaming 3.0.20261007-arm64 arm64
            The output should equal "3.0.20261007-arm64"
            The status should be success
        End
    End

    Describe 'installACLGPUSysext compatibility'
        installACLSysext() {
            echo "installACLSysext $*"
            return "${INSTALL_RC:-0}"
        }

        It 'preserves successful GPU installation'
            When run installACLGPUSysext nvidia-driver-vgpu
            The output should equal "installACLSysext nvidia-driver-vgpu"
            The status should be success
        End

        It 'preserves the original fatal GPU installation error'
            INSTALL_RC=231
            When run installACLGPUSysext nvidia-driver-vgpu
            The output should equal "installACLSysext nvidia-driver-vgpu"
            The status should equal 231
        End
    End

    Describe 'installArtifactStreamingSysext'
        getCPUArch() { echo "${TEST_ARCH:-amd64}"; }
        setup_streaming_install() {
            TEST_STREAMING_DIR="$(mktemp -d)"
            ACR_OVERLAYBD_INSTALL_SCRIPT="${TEST_STREAMING_DIR}/install.sh"
            printf '#!/bin/sh\necho "prepare overlaybd"\nexit "${COMPAT_RC:-0}"\n' > "${ACR_OVERLAYBD_INSTALL_SCRIPT}"
            chmod +x "${ACR_OVERLAYBD_INSTALL_SCRIPT}"
        }
        cleanup_streaming_install() {
            rm -f "${TEST_STREAMING_DIR}/install.sh"
            rmdir "${TEST_STREAMING_DIR}"
        }
        BeforeEach 'setup_streaming_install'
        AfterEach 'cleanup_streaming_install'
        installACLSysext() {
            echo "installACLSysext $*"
            return "${INSTALL_RC:-0}"
        }
        systemd-tmpfiles() {
            echo "systemd-tmpfiles $*"
            return "${TMPFILES_RC:-0}"
        }
        systemctl() {
            echo "systemctl $*"
            return "${RELOAD_RC:-0}"
        }

        It 'merges the extension and prepares writable paths before reloading units'
            When call installArtifactStreamingSysext
            The line 1 of output should equal "installACLSysext artifact-streaming amd64"
            The line 2 of output should equal "systemd-tmpfiles --create /usr/lib/tmpfiles.d/artifact-streaming.conf"
            The line 3 of output should equal "prepare overlaybd"
            The line 4 of output should equal "systemctl daemon-reload"
            The status should be success
        End

        It 'stops when extension installation fails'
            INSTALL_RC=231
            When call installArtifactStreamingSysext
            The output should equal "installACLSysext artifact-streaming amd64"
            The status should equal 231
        End

        It 'passes the CPU architecture through to artifact selection'
            TEST_ARCH=arm64
            When call installArtifactStreamingSysext
            The line 1 of output should equal "installACLSysext artifact-streaming arm64"
            The line 4 of output should equal "systemctl daemon-reload"
            The status should be success
        End

        It 'stops if CPU architecture resolution fails'
            getCPUArch() { return 1; }
            When call installArtifactStreamingSysext
            The output should equal ""
            The status should be failure
        End

        It 'stops when writable-path setup fails'
            TMPFILES_RC=1
            When call installArtifactStreamingSysext
            The output should not include "prepare overlaybd"
            The output should not include "daemon-reload"
            The status should be failure
        End

        It 'stops when the compatibility installer fails'
            export COMPAT_RC=1
            When call installArtifactStreamingSysext
            The output should include "prepare overlaybd"
            The output should not include "daemon-reload"
            The status should be failure
        End

        It 'propagates unit reload errors'
            RELOAD_RC=1
            When call installArtifactStreamingSysext
            The output should include "systemctl daemon-reload"
            The status should be failure
        End
    End

    Describe 'installGPUDriverSysext grid vs cuda selection'
        # Tests the driver-type routing in installGPUDriverSysext():
        # NVIDIA_GPU_DRIVER_TYPE="grid"     -> nvidia-driver-vgpu sysext (converged A10 sizes)
        # NVIDIA_GPU_DRIVER_TYPE="grid-v20" -> fail fast (Ubuntu-only, no ACL sysext)
        # NVIDIA_GPU_DRIVER_TYPE="cuda"/etc -> cuda / cuda-open sysext
        #
        # We mock the SKU lookup and downstream install/setup so we can isolate the
        # selection logic without pulling real sysext images.

        MOCK_VM_SKU=""
        get_compute_sku() { echo "$MOCK_VM_SKU"; }

        # Capture which sysext was selected and avoid real installs.
        installACLGPUSysext() { echo "installACLGPUSysext $1"; }
        systemd-tmpfiles() { return 0; }

        # Mock should_use_nvidia_open_drivers to avoid IMDS dependency.
        MOCK_OPEN_RET=0
        should_use_nvidia_open_drivers() { return "$MOCK_OPEN_RET"; }

        It 'selects the vGPU sysext when NVIDIA_GPU_DRIVER_TYPE is grid'
            NVIDIA_GPU_DRIVER_TYPE="grid"
            MOCK_VM_SKU="Standard_NV36ads_A10_v5"
            When run installGPUDriverSysext
            The status should be success
            The output should include "NVIDIA GRID driver (converged)"
            The output should include "installACLGPUSysext nvidia-driver-vgpu"
        End

        It 'fails fast for grid-v20 (Ubuntu-only) instead of installing a CUDA sysext'
            # RTX PRO 6000 BSE v6 maps to grid-v20, which ships only as the
            # aks-gpu-grid-v20 container image consumed on Ubuntu. There is no
            # nvidia-driver-vgpu v20 sysext for Azure Container Linux, so the guard
            # must exit with ERR_NVIDIA_DRIVER_INSTALL rather than silently falling
            # through to the cuda sysext on a vGPU node. Use 'run' so the guard's
            # exit is captured as a status instead of aborting the example.
            NVIDIA_GPU_DRIVER_TYPE="grid-v20"
            MOCK_VM_SKU="Standard_NC144ds_xl_RTXPRO6000BSE_v6"
            When run installGPUDriverSysext
            The status should equal "$ERR_NVIDIA_DRIVER_INSTALL"
            The output should include "only supported on Ubuntu"
            The output should not include "installACLGPUSysext"
        End

        It 'selects the cuda-open sysext for A100 when NVIDIA_GPU_DRIVER_TYPE is cuda'
            NVIDIA_GPU_DRIVER_TYPE="cuda"
            MOCK_VM_SKU="Standard_ND96asr_v4"
            MOCK_OPEN_RET=0
            When run installGPUDriverSysext
            The status should be success
            The output should include "NVIDIA OpenRM driver (cuda-open)"
            The output should include "installACLGPUSysext nvidia-driver-cuda-open"
        End
    End
End
