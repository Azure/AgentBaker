#!/bin/bash

echo -e "\nGenerating ${SIG_IMAGE_NAME} build performance data from ${BUILD_PERF_DATA_FILE}...\n"

jq --arg sig "${SIG_IMAGE_NAME}" \
--arg date "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
--arg commit "${GIT_BRANCH}" \
'. as $orig | {"sig_image_name":$sig, "build_datetime":$date, "commit":$commit, "scripts": ($orig | reduce .[] as $item ({}; . + $item) | map_values(map_values(.total_time_elapsed)))}' \
${BUILD_PERF_DATA_FILE} > ${SIG_IMAGE_NAME}-build-performance.json

echo "##[group]Build Information"
jq -C '. | {sig_image_name, build_datetime, commit}' ${SIG_IMAGE_NAME}-build-performance.json
echo "##[endgroup]"

scripts=()
for key in $(jq -r '.scripts | keys[]' ${SIG_IMAGE_NAME}-build-performance.json); do
  scripts+=("$key")
done

for script in "${scripts[@]}"; do
  echo "##[group]${script}"
  jq -C ".scripts.\"$script\"" ${SIG_IMAGE_NAME}-build-performance.json
  echo "##[endgroup]"
done

mv ${SIG_IMAGE_NAME}-build-performance.json vhdbuilder/packer/test/build-performance
pushd vhdbuilder/packer/test/build-performance 
	go build -o kustoProgram main.go
  chmod +x kustoProgram
  ./kustoProgram
popd

echo -e "\nBuild performance evaluation script completed."