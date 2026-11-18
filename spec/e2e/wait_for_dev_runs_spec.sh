#!/usr/bin/env shellspec
# shellcheck shell=bash
# ShellSpec supplies parameter rows and reads these variables through Include.
# shellcheck disable=SC2034,SC2215,SC2286,SC2288

Describe 'wait_for_dev_runs'
  Include './.pipelines/scripts/wait_for_dev_runs.sh'

  setup_wait() {
    SYSTEM_COLLECTIONURI='https://dev.azure.com/example/'
    SYSTEM_TEAMPROJECTID='project-id'
    SYSTEM_ACCESSTOKEN='test-token'
    DEV_PIPELINE_DEFINITION_IDS='443878,474207'
    DEV_RUN_WAIT_TIMEOUT_SECONDS=120
    DEV_RUN_POLL_INTERVAL_SECONDS=60
    MOCK_ACTIVE_STATUS=''
    MOCK_ACTIVE_DEFINITION=443878
    MOCK_CLEAR_AFTER=1
    MOCK_POLLS=0
    MOCK_ERROR_STATUS=''
    MOCK_RESPONSE=''
    MOCK_SLEEP_ERROR=false
    SECONDS=0
  }

  BeforeEach 'setup_wait'

  # Reject an unscoped or mutating request instead of calling Azure DevOps.
  # shellcheck disable=SC2329
  curl() {
    local arguments expected status='' arg
    arguments=$(printf '%s\n' "$@")
    for expected in \
      --get --fail --max-time \
      'Authorization: Bearer test-token' \
      'definitions=443878,474207' \
      'branchName=refs/heads/dev' \
      "\$top=1" 'api-version=7.1' \
      'https://dev.azure.com/example/project-id/_apis/build/builds'; do
      if ! grep -Fxq -- "${expected}" <<< "${arguments}"; then
        echo "Unexpected build-list request." >&2
        return 2
      fi
    done
    for arg in "$@"; do
      case "${arg}" in
        statusFilter=inProgress) status=inProgress ;;
        statusFilter=cancelling) status=cancelling ;;
        --request|--data|--data-raw) return 2 ;;
      esac
    done
    if [ -z "${status}" ]; then
      echo "Unexpected status filter." >&2
      return 2
    fi
    if [ "${MOCK_ERROR_STATUS}" = "${status}" ]; then
      return 22
    fi
    if [ -n "${MOCK_RESPONSE}" ]; then
      printf '%s\n' "${MOCK_RESPONSE}"
    elif [ "${status}" = "${MOCK_ACTIVE_STATUS}" ] && [ "${MOCK_POLLS}" -lt "${MOCK_CLEAR_AFTER}" ]; then
      printf '{"value":[{"id":101,"definition":{"id":%s},"sourceBranch":"refs/heads/dev","status":"%s"}]}\n' \
        "${MOCK_ACTIVE_DEFINITION}" "${status}"
    else
      printf '{"value":[]}\n'
    fi
  }

  # Advance the wait clock without sleeping or changing external state.
  # shellcheck disable=SC2329
  sleep() {
    if [ "${MOCK_SLEEP_ERROR}" = true ]; then
      return 130
    fi
    SECONDS=$((SECONDS + $1))
    MOCK_POLLS=$((MOCK_POLLS + 1))
  }

  It 'continues immediately when neither status has an active dev run'
    When call wait_for_dev_runs
    The status should be success
    The output should include 'No active dev GPU runs remain.'
    The output should not include 'Checking again'
    The output should not include 'test-token'
    The stderr should be blank
  End

  It 'accepts a collection URI without a trailing slash'
    SYSTEM_COLLECTIONURI='https://dev.azure.com/example'
    When call wait_for_dev_runs
    The status should be success
    The output should include 'No active dev GPU runs remain.'
    The stderr should be blank
  End

  It 'uses the default three-hour timeout and one-minute poll interval'
    unset DEV_RUN_WAIT_TIMEOUT_SECONDS DEV_RUN_POLL_INTERVAL_SECONDS
    MOCK_ACTIVE_STATUS=inProgress
    When call wait_for_dev_runs
    The status should be success
    The output should include 'Waiting up to 10800s'
    The output should include 'Checking again in 60s.'
    The output should include 'No active dev GPU runs remain.'
    The stderr should be blank
  End

  It 'works with the strict shell options used by the script entry point'
    strict_wait() (
      set -euo pipefail
      wait_for_dev_runs
    )
    MOCK_ACTIVE_STATUS=cancelling
    When call strict_wait
    The status should be success
    The output should include 'Checking again in 60s.'
    The output should include 'No active dev GPU runs remain.'
    The stderr should be blank
  End

  Describe 'active dev runs'
    Parameters
      inProgress 443878
      cancelling 443878
      inProgress 474207
      cancelling 474207
    End

    It "waits for $1 runs from pipeline $2 to finish"
      MOCK_ACTIVE_STATUS="$1"
      MOCK_ACTIVE_DEFINITION="$2"
      When call wait_for_dev_runs
      The status should be success
      The output should include "Dev run 101 (pipeline $2) is $1."
      The output should include 'Checking again in 60s.'
      The output should include 'No active dev GPU runs remain.'
      The stderr should be blank
    End
  End

  It 'fails on timeout instead of starting GPU tests with dev runs active'
    MOCK_ACTIVE_STATUS=inProgress
    MOCK_CLEAR_AFTER=100
    When call wait_for_dev_runs
    The status should be failure
    The output should include 'Dev run 101'
    The output should not include 'No active dev GPU runs remain.'
    The stderr should include 'Timed out waiting for active dev runs after 120s.'
  End

  It 'limits the final sleep to the remaining wait time'
    MOCK_ACTIVE_STATUS=cancelling
    MOCK_CLEAR_AFTER=100
    DEV_RUN_WAIT_TIMEOUT_SECONDS=5
    When call wait_for_dev_runs
    The status should be failure
    The output should include 'Checking again in 5s.'
    The output should not include 'Checking again in 60s.'
    The stderr should include 'Timed out waiting for active dev runs after 5s.'
  End

  Describe 'API errors'
    Parameters
      inProgress
      cancelling
    End

    It "does not treat a failed $1 request as an empty result"
      MOCK_ERROR_STATUS="$1"
      When call wait_for_dev_runs
      The status should be failure
      The output should include 'Waiting up to'
      The output should not include 'No active dev GPU runs remain.'
      The stderr should include "Cannot check $1 dev runs. GPU tests will not start."
      The stderr should not include 'test-token'
    End
  End

  Describe 'invalid API responses'
    Parameters
      'not json'
      '{}'
      '{"value":null}'
      '{"value":[{}]}'
      '{"value":[{"id":101,"definition":{"id":443878},"sourceBranch":"refs/heads/main","status":"inProgress"}]}'
      '{"value":[{"id":101,"definition":{"id":443878},"sourceBranch":"refs/pull/1/merge","status":"inProgress"}]}'
      '{"value":[{"id":101,"definition":{"id":443878},"sourceBranch":"refs/heads/dev","status":"notStarted"}]}'
      '{"value":[{"id":101,"definition":{"id":999999},"sourceBranch":"refs/heads/dev","status":"inProgress"}]}'
    End

    It "fails closed for response $1"
      MOCK_RESPONSE="$1"
      When call wait_for_dev_runs
      The status should be failure
      The output should include 'Waiting up to'
      The output should not include 'No active dev GPU runs remain.'
      The stderr should include 'Invalid build-list response.'
    End
  End

  It 'requires a token before making an API request'
    unset SYSTEM_ACCESSTOKEN
    When call wait_for_dev_runs
    The status should be failure
    The output should be blank
    The stderr should include 'SYSTEM_ACCESSTOKEN are required.'
  End

  Describe 'invalid pipeline IDs'
    Parameters
      ''
      '443878,'
      '443878,abc'
      '443878,0'
    End

    It "rejects IDs '$1' before making an API request"
      DEV_PIPELINE_DEFINITION_IDS="$1"
      When call wait_for_dev_runs
      The status should be failure
      The output should be blank
      The stderr should include 'DEV_PIPELINE_DEFINITION_IDS must be a comma-separated list'
    End
  End

  Describe 'invalid wait durations'
    Parameters
      0 60
      -1 60
      120 0
      invalid 60
      120 1.5
    End

    It "rejects timeout=$1 and interval=$2"
      DEV_RUN_WAIT_TIMEOUT_SECONDS="$1"
      DEV_RUN_POLL_INTERVAL_SECONDS="$2"
      When call wait_for_dev_runs
      The status should be failure
      The output should be blank
      The stderr should include 'Wait timeout and poll interval must be positive integer seconds.'
    End
  End

  It 'does not continue after an interrupted sleep'
    MOCK_ACTIVE_STATUS=inProgress
    MOCK_SLEEP_ERROR=true
    When call wait_for_dev_runs
    The status should be failure
    The output should include 'Checking again in 60s.'
    The output should not include 'No active dev GPU runs remain.'
    The stderr should be blank
  End
End
