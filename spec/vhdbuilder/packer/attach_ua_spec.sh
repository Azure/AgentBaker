#!/bin/bash
# ShellSpec parameter rows are data, not shell commands.
# shellcheck disable=SC2329,SC2288

Describe 'attachUA'
  # Extract only the functions under test; never execute the image-build script.
  BeforeAll "eval \"\$(sed -n '/^ubuntuProESMState()/,/^}$/p;/^attachUA()/,/^}$/p' vhdbuilder/scripts/linux/ubuntu/tool_installs_ubuntu.sh)\""

  setup_ua() {
    TEST_DIR="$(mktemp -d)"
    TRACE="${TEST_DIR}/commands"
    : > "${TRACE}"
    UA_TOKEN="unit-test-only-secret"
    ERR_UA_ATTACH=182
    SUCCESS='{"_schema_version":"0.1","result":"success","errors":[],"failed_services":[],"processed_services":[]}'
    UNATTACHED='{"_schema_version":"0.1","result":"success","errors":[],"execution_status":"inactive","attached":false,"services":[]}'
    DISABLED='{"_schema_version":"0.1","result":"success","errors":[],"execution_status":"inactive","attached":true,"contract":{"id":"FAKE-PRO-PRIVATE-STATE"},"services":[{"name":"esm-apps","entitled":"yes","status":"disabled"},{"name":"esm-infra","entitled":"yes","status":"disabled"},{"name":"livepatch","entitled":"yes","status":"disabled"}]}'
    READY="$(printf '%s' "${DISABLED}" | jq -c '.services |= map(if .name != "livepatch" then .status = "enabled" else . end)')"
    APPS_READY="$(printf '%s' "${DISABLED}" | jq -c '.services[0].status = "enabled"')"
    TRANSIENT='{"_schema_version":"0.1","result":"failure","errors":[{"type":"system","service":null,"message_code":"external-api-error","message":"FAKE-PRO-PRIVATE-STATE","additional_info":{"code":503,"body":"unit-test-only-secret"}}],"failed_services":[],"processed_services":[]}'
    SERVICE_TRANSIENT="$(printf '%s' "${TRANSIENT}" | jq -c '.errors[0] += {"type":"service","service":"esm-infra"} | .failed_services = ["esm-infra"] | .processed_services = ["esm-apps"]')"
    STATUS_RESPONSES=("${UNATTACHED}" "${DISABLED}" "${READY}")
    STATUS_CODES=(0 0 0)
    ATTACH_RESPONSES=("${SUCCESS}")
    ATTACH_CODES=(0)
    ENABLE_RESPONSES=("${SUCCESS}")
    ENABLE_CODES=(0)
  }
  cleanup_ua() {
    rm -f "${TRACE}"
    rmdir "${TEST_DIR}"
  }
  BeforeEach 'setup_ua'
  AfterEach 'cleanup_ua'

  # All external operations are mocked. The command log survives command substitutions,
  # while stdout/stderr deliberately contain private canaries to exercise log redaction.
  ua() {
    local index
    case "$1" in
      status)
        [ "$*" = "status --all --format json" ] || return 99
        index="$(grep -c '^status$' "${TRACE}")" || :
        echo status >> "${TRACE}"
        printf '%s\n' "${STATUS_RESPONSES[$index]:-}"
        return "${STATUS_CODES[$index]:-99}"
        ;;
      attach)
        [ "$#" -eq 5 ] && [ "$2 $3 $4" = "--no-auto-enable --format json" ] && [ "$5" = "${UA_TOKEN}" ] || return 99
        index="$(grep -c '^attach$' "${TRACE}")" || :
        echo attach >> "${TRACE}"
        printf '%s\n' "${ATTACH_RESPONSES[$index]:-}"
        echo "unit-test-only-secret FAKE-PRO-PRIVATE-STATE" >&2
        return "${ATTACH_CODES[$index]:-99}"
        ;;
      enable)
        [ "$2 $3 $4" = "--assume-yes --format json" ] || return 99
        shift 4
        case "$*" in
          "esm-apps esm-infra"|"esm-apps"|"esm-infra") ;;
          *) echo "UNEXPECTED SERVICE $*" >> "${TRACE}"; return 99 ;;
        esac
        index="$(grep -c '^enable ' "${TRACE}")" || :
        echo "enable $*" >> "${TRACE}"
        printf '%s\n' "${ENABLE_RESPONSES[$index]:-}"
        return "${ENABLE_CODES[$index]:-99}"
        ;;
      *) echo "UNEXPECTED UA COMMAND $*" >> "${TRACE}"; return 99 ;;
    esac
  }
  timeout() {
    case "$1" in 120|1000) ;; *) return 99 ;; esac
    shift
    "$@"
  }
  sleep() { echo "sleep $*" >> "${TRACE}"; }

  It 'attaches without auto-enable and enables exactly both ESM services, with no backoff or detach'
    When run attachUA
    The status should be success
    The output should include 'Ubuntu Pro esm-apps and esm-infra are enabled'
    The output should not include 'unit-test-only-secret'
    The output should not include 'FAKE-PRO-PRIVATE-STATE'
    The stderr should eq ''
    The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra
