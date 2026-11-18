#!/usr/bin/env bash

# Wait for active refs/heads/dev runs in DEV_PIPELINE_DEFINITION_IDS (CSV) to
# finish. Requires curl, jq, and the Azure Pipelines SYSTEM_COLLECTIONURI,
# SYSTEM_TEAMPROJECTID, and SYSTEM_ACCESSTOKEN environment variables. The token
# needs permission to view builds in the selected pipelines, not to cancel them.
# Polls every 60 seconds for up to three hours; DEV_RUN_POLL_INTERVAL_SECONDS and
# DEV_RUN_WAIT_TIMEOUT_SECONDS can override these positive integer durations.
# Returns nonzero on timeout, an API error, or an invalid response. This is a
# read-only check, not a lock: another dev run can start after the check passes.
wait_for_dev_runs() {
  local definition_ids="${DEV_PIPELINE_DEFINITION_IDS:-}"
  local timeout_seconds="${DEV_RUN_WAIT_TIMEOUT_SECONDS:-10800}"
  local poll_seconds="${DEV_RUN_POLL_INTERVAL_SECONDS:-60}"

  if [ -z "${SYSTEM_COLLECTIONURI:-}" ] || [ -z "${SYSTEM_TEAMPROJECTID:-}" ] || [ -z "${SYSTEM_ACCESSTOKEN:-}" ]; then
    echo "##vso[task.logissue type=error]SYSTEM_COLLECTIONURI, SYSTEM_TEAMPROJECTID, and SYSTEM_ACCESSTOKEN are required." >&2
    return 1
  fi
  if ! printf '%s\n' "${definition_ids}" | grep -Eq '^[1-9][0-9]*(,[1-9][0-9]*)*$'; then
    echo "##vso[task.logissue type=error]DEV_PIPELINE_DEFINITION_IDS must be a comma-separated list of positive pipeline IDs." >&2
    return 1
  fi
  if ! printf '%s\n' "${timeout_seconds}:${poll_seconds}" | grep -Eq '^[1-9][0-9]*:[1-9][0-9]*$'; then
    echo "##vso[task.logissue type=error]Wait timeout and poll interval must be positive integer seconds." >&2
    return 1
  fi

  local builds_url="${SYSTEM_COLLECTIONURI%/}/${SYSTEM_TEAMPROJECTID}/_apis/build/builds"
  local deadline=$((SECONDS + timeout_seconds))
  local status response active_runs remaining sleep_seconds request_timeout

  echo "Waiting up to ${timeout_seconds}s for refs/heads/dev runs in pipelines ${definition_ids}."
  while true; do
    active_runs=false
    # A cancelling run can still be cleaning up GPU resources.
    for status in inProgress cancelling; do
      remaining=$((deadline - SECONDS))
      if [ "${remaining}" -le 0 ]; then
        echo "##vso[task.logissue type=error]Timed out waiting for active dev runs after ${timeout_seconds}s." >&2
        return 1
      fi
      request_timeout=$((remaining < 30 ? remaining : 30))

      # One matching run is enough to block the gate. Server-side filters and
      # $top=1 avoid scanning completed history or needing continuation pages.
      if ! response=$(curl --fail --silent --show-error --get \
        --connect-timeout 10 --max-time "${request_timeout}" \
        --header "Authorization: Bearer ${SYSTEM_ACCESSTOKEN}" \
        --data-urlencode "definitions=${definition_ids}" \
        --data-urlencode "branchName=refs/heads/dev" \
        --data-urlencode "statusFilter=${status}" \
        --data-urlencode "\$top=1" \
        --data-urlencode 'api-version=7.1' \
        "${builds_url}"); then
        echo "##vso[task.logissue type=error]Cannot check ${status} dev runs. GPU tests will not start." >&2
        return 1
      fi

      if ! jq -e --arg status "${status}" --arg ids "${definition_ids}" '
        type == "object" and (.value | type == "array") and
        all(.value[];
          (.id | type == "number") and
          (.definition.id | tostring | IN($ids | split(",")[])) and
          .sourceBranch == "refs/heads/dev" and .status == $status)
      ' <<< "${response}" >/dev/null 2>&1; then
        echo "##vso[task.logissue type=error]Invalid build-list response. GPU tests will not start." >&2
        return 1
      fi
      if [ "$(jq '.value | length' <<< "${response}")" -gt 0 ]; then
        active_runs=true
        jq -r '.value[] | "Dev run \(.id) (pipeline \(.definition.id)) is \(.status)."' <<< "${response}"
      fi
    done

    # Do not pass the gate if the API requests exhausted the wait budget.
    remaining=$((deadline - SECONDS))
    if [ "${remaining}" -gt 0 ] && [ "${active_runs}" = false ]; then
      echo "No active dev GPU runs remain."
      return 0
    fi
    if [ "${remaining}" -gt 0 ]; then
      sleep_seconds=$((remaining < poll_seconds ? remaining : poll_seconds))
      echo "Checking again in ${sleep_seconds}s."
      sleep "${sleep_seconds}" || return 1
    fi
  done
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  set -euo pipefail
  wait_for_dev_runs
fi
