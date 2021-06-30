#!/bin/bash

#enable MIG mode???
#nvidia-smi -mig 1
echo ${1}
MIG_PROFILE=${1}
echo "mig profile is ${MIG_PROFILE}"
if [ ${MIG_PROFILE} = "MIG1g" ] 
then
    nvidia-smi mig -cgi 19,19,19,19,19,19,19
    nvidia-smi mig -cci 
fi