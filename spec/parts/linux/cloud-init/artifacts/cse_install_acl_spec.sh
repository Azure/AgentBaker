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
