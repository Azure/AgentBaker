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

        It 'unloads a full auto-loaded module stack: dependents (modeset/uvm/drm) before nvidia'
            # Regression guard for the refcnt trap: a real cuda/cuda-lts auto-load leaves nvidia with
            # refcnt>=1 because nvidia_modeset/uvm/drm depend on it. The unload must remove those
            # dependents FIRST, then nvidia. (A refcnt==0 pre-check would skip the whole stack and
            # leave nvidia resident -- the exact bug this replaces.)
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            systemctl() { echo "mock systemctl $*"; }
            # Model a live stack: lsmod reports nvidia until every module has been rmmod'd.
            _loaded="nvidia nvidia_modeset nvidia_uvm nvidia_drm"
            lsmod() { case "${_loaded}" in *nvidia*) echo "nvidia 104165376 3";; *) echo "";; esac; }
            rmmod() { echo "mock rmmod $1"; _loaded="${_loaded//$1/}"; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "module_before=true"
            # persistenced stopped first (holds a device handle, not a module dependency)
            The output should include "mock systemctl stop nvidia-persistenced"
            # dependents unloaded before the base nvidia module
            The output should include "mock rmmod nvidia_drm"
            The output should include "mock rmmod nvidia_uvm"
            The output should include "mock rmmod nvidia_modeset"
            The output should include "mock rmmod nvidia"
            # after the full-stack unload the module is gone -> complete
            The output should include "module_after=false"
            The output should include "status=cleaned"
        End

        It 'keeps the marker (incomplete) when a genuinely busy module refuses to unload'
            # rmmod without -f refuses an in-use module and returns nonzero; nvidia stays loaded. The
            # teardown must report module_after=true and KEEP the marker so the next provision retries.
            marker="$(mktemp)"
            GPU_DKMS_MARKER_FILE="${marker}"
            ldconfig() { echo "mock ldconfig"; }
            systemctl() { echo "mock systemctl $*"; }
            lsmod() { echo "nvidia 104165376 1"; }        # stays loaded regardless of rmmod attempts
            rmmod() { echo "mock rmmod $1 (in use)"; return 1; }
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "module_before=true"
            # rmmod IS attempted now (no self-defeating refcnt pre-check), it just fails on a busy module
            The output should include "mock rmmod nvidia"
            The output should include "module_after=true"
            The output should include "status=incomplete"
            The output should include "marker_after=true"
        End
    End
End
