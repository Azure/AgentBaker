#!/bin/bash

Describe 'systemctlEnableAndStartNoBlock'
    Include "./parts/linux/cloud-init/artifacts/cse_helpers.sh"

    setup_systemctl() {
        unset CSE_STARTTIME_SECONDS
        enabled=false
        active=false
        disk_config=new
        loaded_config=old
        running_config=old
        reloads=0
        restarts=0
        sleeps=0
        failed_operation=""
        failures=0
        failure_attempts=0
    }
    BeforeEach setup_systemctl

    timeout() {
        echo "timeout $*"
        shift
        "$@"
    }

    sleep() {
        sleeps=$((sleeps + 1))
    }

    journalctl() {
        echo "journalctl $*"
    }

    systemctl() {
        echo "systemctl $*"
        if [ "$1" = "$failed_operation" ] && [ "$failure_attempts" -lt "$failures" ]; then
            failure_attempts=$((failure_attempts + 1))
            return 1
        fi
        case "$1" in
            enable)
                enabled=true
                if [ "$2" != "--no-reload" ]; then
                    reloads=$((reloads + 1))
                    loaded_config="$disk_config"
                fi
                ;;
            daemon-reload)
                reloads=$((reloads + 1))
                loaded_config="$disk_config"
                ;;
            restart)
                active=true
                running_config="$loaded_config"
                restarts=$((restarts + 1))
                ;;
            status) return 0 ;;
            *) return 1 ;;
        esac
    }

    Describe 'successful setup'
        Parameters
            "shellspec-test.service" false
            "shellspec-test.service" true
            "shellspec-test.timer" false
            "shellspec-test.timer" true
            "shellspec-test.socket" false
            "shellspec-test.socket" true
        End

        It "enables and restarts $1 with fresh configuration when active=$2"
            active="$2"
            enabled="$2"
            When call systemctlEnableAndStartNoBlock "$1" 240
            The status should be success
            The variable enabled should equal true
            The variable active should equal true
            The variable running_config should equal new
            The variable reloads should equal 1
            The variable restarts should equal 1
            The output should equal "timeout 25 systemctl enable --no-reload $1
systemctl enable --no-reload $1
Executed \"systemctl enable --no-reload $1\" 1 times.
timeout 240 systemctl daemon-reload
systemctl daemon-reload
timeout 240 systemctl restart --no-block $1
systemctl restart --no-block $1"
        End
    End

    It 'restarts an active unit again to apply a later configuration change'
        configure_twice() {
            systemctlEnableAndStartNoBlock shellspec-test.service 30 || return
            disk_config=updated
            systemctlEnableAndStartNoBlock shellspec-test.service 30
        }
        When call configure_twice
        The status should be success
        The variable enabled should equal true
        The variable active should equal true
        The variable running_config should equal updated
        The variable reloads should equal 2
        The variable restarts should equal 2
        The output should include "systemctl restart --no-block shellspec-test.service"
    End

    It 'retries enablement without restarting or reloading until it succeeds'
        failed_operation=enable
        failures=1
        When call systemctlEnableAndStartNoBlock shellspec-test.service 30
        The status should be success
        The variable failure_attempts should equal 1
        The variable sleeps should equal 1
        The variable reloads should equal 1
        The variable restarts should equal 1
        The output should include 'Executed "systemctl enable --no-reload shellspec-test.service" 2 times.'
    End

    It 'preserves restart retries, reloads, and retry diagnostics'
        failed_operation=restart
        failures=1
        When call systemctlEnableAndStartNoBlock shellspec-test.service 30
        The status should be success
        The variable failure_attempts should equal 1
        The variable sleeps should equal 1
        The variable reloads should equal 2
        The variable restarts should equal 1
        The output should include "systemctl status shellspec-test.service --no-pager -l"
        The output should include "journalctl -u shellspec-test.service"
    End

    Describe 'failure propagation'
        Parameters
            "enable" 120 false 0 "could not be enabled by systemctl"
            "restart" 100 true 100 "could not be enqueued for startup"
        End

        It "returns failure after $2 $1 attempts without exiting the caller"
            failed_operation="$1"
            failures="$2"
            call_and_continue() {
                local result=0
                systemctlEnableAndStartNoBlock shellspec-test.service 30 || result=$?
                echo "caller continued: enabled=$enabled reloads=$reloads attempts=$failure_attempts"
                return "$result"
            }
            When run call_and_continue
            The status should equal 1
            The output should include "caller continued: enabled=$3 reloads=$4 attempts=$2"
            The output should include "shellspec-test.service $5"
            The stderr should be defined
        End
    End

    It 'keeps the enablement retry deadline and does not exit the caller'
        CSE_STARTTIME_SECONDS=1
        call_after_deadline() {
            local result=0
            systemctlEnableAndStartNoBlock shellspec-test.service 30 || result=$?
            echo "caller continued"
            return "$result"
        }
        When run call_after_deadline
        The status should equal 1
        The output should include "caller continued"
        The output should not include "timeout"
        The stderr should include "CSE timeout approaching, exiting early."
    End
End
