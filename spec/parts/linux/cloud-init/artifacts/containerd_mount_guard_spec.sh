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

    Describe 'conditional containerd mount-guard activation'
        set +o nounset
        CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
        CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
        CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
        CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
        Include './parts/linux/cloud-init/artifacts/cse_config.sh'

        setupActivation() {
            INSTALLED_CONFIG='version = 2
    root = "/mnt/aks/containers"'
            READ_STATUS=0
            SCRIPT_PRESENT=true
            SCRIPT_CHECKS=0
            INSTALL_STATUS=0
            REMOVE_STATUS=0
            INSTALLED=false
            REMOVED=false
        }

        BeforeEach 'setupActivation'

        sed() {
            [ "$2" = /etc/containerd/config.toml ] || return 1
            [ "$READ_STATUS" -eq 0 ] || return "$READ_STATUS"
            command sed "$1" <<< "$INSTALLED_CONFIG"
        }

        test() {
            if [ "$*" = '-r /opt/azure/containers/containerd-mount-guard.sh' ]; then
                SCRIPT_CHECKS=$((SCRIPT_CHECKS + 1))
                [ "$SCRIPT_PRESENT" = true ]
            else
                builtin test "$@"
            fi
        }

        install() {
            [ "$*" = '-m 0644 /opt/azure/containers/containerd-mount-guard.conf /etc/systemd/system/containerd.service.d/10-mount-guard.conf' ] || return 1
            [ "$INSTALL_STATUS" -eq 0 ] || return "$INSTALL_STATUS"
            INSTALLED=true
        }

        rm() {
            [ "$*" = '-f /etc/systemd/system/containerd.service.d/10-mount-guard.conf' ] || return 1
            [ "$REMOVE_STATUS" -eq 0 ] || return "$REMOVE_STATUS"
            REMOVED=true
            INSTALLED=false
        }

        containerd() {
            echo 'OS-root configuration must not invoke containerd config dump' >&2
            return 1
        }

        Describe 'OS roots'
            Parameters
                'root = "/var/lib/containerd"'
                "root = '/var/lib/containerd'"
                'root = "/mnt-other/containers"'
                'version = 2'
            End

            It "removes any stale hook without requiring a working guard or config dump: $1"
                INSTALLED_CONFIG="$1"
                HAS_KUBELET_DISK_TYPE=true
                SCRIPT_PRESENT=false
                INSTALLED=true
                When call configureContainerdMountGuard
                The status should be success
                The output should be blank
                The stderr should be blank
                The variable INSTALLED should equal false
                The variable REMOVED should equal true
                The variable SCRIPT_CHECKS should equal 0
            End
        End

        Describe 'temporary roots'
            Parameters
                'root = "/mnt/aks/containers"'
                "root = '/mnt'"
                'root = "/mnt/custom-containerd-root"'
                '  root = "/mnt/aks/containers"'
            End

            It "activates only the separate guard drop-in for the installed root: $1"
                INSTALLED_CONFIG="$1"
                HAS_KUBELET_DISK_TYPE=false
                When call configureContainerdMountGuard
                The status should be success
                The output should be blank
                The variable INSTALLED should equal true
                The variable REMOVED should equal false
                The variable SCRIPT_CHECKS should equal 1
            End
        End

        It 'ignores plugin roots and commented roots'
            INSTALLED_CONFIG='# root = "/mnt/unused"
    version = 2
      [plugins.example]
    root = "/mnt/plugin-state"'
            When call configureContainerdMountGuard
            The status should be success
            The output should be blank
            The variable INSTALLED should equal false
            The variable SCRIPT_CHECKS should equal 0
        End

        It 'selects the root from a preserved NVIDIA config'
            INSTALLED_CONFIG='version = 2
    root = "/mnt/gpu-containerd"
    [plugins."io.containerd.cri.v1.runtime".containerd.runtimes.nvidia.options]
    BinaryName = "/usr/bin/nvidia-container-runtime"'
            When call configureContainerdMountGuard
            The status should be success
            The output should be blank
            The variable INSTALLED should equal true
        End

        It 'fails explicitly when a temporary root has no readable guard script'
            SCRIPT_PRESENT=false
            When call configureContainerdMountGuard
            The status should be failure
            The stderr should equal 'Containerd temporary root requires the mount-guard script'
            The variable INSTALLED should equal false
        End

        It 'fails explicitly when the drop-in template cannot be installed'
            INSTALL_STATUS=1
            When call configureContainerdMountGuard
            The status should be failure
            The stderr should equal 'Unable to install the containerd temporary-root mount guard'
            The variable INSTALLED should equal false
        End

        It 'reports a config read failure without removing an existing guard'
            READ_STATUS=1
            INSTALLED=true
            When call configureContainerdMountGuard
            The status should be failure
            The stderr should equal 'Unable to inspect the provisioned containerd root for mount-guard configuration'
            The variable INSTALLED should equal true
            The variable REMOVED should equal false
        End

        It 'reports a classification command failure without removing an existing guard'
            grep() {
                return 2
            }
            INSTALLED=true
            When call configureContainerdMountGuard
            The status should be failure
            The stderr should equal 'Unable to classify the provisioned containerd root'
            The variable INSTALLED should equal true
            The variable REMOVED should equal false
        End

        It 'reports a stale-hook removal failure'
            INSTALLED_CONFIG='root = "/var/lib/containerd"'
            REMOVE_STATUS=1
            When call configureContainerdMountGuard
            The status should be failure
            The stderr should equal 'Unable to remove the containerd mount guard for an OS root'
        End

        It 'delivers the baked drop-in as an inactive template'
            When call grep -F 'cpAndMode /home/packer/containerd-mount-guard.conf' vhdbuilder/packer/packer_source.sh
            The status should be success
            The output should include '/opt/azure/containers/containerd-mount-guard.conf 644'
            The output should not include '/etc/systemd/system/containerd.service.d/'
        End
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