status"
  End

  It 'recovers partial attachment without repeating attach or detaching'
    ATTACH_RESPONSES=("${TRANSIENT}")
    ATTACH_CODES=(1)
    When run attachUA
    The status should be success
    The output should include 'recovering once after 10 seconds'
    The stderr should eq ''
    The contents of file "${TRACE}" should eq "status
attach
sleep 10
status
enable esm-apps esm-infra
status"
  End

  It 'retries an explicitly transient unattached failure only once'
    STATUS_RESPONSES=("${UNATTACHED}" "${UNATTACHED}" "${DISABLED}" "${READY}")
    STATUS_CODES=(0 0 0 0)
    ATTACH_RESPONSES=("${TRANSIENT}" "${SUCCESS}")
    ATTACH_CODES=(1 0)
    When run attachUA
    The status should be success
    The output should include 'recovering once after 10 seconds'
    The stderr should eq ''
    The contents of file "${TRACE}" should eq "status
attach
sleep 10
status
attach
status
enable esm-apps esm-infra
status"
  End

  It 'fails when the single attach retry is exhausted'
    STATUS_RESPONSES=("${UNATTACHED}" "${UNATTACHED}")
    ATTACH_RESPONSES=("${TRANSIENT}" "${TRANSIENT}")
    ATTACH_CODES=(1 1)
    When run attachUA
    The status should eq 182
    The output should include 'recovering once'
    The stderr should include 'no safe recovery remaining'
    The contents of file "${TRACE}" should eq "status
attach
sleep 10
status
attach"
  End

  It 'recovers enablement by enabling only the missing service'
    STATUS_RESPONSES=("${UNATTACHED}" "${DISABLED}" "${APPS_READY}" "${READY}")
    STATUS_CODES=(0 0 0 0)
    ENABLE_RESPONSES=("${SERVICE_TRANSIENT}" "${SUCCESS}")
    ENABLE_CODES=(1 0)
    When run attachUA
    The status should be success
    The output should include 'recovering once'
    The stderr should eq ''
    The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra
sleep 10
status
enable esm-infra
status"
  End

  It 'accepts verified ESM readiness after a transient post-enable bookkeeping error'
    ENABLE_RESPONSES=("${TRANSIENT}")
    ENABLE_CODES=(1)
    When run attachUA
    The status should be success
    The output should include 'recovering once'
    The stderr should eq ''
    The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra
sleep 10
status"
  End

  Describe 'incomplete readiness after system-level enable failures'
    Parameters
      '.'
      '.errors += [{"type":"service","service":"esm-infra","message_code":"external-api-error","additional_info":{"code":503}}]'
    End
    It 'does not retry missing services when bookkeeping could have masked a permanent service error'
      ENABLE_RESPONSES=("$(printf '%s' "${TRANSIENT}" | jq -c "$1")")
      ENABLE_CODES=(1)
      STATUS_RESPONSES=("${UNATTACHED}" "${DISABLED}" "${APPS_READY}")
      When run attachUA
      The status should eq 182
      The output should include 'recovering once'
      The stderr should include 'readiness incomplete after an ambiguous enable failure'
      The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra
