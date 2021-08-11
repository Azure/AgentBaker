#!/bin/bash

#TODO: use mig-parted library to do the partition after the issue is fixed 
MIG_PROFILE=${1}
case ${MIG_PROFILE} in 
    "mig-1g")
        nvidia-smi mig -cgi 19,19,19,19,19,19,19
        ;;
    "mig-2g")
        nvidia-smi mig -cgi 14,14,14
        ;;
    "mig-3g")
        nvidia-smi mig -cgi 9,9
        ;;
    "mig-4g")
        nvidia-smi mig -cgi 5
        ;;
    "mig-7g")
        nvidia-smi mig -cgi 0
        ;;  
    *)
        echo "not a valid GPU instance profile"
        exit ${ERR_MIG_PARTITION_FAILURE}
        ;;
esac
nvidia-smi mig -cci