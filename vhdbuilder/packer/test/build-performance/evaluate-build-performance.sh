#!/bin/bash

# cd into correct directory, cat out VHD performance
# build go program
# execute go program
# apply warnings, if necessary


echo -e "\n${SIG_IMAGE_NAME} build performance data:\n" 
cat ${VHD_BUILD_PERFORMANCE_DATA_FILE} | jq -C .