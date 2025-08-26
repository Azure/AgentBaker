#!/bin/bash

log_and_exit () {
  local FILE=${1}
  local ERR=${2}
  local SHOW_FILE=${3:-false}
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${FILE} ${ERR}. Skipping grid compatibility evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  if [ "${SHOW_FILE}" = "true" ]; then
    cat ${FILE}
  fi
  exit 0
}

if [ ! -f "${GRID_COMPATIBILITY_DATA_FILE}" ]; then
  log_and_exit ${GRID_COMPATIBILITY_DATA_FILE} "not found"
fi

# Check if the file is valid JSON
jq -e . ${GRID_COMPATIBILITY_DATA_FILE} >/dev/null 2>&1
if [ "$?" -ne 0 ]; then
  log_and_exit ${GRID_COMPATIBILITY_DATA_FILE} "contains invalid json" true
fi

# Check if we have actual data
DATA_COUNT=$(jq -e 'keys | length' ${GRID_COMPATIBILITY_DATA_FILE} 2>/dev/null || echo "0")
if [ "${DATA_COUNT}" -eq 0 ]; then
  log_and_exit ${GRID_COMPATIBILITY_DATA_FILE} "contains no data"
fi

echo -e "\nGenerating grid compatibility data for ${SIG_IMAGE_NAME}...\n"

# Enrich the grid compatibility data with metadata similar to build performance
jq --arg sig_name "${SIG_IMAGE_NAME}" \
  --arg arch "${ARCHITECTURE}" \
  --arg captured_sig_version "${CAPTURED_SIG_VERSION}" \
  --arg build_id "${BUILD_ID}" \
  --arg date "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg status "${JOB_STATUS}" \
  --arg branch "${GIT_BRANCH}" \
  --arg commit "${GIT_VERSION}" \
  'to_entries | ([
  {key: "sig_image_name", value: $sig_name},
  {key: "architecture", value: $arch},
  {key: "captured_sig_version", value: $captured_sig_version},
  {key: "build_id", value: $build_id},
  {key: "build_datetime", value: $date},
  {key: "outcome", value: $status},
  {key: "branch", value: $branch},
  {key: "commit", value: $commit}
] + .) | from_entries' ${GRID_COMPATIBILITY_DATA_FILE} > ${SIG_IMAGE_NAME}-grid-compatibility.json

rm ${GRID_COMPATIBILITY_DATA_FILE}

echo "##[group]Grid Compatibility"
jq . -C ${SIG_IMAGE_NAME}-grid-compatibility.json
echo "##[endgroup]"

echo -e "\nENVIRONMENT is: ${ENVIRONMENT}"
if [ "${ENVIRONMENT,,}" != "tme" ]; then
  mv ${SIG_IMAGE_NAME}-grid-compatibility.json vhdbuilder/packer/gridcompatibility
  pushd vhdbuilder/packer/gridcompatibility || exit 0
    echo -e "\nRunning grid compatibility evaluation program...\n"
    if [ -n "${GRID_COMPATIBILITY_BINARY:-}" ] && [ -f "${GRID_COMPATIBILITY_BINARY}" ]; then
      chmod +x ${GRID_COMPATIBILITY_BINARY}
      ./${GRID_COMPATIBILITY_BINARY}
      rm ${GRID_COMPATIBILITY_BINARY}
    else
      echo "Grid compatibility binary not found or not specified: ${GRID_COMPATIBILITY_BINARY:-not_set}"
      echo "##vso[task.logissue type=warning;]Grid compatibility binary not available. Skipping evaluation."
      echo "This is expected during initial scaffolding setup."
    fi
  popd || exit 0
else
  echo -e "Skipping grid compatibility evaluation for tme environment"
fi

rm -f vhdbuilder/packer/gridcompatibility/${SIG_IMAGE_NAME}-grid-compatibility.json

echo -e "\nGrid compatibility evaluation script completed."