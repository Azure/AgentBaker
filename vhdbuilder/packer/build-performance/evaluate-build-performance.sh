#!/bin/bash

log_and_exit () {
  local FILE=${1}
  local ERR=${2}
  local SHOW_FILE=${3:-false}
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${FILE} ${ERR}. Skipping build performance evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  if [[ ${SHOW_FILE} == true ]]; then
    cat ${FILE}
  fi
  exit 0
}

if [[ ! -f ${BUILD_PERF_DATA_FILE} ]]; then
  log_and_exit ${BUILD_PERF_DATA_FILE} "not found"
fi

SCRIPT_COUNT=$(jq -e 'keys | length' ${BUILD_PERF_DATA_FILE})
if [[ $? -ne 0 ]]; then
  log_and_exit ${BUILD_PERF_DATA_FILE} "contains invalid json" true
fi

if [[ ${SCRIPT_COUNT} -eq 0 ]]; then
  log_and_exit ${BUILD_PERF_DATA_FILE} "contains no scripts"
fi

echo -e "\nGenerating build performance data for ${SIG_IMAGE_NAME}...\n"

jq --arg sig "${SIG_IMAGE_NAME}" \
  --arg arch "${ARCHITECTURE}" \
  --arg build_id "${BUILD_ID}" \
  --arg date "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg status "${JOB_STATUS}" \
  --arg branch "${GIT_BRANCH}" \
  --arg commit "${GIT_VERSION}" \
  '{sig_image_name: $sig, architecture: $arch, build_id: $build_id, build_datetime: $date,
  build_status: $status, branch: $branch, commit: $commit, scripts: .}' \
  ${BUILD_PERF_DATA_FILE} > ${SIG_IMAGE_NAME}-build-performance.json

rm ${BUILD_PERF_DATA_FILE}

echo "##[group]Build Information"
jq -C '. | {sig_image_name, architecture, build_id, build_datetime, build_status, branch, commit}' ${SIG_IMAGE_NAME}-build-performance.json
echo "##[endgroup]"

scripts=()
for entry in $(jq -rc '.scripts | to_entries[]' ${SIG_IMAGE_NAME}-build-performance.json); do
  scripts+=("$(echo "$entry" | jq -r '.key')")
done

for script in "${scripts[@]}"; do
  echo "##[group]${script}"
  jq -C ".scripts.\"$script\"" ${SIG_IMAGE_NAME}-build-performance.json
  echo "##[endgroup]"
done

echo -e "\nRunning build performance evaluation program...\n"
mv ${SIG_IMAGE_NAME}-build-performance.json vhdbuilder/packer/build-performance
pushd vhdbuilder/packer/build-performance
  ./buildPerformance
  rm ${SIG_IMAGE_NAME}-build-performance.json
popd

echo -e "\nBuild performance evaluation script completed."