sleep 10
status"
    End
  End

  Describe 'incomplete or untrusted service failure results'
    Parameters
      '.failed_services += ["esm-apps"]'
      '.processed_services = []'
      '.processed_services += ["esm-infra"]'
      '.failed_services = null'
      '.processed_services = [1]'
      '.errors[0].service = "livepatch"'
      '.errors += [{"type":"service","service":"esm-apps","message_code":"service-not-entitled"}]'
    End
    It 'requires a classified cause for every failed service and results for every requested service'
      ENABLE_RESPONSES=("$(printf '%s' "${SERVICE_TRANSIENT}" | jq -c "$1")")
      ENABLE_CODES=(1)
      When run attachUA
      The status should eq 182
      The output should include 'enabling required Ubuntu Pro ESM services'
      The stderr should include 'no safe recovery remaining'
      The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra"
    End
  End

  It 'shares one recovery budget between attach and enable'
    ATTACH_RESPONSES=("${TRANSIENT}")
    ATTACH_CODES=(1)
    ENABLE_RESPONSES=("${TRANSIENT}")
    ENABLE_CODES=(1)
    When run attachUA
    The status should eq 182
    The output should include 'recovering once'
    The stderr should include 'Ubuntu Pro enable failed'
    The contents of file "${TRACE}" should eq "status
attach
sleep 10
status
enable esm-apps esm-infra"
  End

  It 'fails when required-service recovery also fails'
    STATUS_RESPONSES=("${UNATTACHED}" "${DISABLED}" "${APPS_READY}")
    ENABLE_RESPONSES=("${SERVICE_TRANSIENT}" "${SERVICE_TRANSIENT}")
    ENABLE_CODES=(1 1)
    When run attachUA
    The status should eq 182
    The output should include 'recovering once'
    The stderr should include 'Ubuntu Pro enable failed'
    The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra
sleep 10
status
enable esm-infra"
  End

  It 'does not run package upgrades when attachment fails, even in a conditional caller'
    ATTACH_RESPONSES=('{}')
    guarded_caller() {
      if attachUA; then
        echo unexpected-upgrade >> "${TRACE}"
      fi
      echo unexpected-continuation >> "${TRACE}"
    }
    When run guarded_caller
    The status should eq 182
    The output should include 'attaching ua'
    The stderr should include 'Invalid Ubuntu Pro attach success response'
    The contents of file "${TRACE}" should eq "status
attach"
  End

  Describe 'permanent or ambiguous errors'
    Parameters
      'attach-invalid-token'
      'attach-forbidden-expired'
      'attach-forbidden-not-yet'
      'attach-forbidden-never'
      'attach-experied-token'
      'attach-failure'
      'connectivity-error'
      'attach-failure-default-service'
      'attach-failure-unexpected-error'
      'service-not-entitled'
      'unknown-error'
    End
    It 'does not retry or normalize permanent, ambiguous or unknown attach errors'
      ATTACH_RESPONSES=("$(printf '%s' "${TRANSIENT}" | jq -c --arg code "$1" '.errors[0].message_code = $code')")
      ATTACH_CODES=(1)
      When run attachUA
      The status should eq 182
      The output should include 'attaching ua'
      The stderr should include 'no safe recovery remaining'
      The contents of file "${TRACE}" should eq "status
attach"
    End
  End

  Describe 'retryable HTTP statuses'
    Parameters
      500
      502
      503
      504
    End
    It 'allows recovery only for the selected numeric HTTP server failures'
      ATTACH_RESPONSES=("$(printf '%s' "${TRANSIENT}" | jq -c --argjson code "$1" '.errors[0].additional_info.code = $code')")
      ATTACH_CODES=(1)
      When run attachUA
      The status should be success
      The output should include 'recovering once'
      The stderr should eq ''
    End
  End

  Describe 'untrusted error metadata'
    Parameters
      '.errors[0].additional_info.code = 401'
      '.errors[0].additional_info.code = 403'
      '.errors[0].additional_info.code = 429'
      '.errors[0].additional_info.code = 501'
      '.errors[0].additional_info.code = "503"'
      '.errors[0].message_code = null'
      '.errors[0].type = "unknown"'
      'del(.errors[0].type)'
      '.errors[0] += {"type":"service","service":"esm-infra"}'
      '.errors += [{"message_code":"attach-invalid-token"}]'
      '.errors = []'
      '.errors = null'
      'del(._schema_version)'
      '._schema_version = "2.0"'
    End
    It 'fails closed on unclassified HTTP failures and unknown error schemas'
      ATTACH_RESPONSES=("$(printf '%s' "${TRANSIENT}" | jq -c "$1")")
      ATTACH_CODES=(1)
      When run attachUA
      The status should eq 182
      The output should include 'attaching ua'
      The stderr should include 'no safe recovery remaining'
      The contents of file "${TRACE}" should eq "status
