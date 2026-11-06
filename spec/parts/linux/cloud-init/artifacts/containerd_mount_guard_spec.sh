#!/bin/bash

Describe 'containerd-mount-guard.sh'
    Include './parts/linux/cloud-init/artifacts/containerd-mount-guard.sh'

    setup() {
        CONFIG_DUMP='version = 2
root = "/mnt/aks/containers"
[plugins."io.containerd.snapshotter.v1.overlayfs"]'
        CONFIG_STATUS=0
        MOUNT_READY_AFTER=0
        MOUNT_CHECKS=0
        SLEEPS=0
    }

    BeforeEach 'setup'

    containerd() {
        [ "$*" = 'config dump' ] || return 1
        printf '%s\n' "$CONFIG_DUMP"
        return "$CONFIG_STATUS"
    }

    mountpoint() {
        [ "$*" = '-q /mnt' ] || return 1
        MOUNT_CHECKS=$((MOUNT_CHECKS + 1))
        [ "$SLEEPS" -ge "$MOUNT_READY_AFTER" ]
    }

    sleep() {
        [ "$*" = '1' ] || return 1
        SLEEPS=$((SLEEPS + 1))
    }

    It 'starts immediately when the resource disk is already mounted'
        When call waitForContainerdMount
        The status should be success
        The output should be blank
        The variable MOUNT_CHECKS should equal 1
        The variable SLEEPS should equal 0
    End

    It 'waits for the actual mount even if the mount unit has already been skipped'
        MOUNT_READY_AFTER=5
        When call waitForContainerdMount
        The status should be success
        The output should be blank
        The variable MOUNT_CHECKS should equal 6
        The variable SLEEPS should equal 5
    End

    It 'fails instead of starting containerd when the mount never becomes ready'
        MOUNT_READY_AFTER=121
        When call waitForContainerdMount
        The status should be failure
        The stderr should equal 'Timed out waiting for /mnt to be mounted before starting containerd'
        The variable MOUNT_CHECKS should equal 120
        The variable SLEEPS should equal 120
    End

    It 'surfaces configuration errors without pretending the root is on the OS disk'
        CONFIG_STATUS=1
        When call waitForContainerdMount
        The status should be failure
        The stderr should equal 'Unable to read containerd configuration before checking the resource disk'
        The variable MOUNT_CHECKS should equal 0
    End

    Describe 'effective root selection'
        Parameters
            'root = "/var/lib/containerd"'
            "root = '/var/lib/containerd'"
            'root = "/mnt-other/containers"'
            'version = 2'
        End

        It 'ignores a plugin root that is not the containerd data root'
            CONFIG_DUMP='[plugins.example]
root = "/mnt/plugin-state"'
            MOUNT_READY_AFTER=121
            When call waitForContainerdMount
            The status should be success
            The output should be blank
            The variable MOUNT_CHECKS should equal 0
        End

        It "does not wait for an OS root or unrelated plugin root: $1"
            CONFIG_DUMP="$1"
            MOUNT_READY_AFTER=121
            When call waitForContainerdMount
            The status should be success
            The output should be blank
            The variable MOUNT_CHECKS should equal 0
            The variable SLEEPS should equal 0
        End
    End

    Describe 'temporary roots and TOML quote styles'
        Parameters
            'root = "/mnt/aks/containers"'
            "root = '/mnt/aks/containers'"
            'root = "/mnt"'
            "root = '/mnt'"
            'root = "/mnt/custom-containerd-root"'
        End

        It "guards the effective temporary root: $1"
            CONFIG_DUMP="$1"
            MOUNT_READY_AFTER=1
            When call waitForContainerdMount
            The status should be success
            The output should be blank
            The variable MOUNT_CHECKS should equal 2
            The variable SLEEPS should equal 1
        End
    End

    It 'checks the mount on every start rather than using a baked readiness marker'
        startTwice() {
            waitForContainerdMount || return
            MOUNT_READY_AFTER=2
            MOUNT_CHECKS=0
            waitForContainerdMount
        }

        When call startTwice
        The status should be success
        The output should be blank
        The variable MOUNT_CHECKS should equal 3
        The variable SLEEPS should equal 2
    End

    Describe 'production entry point'
        guardedStart() {
            export -f containerd mountpoint sleep
            export CONFIG_DUMP CONFIG_STATUS MOUNT_READY_AFTER MOUNT_CHECKS SLEEPS
            __SOURCED__="" /bin/bash ./parts/linux/cloud-init/artifacts/containerd-mount-guard.sh &&
                echo 'containerd started'
        }

        It 'allows daemon startup after a delayed mount'
            MOUNT_READY_AFTER=5
            When run guardedStart
            The status should be success
            The output should equal 'containerd started'
        End

        It 'prevents daemon startup when resource-disk setup fails'
            MOUNT_READY_AFTER=121
            When run guardedStart
            The status should be failure
            The output should be blank
            The stderr should equal 'Timed out waiting for /mnt to be mounted before starting containerd'
        End
    End

    It 'installs the guard in the OSGuard image'
        When call grep -A2 -F 'source: /AgentBaker/parts/linux/cloud-init/artifacts/containerd-mount-guard' \
            vhdbuilder/packer/imagecustomizer/azlosguard/azlosguard.yml
        The status should be success
        The output should include 'destination: /opt/azure/containers/containerd-mount-guard.sh'
        The output should include 'destination: /etc/systemd/system/containerd.service.d/10-mount-guard.conf'
    End

    It 'keeps the service guard separate from the drop-in overwritten by older CSE scripts'
        When call cat parts/linux/cloud-init/artifacts/containerd-mount-guard.conf
        The status should be success
        The output should include 'ExecStartPre=/bin/bash /opt/azure/containers/containerd-mount-guard.sh'
        The output should include 'TimeoutStartSec=150'
    End

    Describe 'VHD upload mappings'
        Parameters
            base
            cvm
            arm64-gen2
            arm64-gb
            mariner
            mariner-arm64
            mariner-cvm
            acl
            acl-arm64
        End

        It "uploads both guard artifacts in the $1 image template"
            filter='[.provisioners[] | select(.type == "file" and .source == "parts/linux/cloud-init/artifacts/containerd-mount-guard.sh" and .destination == "/home/packer/containerd-mount-guard.sh")] | length == 1'
            dropin_filter='[.provisioners[] | select(.type == "file" and .source == "parts/linux/cloud-init/artifacts/containerd-mount-guard.conf" and .destination == "/home/packer/containerd-mount-guard.conf")] | length == 1'
            checkMappings() {
                jq -e "$filter" "vhdbuilder/packer/vhd-image-builder-$1.json" &&
                    jq -e "$dropin_filter" "vhdbuilder/packer/vhd-image-builder-$1.json"
            }
            When call checkMappings "$1"
            The status should be success
            The output should equal "true
true"
        End
    End
End
