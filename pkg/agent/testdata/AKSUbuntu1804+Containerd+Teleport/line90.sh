#!/bin/bash

#enable MIG mode???
#nvidia-smi -mig 1
echo ${1}
echo ${2}
MIG_PROFILE=${2}
echo "mig profile is ${MIG_PROFILE}"
if [ ${MIG_PROFILE} = "all1g5gb" ] 
then
    nvidia-smi mig -cgi 19,19,19,19,19,19,19
    nvidia-smi mig -cci 
fi