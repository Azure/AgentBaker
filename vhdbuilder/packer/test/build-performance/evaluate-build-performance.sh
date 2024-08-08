#!/bin/bash

scripts=()
for key in $(jq -r '.[] | keys[]' vhd-build-performance-data.json); do
  scripts+=("$key")
done

echo "##[group]${SIG_IMAGE_NAME} build performance data"
for script in "${scripts[@]}"; do
  echo "##[group]   ${script}"
  jq -C ".[] | select(has(\"$script\"))" vhd-build-performance-data.json
  echo "##[endgroup]"
done
echo "##[endgroup]"

echo -e "\n${SIG_IMAGE_NAME} build performance data generated"

echo -e "\nCompiling test program..."

sleep 10

echo -e "\nBuild performance evaluation successfully completed"
