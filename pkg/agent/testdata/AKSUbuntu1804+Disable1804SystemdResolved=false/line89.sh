#!/bin/bash

#enable MIG mode???
#nvidia-smi -mig 1

#TODO: use mig-parted library to do the partition after Nvidia fix it 
MIG_PROFILE=${1}
if [ ${MIG_PROFILE} = "mig-1g" ] 
then
    nvidia-smi mig -cgi 19,19,19,19,19,19,19
    nvidia-smi mig -cci 

elif [ ${MIG_PROFILE} = "mig-2g" ] 
then 
    nvidia-smi mig -cgi 14,14,14
    nvidia-smi mig -cci

elif [ ${MIG_PROFILE} = "mig-3g" ] 
then 
    nvidia-smi mig -cgi 9,9
    nvidia-smi mig -cci

elif [ ${MIG_PROFILE} = "mig-4g" ] 
then 
    nvidia-smi mig -cgi 5
    nvidia-smi mig -cci

elif [ ${MIG_PROFILE} = "mig-7g" ] 
then 
    nvidia-smi mig -cgi 0
    nvidia-smi mig -cci

else
    echo "not valid GPU instance profile"
fi