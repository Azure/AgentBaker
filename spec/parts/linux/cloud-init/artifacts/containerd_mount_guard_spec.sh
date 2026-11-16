#!/bin/bash

Describe 'containerd temporary-disk mount guard'
    CSE_CONFIG_GPU_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_gpu.sh"
    CSE_CONFIG_LOCALDNS_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_localdns.sh"
    CSE_CONFIG_KUBELET_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_kubelet.sh"
    CSE_CONFIG_NETWORK_FILEPATH="./parts/linux/cloud-init/artifacts/cse_config_network.sh"
    Include './parts/linux/cloud-init/artifacts/cse_config.sh'

    setup() {
        WORKDIR=$(mktemp -d)
        DROPIN="$WORKDIR/10-mount-guard.conf"
        HAS_KUBELET_DISK_TYPE=true
        WAIT_STATUS=0
        WRITE_STATUS=0
        REMOVE_STATUS=0
        WAITED=false
        WROTE=false
        REMOVED=false
    }
    cleanup() {
        command rm -f "$DROPIN" "$WORKDIR/mountpoint" "$WORKDIR/sleep" "$WORKDIR/timeout" "$WORKDIR/checks"
        rmdir "$WORKDIR"
    }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    retrycmd_if_failure() {
        [ "$*" = '120 1 5 mountpoint -q /mnt' ] || return 1
        WAITED=true
        return "$WAIT_STATUS"
    }
    tee() {
        [ "$*" = '/etc/systemd/system/containerd.service.d/10-mount-guard.conf' ] || return 1
        [ "$WAITED" = true ] || return 1
        cat > "$DROPIN"
        [ "$WRITE_STATUS" -eq 0 ] || return "$WRITE_STATUS"
        WROTE=true
    }
    rm() {
        [ "$*" = '-f /etc/systemd/system/containerd.service.d/10-mount-guard.conf' ] || return 1
        [ "$REMOVE_STATUS" -eq 0 ] || return "$REMOVE_STATUS"
        command rm -f "$DROPIN"
        REMOVED=true
    }

    It 'waits for the mount before installing a persistent per-start check'
        When call configureContainerdMountGuard
        The status should be success
        The output should be blank
        The variable WAITED should equal true
        The variable WROTE should equal true
        The contents of file "$DROPIN" should include 'ExecStartPre=/bin/sh -c'
        The contents of file "$DROPIN" should include 'timeout 120'
        The contents of file "$DROPIN" should include 'TimeoutStartSec=150'
        The contents of file "$DROPIN" should not include 'containerd config dump'
    End

    It 'fails without installing a hook when the provisioning wait fails'
        WAIT_STATUS=1
        When call configureContainerdMountGuard
        The status should be failure
        The stderr should equal 'Timed out waiting for /mnt before configuring containerd'
        The variable WROTE should equal false
    End

    It 'reports drop-in write failures'
        WRITE_STATUS=1
        When call configureContainerdMountGuard
        The status should be failure
        The stderr should equal 'Unable to configure the containerd temporary-disk mount guard'
    End

    It 'removes stale activation for OS placement without waiting'
        HAS_KUBELET_DISK_TYPE=false
        WAIT_STATUS=1
        touch "$DROPIN"
        When call configureContainerdMountGuard
        The status should be success
        The output should be blank
        The variable REMOVED should equal true
        The variable WAITED should equal false
        The variable WROTE should equal false
        The path "$DROPIN" should not be exist
    End

    It 'reports stale-hook removal failures'
        HAS_KUBELET_DISK_TYPE=false
        REMOVE_STATUS=1
        When call configureContainerdMountGuard
        The status should be failure
        The stderr should equal 'Unable to remove the containerd mount guard for an OS disk'
    End

    Describe 'the generated POSIX per-start command'
        setupPrestart() {
            configureContainerdMountGuard
            PRESTART=$(sed -n "s/^ExecStartPre=\/bin\/sh -c '\\(.*\\)'$/\\1/p" "$DROPIN")
            export WORKDIR READY_AFTER=0 TIMEOUT_STATUS=0
            printf '%s\n' '#!/bin/sh' \
                '[ "$*" = "-q /mnt" ] || exit 2' \
                'checks=$(cat "$WORKDIR/checks")' \
                'checks=$((checks + 1))' \
                'echo "$checks" > "$WORKDIR/checks"' \
                '[ "$checks" -gt "$READY_AFTER" ]' > "$WORKDIR/mountpoint"
            printf '%s\n' '#!/bin/sh' '[ "$*" = "1" ]' > "$WORKDIR/sleep"
            printf '%s\n' '#!/bin/sh' '[ "$1" = "120" ] || exit 2' \
                '[ "$TIMEOUT_STATUS" -eq 0 ] || exit "$TIMEOUT_STATUS"' \
                'shift' 'exec "$@"' > "$WORKDIR/timeout"
            chmod +x "$WORKDIR/mountpoint" "$WORKDIR/sleep" "$WORKDIR/timeout"
            echo 0 > "$WORKDIR/checks"
        }
        BeforeEach 'setupPrestart'

        runPrestart() {
            PATH="$WORKDIR:$PATH" /bin/sh -c "$PRESTART"
        }

        It 'starts without sleeping when the mount is ready'
            When call runPrestart
            The status should be success
            The output should be blank
            The contents of file "$WORKDIR/checks" should equal 1
        End

        It 'waits for delayed readiness independently of mount-unit state'
            READY_AFTER=5
            When call runPrestart
            The status should be success
            The output should be blank
            The contents of file "$WORKDIR/checks" should equal 6
        End

        It 'rechecks readiness on every start without a baked marker'
            repeatPrestart() {
                runPrestart || return
                READY_AFTER=3
                runPrestart
            }
            When call repeatPrestart
            The status should be success
            The output should be blank
            The contents of file "$WORKDIR/checks" should equal 4
        End

        It 'fails explicitly when the mount wait times out'
            TIMEOUT_STATUS=124
            When call runPrestart
            The status should be failure
            The stderr should equal 'Unable to confirm /mnt mount before starting containerd'
        End
    End
End
