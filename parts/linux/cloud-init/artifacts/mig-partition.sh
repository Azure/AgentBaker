#!/bin/bash

# NOTE: Currently, Nvidia library mig-parted (https://github.com/NVIDIA/mig-parted) cannot work properly because of the outdated GPU driver version.
# TODO: Use mig-parted library to do the partition after the above issue is fixed.
MIG_PROFILE=${1}
TOTAL_GPU_INSTANCE_SLICES=${2:-7}

case ${TOTAL_GPU_INSTANCE_SLICES} in
    ''|*[!0-9]*)
        echo "total GPU instance slices must be a positive integer: ${TOTAL_GPU_INSTANCE_SLICES}" >&2
        exit 1
        ;;
esac
if [ "${TOTAL_GPU_INSTANCE_SLICES}" -le 0 ]; then
    echo "total GPU instance slices must be a positive integer: ${TOTAL_GPU_INSTANCE_SLICES}" >&2
    exit 1
fi

case ${MIG_PROFILE} in
    "MIG1g")
        PROFILE_WIDTH=1
        PROFILE_ID=19
        ;;
    "MIG2g")
        PROFILE_WIDTH=2
        PROFILE_ID=14
        ;;
    "MIG3g")
        PROFILE_WIDTH=3
        PROFILE_ID=9
        ;;
    "MIG4g")
        PROFILE_WIDTH=4
        PROFILE_ID=5
        ;;
    "MIG7g")
        PROFILE_WIDTH=7
        PROFILE_ID=0
        ;;
    *)
        echo "not a valid GPU instance profile: ${MIG_PROFILE}" >&2
        exit 1
        ;;
esac

PROFILE_COUNT=$((TOTAL_GPU_INSTANCE_SLICES / PROFILE_WIDTH))
if [ "${PROFILE_COUNT}" -eq 0 ]; then
    echo "GPU instance profile ${MIG_PROFILE} requires more than ${TOTAL_GPU_INSTANCE_SLICES} slices" >&2
    exit 1
fi

MIG_LAYOUT=${PROFILE_ID}
PROFILE_INDEX=1
while [ "${PROFILE_INDEX}" -lt "${PROFILE_COUNT}" ]; do
    MIG_LAYOUT="${MIG_LAYOUT},${PROFILE_ID}"
    PROFILE_INDEX=$((PROFILE_INDEX + 1))
done

nvidia-smi mig -cgi "${MIG_LAYOUT}"
nvidia-smi mig -cci
