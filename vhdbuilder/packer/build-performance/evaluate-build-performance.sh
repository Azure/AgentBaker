#!/bin/bash

if [[ ! -f ${BUILD_PERF_DATA_FILE} ]]; then
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${BUILD_PERF_DATA_FILE} not found. \
  Skipping build performance evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  exit 0
fi

SCRIPT_COUNT=$(jq -e 'keys | length' ${BUILD_PERF_DATA_FILE})
if [[ $? -ne 0 ]]; then
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${BUILD_PERF_DATA_FILE} contains invalid json. \
  Skipping build performance evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  exit 0
fi

echo "Script count is ${SCRIPT_COUNT}"
if [[ ${SCRIPT_COUNT} -eq 0 ]]; then
  echo "##vso[task.logissue type=warning;sourcepath=$(basename $0);]${BUILD_PERF_DATA_FILE} is empty. \
  Skipping build performance evaluation."
  echo "##vso[task.complete result=SucceededWithIssues;]"
  exit 0
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

rm ${SIG_IMAGE_NAME}-build-performance.json
echo -e "\nBuild performance evaluation script completed."