#!/bin/bash

#enable MIG mode???
#nvidia-smi -mig 1

MIG_PROFILE=${1}
echo "mig profile is ${MIG_PROFILE}"
if [ ${MIG_PROFILE} = "MIG1g" ] 
then
    nvidia-smi mig -cgi 19,19,19,19,19,19,19
    nvidia-smi mig -cci 

elif [ ${MIG_PROFILE} = "MIG2g" ] 
then
    nvidia-smi mig -cgi 14,14,14
    nvidia-smi mig -cci

elif [ ${MIG_PROFILE} = "MIG3g" ] 
then
    nvidia-smi mig -cgi 9,9
    nvidia-smi mig -cci

elif [ ${MIG_PROFILE} = "MIG4g" ] 
then
    nvidia-smi mig -cgi 5
    nvidia-smi mig -cci

elif [ ${MIG_PROFILE} = "MIG7g" ] 
then
    nvidia-smi mig -cgi 0
    nvidia-smi mig -cci

#else: error msg 
fi