attach"
    End
  End

  Describe 'nonstandard exit statuses'
    Parameters
      2
      4
      124
      137
    End
    It 'does not infer transience from already-attached, partial-success or timeout exit codes'
      ATTACH_RESPONSES=("${TRANSIENT}")
      ATTACH_CODES=("$1")
      When run attachUA
      The status should eq 182
      The output should include 'attaching ua'
      The stderr should include "exit $1"
      The contents of file "${TRACE}" should eq "status
attach"
    End
  End

  Describe 'invalid operation results'
    Parameters
      ''
      'not-json unit-test-only-secret'
      '{}'
      '{"_schema_version":"0.1","result":"failure","errors":[],"failed_services":[]}'
      '{"_schema_version":"0.1","result":"success","errors":[],"failed_services":["esm-infra"]}'
      '{"_schema_version":"0.1","result":"success","errors":[],"failed_services":[]} {}'
      '{"_schema_version":"0.1","result":"success","errors":[],"failed_services":[],"processed_services":["livepatch"]}'
    End
    It 'does not accept exit zero with missing, invalid or contradictory operation JSON'
      ATTACH_RESPONSES=("$1")
      When run attachUA
      The status should eq 182
      The output should include 'attaching ua'
      The stderr should include 'Invalid Ubuntu Pro attach success response'
      The stderr should not include 'unit-test-only-secret'
      The contents of file "${TRACE}" should eq "status
attach"
    End
  End

  Describe 'preexisting attachments'
    Parameters
      '.'
      '.services += [{"name":"fips-updates","entitled":"yes","status":"enabled"}]'
      '.contract.id = "different-contract"'
    End
    It 'refuses initially attached machines, including post-FIPS or different-contract state, without changes'
      STATUS_RESPONSES=("$(printf '%s' "${READY}" | jq -c "$1")")
      When run attachUA
      The status should eq 182
      The output should eq ''
      The stderr should include 'initially attached'
      The contents of file "${TRACE}" should eq 'status'
    End
  End

  Describe 'untrusted initial status'
    Parameters
      'del(.attached)'
      '.attached = "false"'
      '._schema_version = "unknown"'
      '.result = "failure"'
      '.errors = [{}]'
      '.execution_status = "active"'
      '.execution_status = "unknown"'
    End
    It 'fails before attachment when initial status is untrusted'
      STATUS_RESPONSES=("$(printf '%s' "${UNATTACHED}" | jq -c "$1")")
      When run attachUA
      The status should eq 182
      The output should eq ''
      The stderr should include 'Unable to determine initial Ubuntu Pro state'
      The contents of file "${TRACE}" should eq 'status'
    End
  End

  It 'does not mistake a failed status command for unattached even if its JSON looks valid'
    STATUS_CODES=(1)
    When run attachUA
    The status should eq 182
    The output should eq ''
    The stderr should include 'Unable to determine initial Ubuntu Pro state'
    The contents of file "${TRACE}" should eq 'status'
  End

  Describe 'required service readiness'
    Parameters
      '.services[1].status = "disabled"'
      '.services[1].status = "warning"'
      '.services[0].entitled = "no"'
      '.services = [.services[0]]'
      '.services += [.services[0]]'
      '.services = null'
      '.services = {"apps":.services[0],"infra":.services[1]}'
      '.attached = false'
    End
    It 'requires both entitled ESM services to be enabled before returning success'
      STATUS_RESPONSES=("${UNATTACHED}" "${DISABLED}" "$(printf '%s' "${READY}" | jq -c "$1")")
      When run attachUA
      The status should eq 182
      The output should include 'enabling required Ubuntu Pro ESM services'
      The stderr should be present
      The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra
