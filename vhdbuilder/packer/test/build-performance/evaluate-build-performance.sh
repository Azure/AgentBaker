#!/bin/bash

# cd into correct directory, cat out VHD performance
# build go program
# execute go program
# apply warnings, if necessary


echo -e "\n${SIG_IMAGE_NAME} build performance data:\n" 
cat ${VHD_BUILD_PERFORMANCE_DATA}

cd vhdbuilder/packer/test/build-performance
echo -e "\nCompiling vhd build performance test program..\n"

go build main.go

./main.go

echo -e "\nVHD build performance evaluation complete.\n"