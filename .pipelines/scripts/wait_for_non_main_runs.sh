#!/usr/bin/env bash

# Wait for active runs outside refs/heads/main in GPU_PIPELINE_DEFINITION_IDS
# (CSV) to finish. Requires curl, jq, and the Azure Pipelines SYSTEM_COLLECTIONURI,
# SYSTEM_TEAMPROJECTID, and SYSTEM_ACCESSTOKEN variables. The token needs view-build
# permission. Polls every 60 seconds for up to three hours; override these positive
# integer durations with NON_MAIN_RUN_POLL_INTERVAL_SECONDS and
# NON_MAIN_RUN_WAIT_TIMEOUT_SECONDS. Returns nonzero on timeout or API errors.
# Uses a temporary response-header file under the working directory's tmp folder.
# This read-only check is not a lock: another run can start after it passes.
wait_for_non_main_runs() (
  local definition_ids="${GPU_PIPELINE_DEFINITION_IDS:-}"
  local timeout_seconds="${NON_MAIN_RUN_WAIT_TIMEOUT_SECONDS:-10800}"
  local poll_seconds="${NON_MAIN_RUN_POLL_INTERVAL_SECONDS:-60}"

  if [ -z "${SYSTEM_COLLECTIONURI:-}" ] || [ -z "${SYSTEM_TEAMPROJECTID:-}" ] || [ -z "${SYSTEM_ACCESSTOKEN:-}" ]; then
    echo "##vso[task.logissue type=error]SYSTEM_COLLECTIONURI, SYSTEM_TEAMPROJECTID, and SYSTEM_ACCESSTOKEN are required." >&2
    return 1
  fi
  if ! printf '%s\n' "${definition_ids}" | grep -Eq '^[1-9][0-9]*(,[1-9][0-9]*)*$'; then
    echo "##vso[task.logissue type=error]GPU_PIPELINE_DEFINITION_IDS must be a comma-separated list of positive pipeline IDs." >&2
    return 1
  fi
  if ! printf '%s\n' "${timeout_seconds}:${poll_seconds}" | grep -Eq '^[1-9][0-9]*:[1-9][0-9]*$'; then
    echo "##vso[task.logissue type=error]Wait timeout and poll interval must be positive integer seconds." >&2
    return 1
  fi

  local builds_url="${SYSTEM_COLLECTIONURI%/}/${SYSTEM_TEAMPROJECTID}/_apis/build/builds"
  local deadline=$((SECONDS + timeout_seconds))
  local status response blocking_runs active_runs remaining sleep_seconds request_timeout
  local response_headers continuation_token next_token
  local -a request_params
  mkdir -p "${PWD}/tmp" || return 1
  response_headers=$(mktemp "${PWD}/tmp/gpu-run-wait-headers.XXXXXX") || return 1
  trap 'rm -f -- "${response_headers}"' EXIT

  echo "Waiting up to ${timeout_seconds}s for non-main runs in pipelines ${definition_ids}."
  while true; do
    active_runs=false
    # A cancelling run can still be cleaning up GPU resources.
    for status in inProgress cancelling; do
      continuation_token=''
      while true; do
        remaining=$((deadline - SECONDS))
        if [ "${remaining}" -le 0 ]; then
          echo "##vso[task.logissue type=error]Timed out waiting for active non-main runs after ${timeout_seconds}s." >&2
          return 1
        fi
        request_timeout=$((remaining < 30 ? remaining : 30))
        request_params=(
          --data-urlencode "definitions=${definition_ids}"
          --data-urlencode "statusFilter=${status}"
          --data-urlencode "\$top=100"
          --data-urlencode 'api-version=7.1'
        )
        if [ -n "${continuation_token}" ]; then
          request_params+=(--data-urlencode "continuationToken=${continuation_token}")
        fi

        if ! response=$(curl --fail --silent --show-error --get \
          --connect-timeout 10 --max-time "${request_timeout}" \
          --header "Authorization: Bearer ${SYSTEM_ACCESSTOKEN}" \
          --dump-header "${response_headers}" \
          "${request_params[@]}" "${builds_url}"); then
          echo "##vso[task.logissue type=error]Cannot check ${status} runs. GPU tests will not start." >&2
          return 1
        fi
        if ! jq -e --arg status "${status}" --arg ids "${definition_ids}" '
          type == "object" and (.value | type == "array") and
          all(.value[];
            (.id | type == "number") and
            (.definition.id | tostring | IN($ids | split(",")[])) and
            (.sourceBranch | type == "string") and .status == $status)
        ' <<< "${response}" >/dev/null 2>&1; then
          echo "##vso[task.logissue type=error]Invalid build-list response. GPU tests will not start." >&2
          return 1
        fi

        blocking_runs=$(jq -r '.value[] | select(.sourceBranch != "refs/heads/main") |
          "GPU run \(.id) (\(.sourceBranch)) is \(.status)."' <<< "${response}")
        if [ -n "${blocking_runs}" ]; then
          active_runs=true
          printf '%s\n' "${blocking_runs}"
        fi

        # Main runs can occupy a whole page. Check every page before declaring
        # that no non-main run is active; do not rely on the first matching run.
        next_token=$(awk '
          tolower($0) ~ /^x-ms-continuationtoken:/ {
            sub(/^[^:]*:[[:space:]]*/, "")
            sub(/[[:space:]]*$/, "")
            token = $0
          }
          END { print token }
        ' "${response_headers}")
        [ -n "${next_token}" ] || break
        if [ "${next_token}" = "${continuation_token}" ]; then
          echo "##vso[task.logissue type=error]Repeated build-list continuation token. GPU tests will not start." >&2
          return 1
        fi
        continuation_token="${next_token}"
      done
    done

    remaining=$((deadline - SECONDS))
    if [ "${remaining}" -gt 0 ] && [ "${active_runs}" = false ]; then
      echo "No active non-main GPU runs remain."
      return 0
    fi
    if [ "${remaining}" -gt 0 ]; then
      sleep_seconds=$((remaining < poll_seconds ? remaining : poll_seconds))
      echo "Checking again in ${sleep_seconds}s."
      sleep "${sleep_seconds}" || return 1
    fi
  done
)

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  set -euo pipefail
  wait_for_non_main_runs
fi
