#!/bin/sh

# Selects the Ubuntu GPU E2E scenario set. PRs default to the core suite; authors
# may opt into the full suite through the PR-template checkbox. Non-PR runs always
# use the full suite.

set -eu

CORE_GPU_E2E_TESTS='
  Test_Ubuntu2404_GPUA10
  Test_Ubuntu2404_NvidiaDevicePluginRunning
  Test_Ubuntu2404_NvidiaDevicePluginRunning_MIG
'
FULL_PROFILE_MARKER='- [x] run the full ubuntu gpu e2e suite for this pr <!-- gpu-e2e-profile:full -->'
readonly CORE_GPU_E2E_TESTS FULL_PROFILE_MARKER

log_warning() {
  echo "##vso[task.logissue type=warning]$1"
}

read_pr_body() {
  if [ -n "${GPU_E2E_PR_BODY:-}" ]; then
    printf '%s\n' "$GPU_E2E_PR_BODY"
    return
  fi

  repository_name="${BUILD_REPOSITORY_NAME:-Azure/AgentBaker}"
  case "$repository_name" in
    */*) ;;
    *)
      repository_name="Azure/${repository_name}"
      ;;
  esac
  pr_number="${SYSTEM_PULLREQUEST_PULLREQUESTNUMBER:-}"
  if [ -z "$pr_number" ]; then
    return 1
  fi

  if [ -n "${GITHUB_TOKEN:-}" ]; then
    curl \
      --fail \
      --silent \
      --show-error \
      --location \
      --retry 3 \
      --header 'Accept: application/vnd.github+json' \
      --header "Authorization: Bearer ${GITHUB_TOKEN}" \
      "https://api.github.com/repos/${repository_name}/pulls/${pr_number}" |
      jq -r '.body // ""'
    return
  fi

  curl \
    --fail \
    --silent \
    --show-error \
    --location \
    --retry 3 \
    --header 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/${repository_name}/pulls/${pr_number}" |
    jq -r '.body // ""'
}

profile=full
if [ "${BUILD_REASON:-}" = "PullRequest" ]; then
  profile=core
  if pr_body="$(read_pr_body)"; then
    normalized_pr_body="$(printf '%s\n' "$pr_body" | tr '[:upper:]' '[:lower:]')"
    if printf '%s\n' "$normalized_pr_body" | grep -Fqx -- "$FULL_PROFILE_MARKER"; then
      profile=full
    fi
  else
    log_warning "Unable to read PR body; running default core GPU E2E suite"
  fi
fi

if [ "$profile" = "core" ]; then
  tags_to_run=''
  for test_name in $CORE_GPU_E2E_TESTS; do
    if [ -n "$tags_to_run" ]; then
      tags_to_run="${tags_to_run},"
    fi
    tags_to_run="${tags_to_run}name=${test_name}"
  done
else
  tags_to_run='gpu=true'
fi

echo "Selected GPU E2E profile: $profile"
echo "TAGS_TO_RUN: $tags_to_run"
echo "##vso[task.setvariable variable=GPU_E2E_PROFILE]$profile"
echo "##vso[task.setvariable variable=TAGS_TO_RUN]$tags_to_run"
