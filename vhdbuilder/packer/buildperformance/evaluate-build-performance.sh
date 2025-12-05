#!/bin/bash

log_and_exit () {
  local FILE=${1}
  local ERR=${2}
  local SHOW_FILE=${3:-false}
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${FILE} ${ERR}. Skipping build performance evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  if [ "${SHOW_FILE}" = "true" ]; then
    cat ${FILE}
  fi
  exit 0
}
