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

        It 'preserves a customer-replaced driver but still cleans AKS''s own baked residue'
            # A --gpu-driver None node whose customer baked their own DIFFERENT-version driver (via
            # PreparedImageSpecification / custom image) on top of the shared VHD: the marker records
            # AKS''s baked version, but a different version is present on disk. AKS must NOT wipe the
            # customer''s driver; it removes only its own attributable residue -- the version-scoped
            # DKMS dir and the aks-gpu /usr/bin/lib64 staging.
            marker="$(mktemp)"
            printf 'kernel=6.8.0-1063-azure\ndriver_version=580.65.06\ndriver_kind=cuda\narch=x86_64\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/570.124.06"  # only the customer''s version present
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { return 1; }  # no loaded/on-disk module; detect via the DKMS registration
            rm() { echo "mock rm $*"; }
            rmmod() { echo "mock rmmod $*"; }  # must NOT be called
            ldconfig() { echo "mock ldconfig"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=preserved_customer_driver"
            The output should include "marker_version=580.65.06"
            The output should include "installed_version=570.124.06"
            # AKS''s own residue is cleaned: its version-scoped DKMS dir and the /usr/bin/lib64 staging...
            The output should include "mock rm -rf ${dkmsdir}/580.65.06"
            The output should include "mock rm -rf /usr/bin/lib64"
            # ...but the customer''s driver is preserved: no full teardown, no rmmod, no userspace wipe
            The output should not include "Removing pre-baked NVIDIA driver"
            The output should not include "mock rm -f /usr/bin/nvidia-smi"
            The output should not include "mock rmmod"
        End

        It 'preserves a customer version that coexists with AKS''s lingering DKMS dir (marker sorts first)'
            # AKS''s own 580 DKMS dir still lingers next to the customer''s 590. Scanning only the first
            # registration would return 580 (== marker) and blanket-teardown the customer''s 590, so we
            # must scan every registration and treat any non-marker version as customer-owned.
            marker="$(mktemp)"
            printf 'driver_version=580.65.06\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/580.65.06" "${dkmsdir}/590.00.00"
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { return 1; }
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=preserved_customer_driver"
            The output should include "installed_version=590.00.00"
            The output should include "mock rm -rf ${dkmsdir}/580.65.06"
            The output should not include "Removing pre-baked NVIDIA driver"
        End

        It 'preserves a customer version that coexists with AKS''s lingering DKMS dir (customer sorts first)'
            # Same coexistence, opposite glob order: the customer''s 570 sorts before AKS''s 580. The
            # decision must be order-independent -- preserve either way.
            marker="$(mktemp)"
            printf 'driver_version=580.65.06\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/570.124.06" "${dkmsdir}/580.65.06"
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { return 1; }
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=preserved_customer_driver"
            The output should include "installed_version=570.124.06"
            The output should include "mock rm -rf ${dkmsdir}/580.65.06"
            The output should not include "Removing pre-baked NVIDIA driver"
        End

        It 'preserves when an authoritative source reports the marker but a DKMS version differs'
            # AKS''s own on-disk .ko (marker version) is what modinfo reports, yet the customer has a
            # different version DKMS-registered. The marker-version source must NOT short-circuit the
            # scan -- the differing DKMS version still proves a customer driver and blocks teardown.
            marker="$(mktemp)"
            printf 'driver_version=580.65.06\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/590.00.00"
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { echo "580.65.06"; }  # AKS''s own on-disk .ko == marker
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "status=preserved_customer_driver"
            The output should include "installed_version=590.00.00"
            The output should not include "Removing pre-baked NVIDIA driver"
        End

        It 'tears down a same-version customer rebuild like AKS''s own bake'
            # If every version present EQUALS what AKS baked, a customer rebuild is indistinguishable
            # from AKS''s own pre-bake -- both are torn down (accepted: same version == equivalent driver).
            marker="$(mktemp)"
            printf 'driver_version=580.65.06\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/580.65.06"
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { return 1; }
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "Removing pre-baked NVIDIA driver"
            The output should not include "status=preserved_customer_driver"
        End

        It 'still tears down the prebake when it matches the marker (AKS-owned: GPU-Operator / non-GPU)'
            # No customer driver -- the only version on disk equals what AKS baked, so the pre-bake is
            # AKS''s own dead weight and is torn down as before (clean slate for the GPU Operator, etc.).
            marker="$(mktemp)"
            printf 'driver_version=580.65.06\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/580.65.06"
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { return 1; }  # no on-disk .ko lookup; fall through to the DKMS registration
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }  # no nvidia module loaded
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "Removing pre-baked NVIDIA driver"
            The output should include "mock rm -rf /var/lib/dkms/nvidia"
        End

        It 'force_full_teardown ignores a different-version driver and tears down anyway (grid path)'
            # The grid caller passes force_full_teardown: a cuda-vs-grid KIND collision means the cuda
            # pre-bake must go regardless of version, so the customer-preserve branch is skipped.
            marker="$(mktemp)"
            printf 'driver_version=580.65.06\n' > "${marker}"
            GPU_DKMS_MARKER_FILE="${marker}"
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/570.124.06"
            GPU_DKMS_NVIDIA_DIR="${dkmsdir}"
            modinfo() { return 1; }  # a customer version is present, but the flag overrides preserve
            rm() { echo "mock rm $*"; }
            ldconfig() { echo "mock ldconfig"; }
            lsmod() { echo ""; }
            When call cleanUpPrebakedGPUDriver force_full_teardown
            The status should be success
            The output should include "Removing pre-baked NVIDIA driver"
            The output should not include "status=preserved_customer_driver"
        End
    End

    Describe 'findCustomerDriverVersion'
        It 'returns a DKMS-registered version that differs from the marker'
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/570.124.06"
            modinfo() { return 1; }  # no loaded/on-disk module
            When call findCustomerDriverVersion "580.65.06" "${dkmsdir}"
            The status should be success
            The output should equal "570.124.06"
        End

        It 'skips the marker version and returns a coexisting customer registration'
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/580.65.06" "${dkmsdir}/590.00.00"
            modinfo() { return 1; }
            When call findCustomerDriverVersion "580.65.06" "${dkmsdir}"
            The status should be success
            The output should equal "590.00.00"
        End

        It 'returns an on-disk module version that differs from the marker'
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/580.65.06"  # marker version only in DKMS
            modinfo() { echo "575.00.00"; }  # a .run install with a different on-disk module
            When call findCustomerDriverVersion "580.65.06" "${dkmsdir}"
            The status should be success
            The output should equal "575.00.00"
        End

        It 'echoes nothing when every present version equals the marker'
            dkmsdir="$(mktemp -d)"; mkdir -p "${dkmsdir}/580.65.06"
            modinfo() { return 1; }
            When call findCustomerDriverVersion "580.65.06" "${dkmsdir}"
            The status should be success
            The output should equal ""
        End

        It 'echoes nothing when no driver is present'
            dkmsdir="$(mktemp -d)"  # empty: no version dirs
            modinfo() { return 1; }
            When call findCustomerDriverVersion "580.65.06" "${dkmsdir}"
            The status should be success
            The output should equal ""
        End
    End

    Describe 'installPackageFromCache version matching'
        deb_cache_root="/tmp/shellspec-deb-cache-$$"

        setup_deb_cache() {
            mkdir -p "$deb_cache_root"
        }

        cleanup_deb_cache() {
            rm -rf "$deb_cache_root"
        }

        BeforeEach 'setup_deb_cache'
        AfterEach 'cleanup_deb_cache'

        # Mock functions used by installPackageFromCache
        fallbackToKubeBinaryInstall() { return 1; }
        logs_to_events() { shift; eval "$@"; }
        extractDebBinaryFromFile() { echo "extractDebBinaryFromFile $1 $2 $3"; }

        It 'does not match version 1.34.10 when requesting 1.34.1'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.1-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.10-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.11-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.12-1ubuntu22.04u1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal "kubelet_1.34.1-1ubuntu22.04u1_amd64.deb"
        End

        It 'matches the latest release of the exact version requested'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.1-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.1-2ubuntu22.04u1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal "kubelet_1.34.1-2ubuntu22.04u1_amd64.deb"
        End

        It 'returns empty when no matching version exists'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.10-1ubuntu22.04u1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.2-1ubuntu22.04u1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal ""
        End

        It 'matches version with plus suffix (azure convention)'
            downloadDir="$deb_cache_root"
            packageName="kubelet"
            packageVersion="1.34.1"
            touch "$downloadDir/kubelet_1.34.1+azure-1_amd64.deb"
            touch "$downloadDir/kubelet_1.34.10+azure-1_amd64.deb"

            result() {
                ls "${downloadDir}" | grep "${packageName}" | grep -E "${packageVersion}([^0-9]|$)" | sort -V | tail -n 1
            }
            When call result
            The output should equal "kubelet_1.34.1+azure-1_amd64.deb"
        End
    End
End
