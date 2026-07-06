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
            When call cleanUpPrebakedGPUDriver
            The status should be success
            The output should include "AKS_GPU_PREBAKE event=teardown"
            The output should include "status=cleaned"
            The output should include "marker_after=false"
            # the setuid nvidia-modprobe is part of the security-coverage check
            The output should include "modprobe_after=false"
        End
    End

    Describe 'installNvidiaIMEX'
        # nvidia-smi is only used for an informational log line; stub it so the function does not need a GPU.
        nvidia_smi_stub() { echo "580.159.04"; }

        It 'fails when no nvidia-imex deb was baked into the cache'
            nvidia-smi() { nvidia_smi_stub; }
            # empty find output => no cached deb
            find() { :; }
            When call installNvidiaIMEX
            The status should be failure
            The output should include "no cached nvidia-imex deb"
        End

        It 'installs from the cache, creates the channel, cleans the cache, and leaves the daemon disabled'
            nvidia-smi() { nvidia_smi_stub; }
            find() { echo "/opt/nvidia-imex/downloads/nvidia-imex_580.159.04-1ubuntu1_arm64.deb"; }
            dpkg() { echo "mock dpkg $*"; }
            rm() { echo "mock rm $*"; }
            systemctl() { echo "mock systemctl $*"; }
            When call installNvidiaIMEX
            The status should be success
            # installed from the baked cache (no runtime download)
            The output should include "mock dpkg -i /opt/nvidia-imex/downloads/nvidia-imex"
            # cache removed after install
            The output should include "mock rm -rf /opt/nvidia-imex"
            # daemon deliberately NOT started: the unit is disabled
            The output should include "mock systemctl disable nvidia-imex"
            # channel created + reboot flagged; final message proves we reached the end
            The output should include "channel pending reboot"
            The variable REBOOTREQUIRED should eq "true"
            # the channel modprobe option was written
            The contents of file "/etc/modprobe.d/nvidia-imex.conf" should include "NVreg_CreateImexChannel0=1"
        End
    End
End
