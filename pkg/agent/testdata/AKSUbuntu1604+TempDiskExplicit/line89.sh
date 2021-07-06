#!/bin/bash

#enable MIG mode???
#nvidia-smi -mig 1

#TODO: use mig-parted library to do the partition after it is fix 
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
        ;;
esac
nvidia-smi mig -cci