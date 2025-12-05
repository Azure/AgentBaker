#!/bin/bash
set -euo pipefail

readonly EXPECTED_MAJOR_VERSION=17
readonly VERSION_TOLERANCE=1
readonly GRID_VERSION_MIN=10
readonly GRID_VERSION_MAX=30

log_and_exit() {
  local FILE=${1}
  local ERR=${2}
  local SHOW_FILE=${3:-false}
  echo "##vso[task.logissue type=warning;sourcepath=$(basename "$0");]${FILE} ${ERR}. Skipping grid compatibility evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  if [ "${SHOW_FILE}" = "true" ]; then
    cat "${FILE}"
  fi
  exit 0
}
