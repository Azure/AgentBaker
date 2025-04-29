#!/bin/bash

#NOTE: Currently, Nvidia library mig-parted (https://github.com/NVIDIA/mig-parted) cannot work properly because of the outdated GPU driver version
#TODO: Use mig-parted library to do the partition after the above issue is fixed 
MIG_PROFILE=${1}
case ${MIG_PROFILE} in 
    "MIG1g")
        nvidia-smi mig -cgi 19,19,19,19,19,19,19
        ;;
    "MIG2g")
        nvidia-smi mig -cgi 14,14,14
        ;;
    "MIG3g")
        nvidia-smi mig -cgi 9,9
        ;;
    "MIG4g")
        nvidia-smi mig -cgi 5
        ;;
    "MIG7g")
        nvidia-smi mig -cgi 0
        ;;  
    *)
        echo "not a valid GPU instance profile"
        exit 1
        ;;
esac
nvidia-smi mig -cci