status"
    End
  End

  It 'does not enable services after an unreadable partial-attachment state'
    ATTACH_RESPONSES=("${TRANSIENT}")
    ATTACH_CODES=(1)
    STATUS_RESPONSES=("${UNATTACHED}" '{}')
    When run attachUA
    The status should eq 182
    The output should include 'recovering once'
    The stderr should include 'Unable to determine Ubuntu Pro state after attach'
    The contents of file "${TRACE}" should eq "status
attach
sleep 10
status"
  End

  It 'fails a required ESM authorization error without retrying or reading it as success'
    ENABLE_RESPONSES=("$(printf '%s' "${TRANSIENT}" | jq -c '.errors[0].message_code = "service-not-entitled"')")
    ENABLE_CODES=(1)
    When run attachUA
    The status should eq 182
    The output should include 'enabling required Ubuntu Pro ESM services'
    The stderr should include 'Ubuntu Pro enable failed'
    The contents of file "${TRACE}" should eq "status
attach
status
enable esm-apps esm-infra"
  End

  run_with_xtrace() {
    set -x
    attachUA
    case "$-" in *x*) ;; *) return 99 ;; esac
    set +x
  }
  It 'keeps token and private JSON out of logs while preserving caller xtrace'
    ATTACH_RESPONSES=("${TRANSIENT}")
    ATTACH_CODES=(1)
    When run run_with_xtrace
    The status should be success
    The output should include 'esm-apps and esm-infra are enabled'
    The output should not include 'unit-test-only-secret'
    The output should not include 'FAKE-PRO-PRIVATE-STATE'
    The stderr should include 'set +x'
    The stderr should not include 'unit-test-only-secret'
    The stderr should not include 'FAKE-PRO-PRIVATE-STATE'
  End

  It 'fails before running Pro when the token is missing'
    UA_TOKEN=''
    When run attachUA
    The status should eq 182
    The output should eq ''
    The stderr should include 'requires a token and jq'
    The contents of file "${TRACE}" should eq ''
  End

  Describe 'Ubuntu build call paths'
    Skip if 'the existing build caller requires Bash 4 case conversion' [ "${BASH_VERSINFO[0]}" -lt 4 ]
    Parameters
      '20.04' 'false' 'pro'
      '20.04' 'true' 'pro'
      '22.04' 'true' 'pro'
      '24.04' 'true' 'pro'
      '22.04' 'false' 'no-pro'
      '24.04' 'false' 'no-pro'
    End

    run_ubuntu_upgrade_phase() {
      local UBUNTU_RELEASE="$1" OS_VERSION="$1" ENABLE_FIPS="$2" VHD_BUILD_TIMESTAMP=""
      local phase_script
      apt_get_update() { echo apt-update >> "${TRACE}"; }
      apt_get_dist_upgrade() { echo dist-upgrade >> "${TRACE}"; }
      installFIPS() { echo install-fips >> "${TRACE}"; }
      # Extract only the Ubuntu upgrade branch, including its final closing fi.
      # All mutating commands in this branch are mocked; no full script is sourced.
      phase_script="$(sed -n '/^  # Enable ESM only/,/^fi$/p' vhdbuilder/packer/pre-install-dependencies.sh)"
      [ -n "${phase_script}" ] || return 99
      eval "if false; then :; else
${phase_script}"
      set +x
    }

    It 'makes ESM ready before distro upgrades and retains the later FIPS phase'
      expected=''
      if [ "$3" = pro ]; then
        expected="status
attach
status
enable esm-apps esm-infra
status
"
      fi
      expected="${expected}apt-update
dist-upgrade"
      if [ "$2" = true ]; then
        expected="${expected}
install-fips"
      fi
      When run run_ubuntu_upgrade_phase "$1" "$2"
      The status should be success
      The output should not include 'unit-test-only-secret'
      The stderr should not include 'unit-test-only-secret'
      The contents of file "${TRACE}" should eq "${expected}"
    End
  End
End
