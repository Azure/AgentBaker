#!/bin/bash

echo "##[group]${SIG_IMAGE_NAME} build performance data"
jq -C '.' vhd-build-performance-data.json
echo "##[endgroup]"
echo "VHD Build Performance Data generated."