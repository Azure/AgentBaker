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

    Describe 'getLatestDebPackageVersion'
        getCPUArch() { echo "amd64"; }

        It 'selects the latest revision for the requested upstream version'
            apt() {
                cat <<'EOF'
Listing...
kubelet/repo 1.34.10-ubuntu22.04u1 amd64
kubelet/repo 1.34.10-ubuntu22.04u9 amd64
kubelet/repo 1.34.11-ubuntu22.04u1 amd64
EOF
            }

            When call getLatestDebPackageVersion kubelet 1.34.10
            The output should equal "1.34.10-ubuntu22.04u9"
        End

        It 'ignores revisions for other architectures'
            apt() {
                cat <<'EOF'
Listing...
kubelet/repo 1.34.10-ubuntu22.04u9 arm64
kubelet/repo 1.34.10-ubuntu22.04u8 amd64
EOF
            }

            When call getLatestDebPackageVersion kubelet 1.34.10
            The output should equal "1.34.10-ubuntu22.04u8"
        End

        It 'does not match a longer patch version'
            apt() {
                cat <<'EOF'
Listing...
kubelet/repo 1.34.1-ubuntu22.04u3 amd64
kubelet/repo 1.34.10-ubuntu22.04u9 amd64
EOF
            }

            When call getLatestDebPackageVersion kubelet 1.34.1
            The output should equal "1.34.1-ubuntu22.04u3"
        End
    End

    Describe 'installContainerdWithAptGet revision comparison'
        containerd_download_root="/tmp/cse-install-ubuntu-containerd-$$"

        setup_containerd_revision() {
            mkdir -p "${containerd_download_root}"
            CONTAINERD_DOWNLOADS_DIR="${containerd_download_root}"
        }

        cleanup_containerd_revision() {
            rm -rf "${containerd_download_root}"
        }

        BeforeEach 'setup_containerd_revision'
        AfterEach 'cleanup_containerd_revision'

        semverCompare() {
            [ "$1" = "$2" ] && return 0
            [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | tail -n 1)" = "$1" ]
        }
        dpkg() { echo "ii  moby-containerd"; }
        getLatestDebPackageVersion() { echo "1:1.7.35+azure-ubuntu22.04u2"; }
        removeContainerd() { echo "removeContainerd"; }
        downloadContainerdFromVersion() {
            touch "${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_1.7.35+azure-ubuntu22.04u2_amd64.deb"
        }
        installDebPackageFromFile() { echo "installDebPackageFromFile $1"; }
        logs_to_events() {
            shift
            eval "$*"
        }

        It 'installs the latest revision when the installed upstream version is equal but stale'
            dpkg-query() { echo "1:1.7.35+azure-ubuntu22.04u1"; }

            When call installContainerdWithAptGet 1.7.35 "${CONTAINERD_DOWNLOADS_DIR}"

            The output should include "installed moby-containerd package version 1:1.7.35+azure-ubuntu22.04u1 does not match latest revision 1:1.7.35+azure-ubuntu22.04u2"
            The output should include "installDebPackageFromFile ${CONTAINERD_DOWNLOADS_DIR}/moby-containerd_1.7.35+azure-ubuntu22.04u2_amd64.deb"
        End

        It 'skips installation when the latest revision is already installed'
            dpkg-query() { echo "1:1.7.35+azure-ubuntu22.04u2"; }

            When call installContainerdWithAptGet 1.7.35 "${CONTAINERD_DOWNLOADS_DIR}"

            The output should include "currently installed containerd version 1:1.7.35+azure-ubuntu22.04u2 satisfies target version 1.7.35"
            The output should not include "removeContainerd"
            The output should not include "installDebPackageFromFile"
        End
    End

    Describe 'logResolvedPackageVersion'
        resolved_version_log="/tmp/cse-install-ubuntu-resolved-version-$$"

        cleanup_resolved_version_log() {
            rm -f "${resolved_version_log}"
        }

        BeforeEach 'cleanup_resolved_version_log'
        AfterEach 'cleanup_resolved_version_log'

        It 'does not create the VHD completion marker during node provisioning'
            VHD_LOGS_FILEPATH="${resolved_version_log}"

            When call logResolvedPackageVersion moby-runc 1.4.3 1.4.3-1ubuntu22.04u1

            The output should equal "Resolved moby-runc package version 1.4.3 -> 1.4.3-1ubuntu22.04u1"
            The path "${resolved_version_log}" should not be exist
        End

        It 'appends the resolved version when the VHD completion marker exists'
            VHD_LOGS_FILEPATH="${resolved_version_log}"
            touch "${VHD_LOGS_FILEPATH}"

            When call logResolvedPackageVersion moby-runc 1.4.3 1.4.3-1ubuntu22.04u1

            The output should equal "Resolved moby-runc package version 1.4.3 -> 1.4.3-1ubuntu22.04u1"
            The contents of file "${resolved_version_log}" should include "moby-runc package version 1.4.3-1ubuntu22.04u1 (requested 1.4.3)"
        End
    End

    Describe 'ensureRunc repository fallback'
        runc_download_root="/tmp/cse-install-ubuntu-runc-$$"

        setup_runc_fallback() {
            mkdir -p "${runc_download_root}"
            RUNC_DOWNLOADS_DIR="${runc_download_root}"
            VHD_LOGS_FILEPATH="${runc_download_root}/missing-vhd-marker"
        }

        cleanup_runc_fallback() {
            rm -rf "${runc_download_root}"
        }

        BeforeEach 'setup_runc_fallback'
        AfterEach 'cleanup_runc_fallback'

        isARM64() { echo 0; }
        getCPUArch() { echo "amd64"; }
        runc() { echo "runc version 1.4.2"; }
        getLatestDebPackageVersion() { echo "1.4.3-10ubuntu22.04u1"; }
        apt_get_install() { echo "apt_get_install $*"; }

        It 'installs the exact latest revision for a revisionless version'
            When call ensureRunc 1.4.3 "" "${RUNC_DOWNLOADS_DIR}"

            The output should include "Resolved moby-runc package version 1.4.3 -> 1.4.3-10ubuntu22.04u1"
            The output should include "apt_get_install 20 30 120 moby-runc=1.4.3-10ubuntu22.04u1 --allow-downgrades"
            The output should not include "moby-runc=1.4.3*"
        End

        It 'installs the latest revision when the installed upstream version is equal but stale'
            runc() { echo "runc version 1.4.3"; }
            dpkg() { echo "ii  moby-runc"; }
            dpkg-query() { echo "1.4.3-1ubuntu22.04u1"; }

            When call ensureRunc 1.4.3 "" "${RUNC_DOWNLOADS_DIR}"

            The output should include "installed moby-runc package version 1.4.3-1ubuntu22.04u1 does not match latest revision 1.4.3-10ubuntu22.04u1"
            The output should include "apt_get_install 20 30 120 moby-runc=1.4.3-10ubuntu22.04u1 --allow-downgrades"
        End

        It 'skips installation when the latest revision is already installed'
            runc() { echo "runc version 1.4.3"; }
            dpkg() { echo "ii  moby-runc"; }
            dpkg-query() { echo "1.4.3-10ubuntu22.04u1"; }

            When call ensureRunc 1.4.3 "" "${RUNC_DOWNLOADS_DIR}"

            The output should include "target moby-runc package version 1.4.3-10ubuntu22.04u1 is already installed"
            The output should not include "apt_get_install"
        End
    End
End
