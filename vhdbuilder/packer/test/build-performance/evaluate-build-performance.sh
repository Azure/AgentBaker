#!/bin/bash

echo "##[group]${SIG_IMAGE_NAME} build performance data"
echo "##[group]     Inner Group"
jq -C '.' vhd-build-performance-data.json
echo "##[endgroup]"
echo "##[endgroup]"
echo "VHD Build Performance Data generated."