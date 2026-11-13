#!/bin/bash

Describe 'AKS-owned system slice placement'
    Include "./parts/linux/cloud-init/artifacts/cse_config.sh"

    setup() {
        TEST_ROOT=$(mktemp -d)
        SYSTEM_SLICE_SYSTEMD_DIR="${TEST_ROOT}/systemd"
        mkdir -p "${SYSTEM_SLICE_SYSTEMD_DIR}"
        for service in localdns localdns-fallback aks-log-collector; do
            cp "./parts/linux/cloud-init/artifacts/${service}.service" "${SYSTEM_SLICE_SYSTEMD_DIR}/${service}.service"
        done
        cat > "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.slice" <<'EOF'
[Unit]
Description=localdns Slice
DefaultDependencies=no
Before=slices.target
Requires=system.slice
After=system.slice
[Slice]
MemoryMax=128M
CPUQuota=250%
EOF
        cp ./parts/linux/cloud-init/artifacts/aks-log-collector.slice "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log-collector.slice"
        cat > "${SYSTEM_SLICE_SYSTEMD_DIR}/aks.slice" <<'EOF'
[Slice]
MemoryMax=1G
EOF
        cat > "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log.slice" <<'EOF'
[Unit]
Before=slices.target
[Slice]
CPUAccounting=yes
CPUWeight=5
EOF
        LOCALDNS_CONTROL_GROUP=""
        FALLBACK_CONTROL_GROUP=""
        COLLECTOR_CONTROL_GROUP=""
        SYSTEMCTL_RELOAD_STATUS=0
        SYSTEMCTL_CAT_STATUS=0
        RESTART_STATUS=0
        LEGACY_SLICE_ACTIVE_STATE=active
    }

    cleanup() {
        rm -rf "${TEST_ROOT}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    systemctl() {
        case "$*" in
            "daemon-reload") return "${SYSTEMCTL_RELOAD_STATUS}" ;;
            "show localdns.service -p ControlGroup --value")
                printf '%s\n' "${LOCALDNS_CONTROL_GROUP}" ;;
            "show localdns-fallback.service -p ControlGroup --value")
                printf '%s\n' "${FALLBACK_CONTROL_GROUP}" ;;
            "show aks-log-collector.service -p ControlGroup --value")
                printf '%s\n' "${COLLECTOR_CONTROL_GROUP}" ;;
            show*" -p LoadState --value")
                if [ -f "${SYSTEM_SLICE_SYSTEMD_DIR}/$2" ]; then
                    echo loaded
                else
                    echo not-found
                fi
                ;;
            show*" -p FragmentPath -p DropInPaths --value")
                printf '%s\n' "${SYSTEM_SLICE_SYSTEMD_DIR}/$2" ;;
            show*" -p ActiveState --value")
                printf '%s\n' "${LEGACY_SLICE_ACTIVE_STATE}" ;;
            cat*)
                [ "${SYSTEMCTL_CAT_STATUS}" -eq 0 ] || return "${SYSTEMCTL_CAT_STATUS}"
                cat "${SYSTEM_SLICE_SYSTEMD_DIR}/$2"
                if [ -f "${SYSTEM_SLICE_SYSTEMD_DIR}/$2.d/limits.conf" ]; then
                    cat "${SYSTEM_SLICE_SYSTEMD_DIR}/$2.d/limits.conf"
                fi
                ;;
            *) echo "Unexpected systemctl call: $*" >&2; return 1 ;;
        esac
    }

    systemctl_restart() {
        echo "restart $*"
        LOCALDNS_CONTROL_GROUP="/system.slice/system-localdns.slice/localdns.service"
        return "${RESTART_STATUS}"
    }

    Describe 'configureSystemSlicePlacement'
        Parameters
            '' ''
            '/kubereserved.slice' '/system.slice'
        End

        It 'preserves slice limits and lifecycle in both hardening modes'
            KUBE_RESERVED_CGROUP="$1"
            SYSTEM_RESERVED_CGROUP="$2"
            mkdir -p "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log-collector.slice.d"
            printf '[Slice]\nMemoryMax=192M\nCPUQuota=10%%\n' > "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log-collector.slice.d/limits.conf"
            When call configureSystemSlicePlacement
            The status should be success
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-localdns.slice" should include "MemoryMax=128M"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-localdns.slice" should include "CPUQuota=250%"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-localdns.slice" should include "DefaultDependencies=no"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-localdns.slice" should include "Requires=system.slice"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks.slice" should include "MemoryMax=1G"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks-log.slice" should include "CPUWeight=5"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks-log-collector.slice" should include "CPUShares=100"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks-log-collector.slice" should include "CPUWeight=1"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks-log-collector.slice" should include "MemoryMax=192M"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks-log-collector.slice" should include "CPUQuota=10%"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-aks-log-collector.slice" should include "After=system.slice"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.service.d/90-aks-placement.conf" should include "Slice=system-localdns.slice"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns-fallback.service.d/90-aks-placement.conf" should include "Slice=system-localdns.slice"
            The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log-collector.service.d/90-aks-placement.conf" should include "Slice=system-aks-log-collector.slice"
            The path "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.slice.d" should not be exist
            The path "${SYSTEM_SLICE_SYSTEMD_DIR}/azure.slice.d" should not be exist
            The path "${SYSTEM_SLICE_SYSTEMD_DIR}/waagent.service.d" should not be exist
        End
    End

    It 'does not install orphan units on a legacy VHD without these assets'
        rm -f "${SYSTEM_SLICE_SYSTEMD_DIR}"/*.service "${SYSTEM_SLICE_SYSTEMD_DIR}"/*.slice
        When call configureSystemSlicePlacement
        The status should be success
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}" should be empty directory
    End

    It 'reports failures while reading an installed slice instead of losing its limits'
        SYSTEMCTL_CAT_STATUS=1
        When call configureSystemSlicePlacement
        The status should be failure
        The output should include "copySystemSlice: failed to read localdns.slice"
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/system-localdns.slice" should not be exist
    End

    It 'reports daemon-reload failure'
        SYSTEMCTL_RELOAD_STATUS=1
        When call configureSystemSlicePlacement
        The status should be failure
        The output should include "failed to reload systemd"
    End

    It 'retires unused legacy slices without masking or explicitly stopping units'
        When call retireLegacySystemSlices
        The status should be success
        The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.slice.d/90-aks-placement.conf" should include "StopWhenUnneeded=yes"
        The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/aks.slice.d/90-aks-placement.conf" should include "StopWhenUnneeded=yes"
    End

    It 'does not retire slices during an in-flight legacy log collection'
        COLLECTOR_CONTROL_GROUP="/aks.slice/aks-log.slice/aks-log-collector.slice/aks-log-collector.service"
        When call retireLegacySystemSlices
        The status should be success
        The output should include "Keeping aks.slice"
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/aks.slice.d" should not be exist
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log.slice.d" should not be exist
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/aks-log-collector.slice.d" should not be exist
    End

    It 'does not retire a disabled but still running legacy DNS service'
        LOCALDNS_CONTROL_GROUP="/localdns.slice/localdns.service"
        When call retireLegacySystemSlices
        The status should be success
        The output should include "Keeping localdns.slice"
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.slice.d" should not be exist
    End

    It 'does not create cleanup drop-ins for inactive implicit legacy slices'
        LEGACY_SLICE_ACTIVE_STATE=inactive
        When call retireLegacySystemSlices
        The status should be success
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.slice.d" should not be exist
        The path "${SYSTEM_SLICE_SYSTEMD_DIR}/aks.slice.d" should not be exist
    End

    It 'reconciles a real PIS node before cluster integration without running basePrep'
        eval "$(sed -n '/^function nodePrep {/,/^}/p' ./parts/linux/cloud-init/artifacts/cse_main.sh)"
        SHOULD_ENABLE_LOCALDNS=true
        LOCALDNS_CONTROL_GROUP="/localdns.slice/localdns.service"
        logs_to_events() {
            shift
            if [ "$1" = "configureAzureJson" ]; then
                echo "Reached cluster integration"
                exit 0
            fi
            "$@"
        }
        When run nodePrep
        The status should be success
        The output should include "restart 3 5 120 localdns"
        The output should include "Reached cluster integration"
        The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/localdns.service.d/90-aks-placement.conf" should include "Slice=system-localdns.slice"
        The contents of file "${SYSTEM_SLICE_SYSTEMD_DIR}/system-localdns.slice" should include "MemoryMax=128M"
    End

    Describe 'reconcileLocalDNSPlacement'
        It 'uses the normal DNS restart handoff on a cached legacy VHD'
            SHOULD_ENABLE_LOCALDNS=true
            LOCALDNS_CONTROL_GROUP="/localdns.slice/localdns.service"
            When call reconcileLocalDNSPlacement
            The status should be success
            The output should include "restart 3 5 120 localdns"
        End

        It 'moves an active legacy fallback through the DNS start handoff'
            SHOULD_ENABLE_LOCALDNS=true
            FALLBACK_CONTROL_GROUP="/localdns.slice/localdns-fallback.service"
            When call reconcileLocalDNSPlacement
            The status should be success
            The output should include "restart 3 5 120 localdns"
        End

        It 'does not restart DNS that is already nested'
            SHOULD_ENABLE_LOCALDNS=true
            LOCALDNS_CONTROL_GROUP="/system.slice/system-localdns.slice/localdns.service"
            When call reconcileLocalDNSPlacement
            The status should be success
            The output should be blank
        End

        It 'does not activate disabled DNS'
            SHOULD_ENABLE_LOCALDNS=false
            LOCALDNS_CONTROL_GROUP="/localdns.slice/localdns.service"
            When call reconcileLocalDNSPlacement
            The status should be success
            The output should be blank
        End

        It 'propagates DNS restart failure'
            SHOULD_ENABLE_LOCALDNS=true
            LOCALDNS_CONTROL_GROUP="/localdns.slice/localdns.service"
            RESTART_STATUS=1
            When call reconcileLocalDNSPlacement
            The status should be failure
            The output should include "Recreating LocalDNS"
        End
    End
End

Describe 'LocalDNS slice compatibility with an older provisioner'
    Include "./parts/linux/cloud-init/artifacts/localdns.sh"

    setup() {
        TEST_ROOT=$(mktemp -d)
        LOCALDNS_SLICE_PATH="${TEST_ROOT}"
        SYSTEMCTL_RELOAD_STATUS=0
        printf '[Slice]\nMemoryMax=128M\nCPUQuota=250%%\n' > "${TEST_ROOT}/localdns.slice"
    }

    cleanup() { rm -rf "${TEST_ROOT}"; }
    BeforeEach 'setup'
    AfterEach 'cleanup'

    systemctl() {
        case "$*" in
            "cat localdns.slice") cat "${TEST_ROOT}/localdns.slice" ;;
            "daemon-reload") echo reload; return "${SYSTEMCTL_RELOAD_STATUS}" ;;
            *) return 1 ;;
        esac
    }

    It 'creates the nested slice from the provisioner limits before CoreDNS starts'
        When call configure_localdns_slice
        The status should be success
        The output should equal reload
        The contents of file "${TEST_ROOT}/system-localdns.slice" should include "MemoryMax=128M"
        The contents of file "${TEST_ROOT}/system-localdns.slice" should include "CPUQuota=250%"
    End

    It 'does not reload unchanged limits on a service restart'
        cp "${TEST_ROOT}/localdns.slice" "${TEST_ROOT}/system-localdns.slice"
        When call configure_localdns_slice
        The status should be success
        The output should be blank
    End

    It 'reports a reload failure before starting CoreDNS'
        SYSTEMCTL_RELOAD_STATUS=1
        When call configure_localdns_slice
        The status should be failure
        The output should include "Failed to reload LocalDNS slice configuration"
    End
End
