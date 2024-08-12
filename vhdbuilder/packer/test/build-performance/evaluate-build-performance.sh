#!/bin/bash

echo -e "\nGenerating ${SIG_IMAGE_NAME} build performance data from ${BUILD_PERF_DATA_FILE}...\n"

scripts=()
for key in $(jq -r '.[] | keys[]' ${BUILD_PERF_DATA_FILE}); do
  scripts+=("$key")
done

for script in "${scripts[@]}"; do
  echo "##[group]${script}"
  jq -C ".[] | select(has(\"$script\"))" ${BUILD_PERF_DATA_FILE}
  echo "##[endgroup]"
done

jq --arg sig "${SIG_IMAGE_NAME}" \
--arg date "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
--arg commit "${GIT_VERSION}" \
'. as $orig | [{"sig_image_name":$sig}, {"build_datetime":$date}, {"commit":$commit}, {"scripts": $orig}]' \
${VHD_BUILD_PERFORMANCE_DATA_FILE} > ${SIG_IMAGE_NAME}-build-performance.json

pushd /home/packer
  chmod +x kustoProgram
  ./kustoProgram
popd

echo -e "\nBuild performance evaluation successfully completed"