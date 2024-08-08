#!/bin/bash

echo -e "\n${SIG_IMAGE_NAME} build performance data:\n" 
cat ${VHD_BUILD_PERFORMANCE_DATA_FILE} | jq -C .