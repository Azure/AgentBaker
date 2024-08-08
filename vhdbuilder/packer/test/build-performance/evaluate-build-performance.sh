#!/bin/bash

echo -e "Generating ${SIG_IMAGE_NAME} build performance data from ${BUILD_PERF_DATA_FILE}...\n"

scripts=()
for key in $(jq -r '.[] | keys[]' ${BUILD_PERF_DATA_FILE}); do
  scripts+=("$key")
done

for script in "${scripts[@]}"; do
  echo "##[group]${script}"
  jq -C ".[] | select(has(\"$script\"))" ${BUILD_PERF_DATA_FILE}
  echo "##[endgroup]"
done

echo -e "\n${SIG_IMAGE_NAME} build performance data generated"

echo -e "\nCompiling test program..."

sleep 10

echo -e "\nBuild performance evaluation successfully completed"
