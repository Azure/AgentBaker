#!/bin/bash

echo -e "\nGenerating ${SIG_IMAGE_NAME} build performance data from ${BUILD_PERF_DATA_FILE}...\n"

jq --arg sig_image_name "${SIG_IMAGE_NAME}" \
   --arg build_datetime "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
   --arg commit "$GIT_VERSION" \
   '{sig_image_name: $sig_image_name, build_datetime: $build_datetime, commit: $commit, scripts: .}' ${BUILD_PERF_DATA_FILE} \
   >> ${SIG_IMAGE_NAME}-build-performance.json

echo "##[group]Build Information"
jq -C '. | {sig_image_name, build_datetime, commit}' ${SIG_IMAGE_NAME}-build-performance.json
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