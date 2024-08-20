#!/bin/bash

echo -e "\nGenerating build performance data for ${SIG_IMAGE_NAME}...\n"

jq --arg sig "${SIG_IMAGE_NAME}" \
  --arg arch "${ARCHITECTURE}" \
  --arg build_id "${BUILD_ID}" \
  --arg date "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
  --arg status "${JOB_STATUS}" \
  --arg uri "${BUILD_URI}" \
  --arg branch "${GIT_BRANCH}" \
  --arg commit "${GIT_VERSION}" \
  "{sig_image_name: $sig, architecture: $arch, build_id: $build_id, build_datetime: $date, '\'
  build_status: $status, build_uri: $uri, branch: $branch, commit: $commit, scripts: .}"
  ${BUILD_PERF_DATA_FILE} > ${SIG_IMAGE_NAME}-build-performance.json

echo "##[group]Build Information"
jq -C '. | {sig_image_name, architecture, build_id, build_datetime, build_status, build_uri, git_branch, commit}' ${SIG_IMAGE_NAME}-build-performance.json
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
echo -e "\n\n"

#mv ${SIG_IMAGE_NAME}-build-performance.json vhdbuilder/packer/test/build-performance
#pushd vhdbuilder/packer/test/build-performance 
  #chmod +x PerformanceDataIngestor
  #./PerformanceDataIngestor
#popd

echo -e "\nBuild performance evaluation script completed."