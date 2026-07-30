#!/bin/bash

#NOTE: Currently, Nvidia library mig-parted (https://github.com/NVIDIA/mig-parted) cannot work properly because of the outdated GPU driver version
#TODO: Use mig-parted library to do the partition after the above issue is fixed 

mig_profile_id() {
    case "$1" in
        "MIG1g")
            echo "19"
            ;;
        "MIG2g")
            echo "14"
            ;;
        "MIG3g")
            echo "9"
            ;;
        "MIG4g")
            echo "5"
            ;;
        "MIG7g")
            echo "0"
            ;;
        *)
            echo "not a valid MIG profile: $1" >&2
            return 1
            ;;
    esac
}

# TODO: Support GPU models with fewer than seven total partitions.
uniform_mig_profile_layout() {
    case "$1" in
        "MIG1g")
            echo "19,19,19,19,19,19,19"
            ;;
        "MIG2g")
            echo "14,14,14"
            ;;
        "MIG3g")
            echo "9,9"
            ;;
        "MIG4g")
            echo "5"
            ;;
        "MIG7g")
            echo "0"
            ;;
        *)
            echo "not a valid MIG profile: $1" >&2
            return 1
            ;;
    esac
}

parse_mig_profile_layout() {
    local profiles_csv="$1"
    local layout=""
    local is_last_profile
    local profile
    local profile_id

    while true; do
        case "$profiles_csv" in
            *,*)
                profile="${profiles_csv%%,*}"
                profiles_csv="${profiles_csv#*,}"
                is_last_profile=false
                ;;
            *)
                profile="$profiles_csv"
                is_last_profile=true
                ;;
        esac

        profile_id="$(mig_profile_id "${profile}")" || return 1
        if [ -n "${layout}" ]; then
            layout="${layout},${profile_id}"
        else
            layout="${profile_id}"
        fi

        if $is_last_profile; then
            break
        fi
    done

    echo "${layout}"
}

if [ -n "${NVIDIA_MIG_PROFILE_LAYOUT:-}" ]; then
    MIG_LAYOUT="$(parse_mig_profile_layout "${NVIDIA_MIG_PROFILE_LAYOUT}")" || exit 1
elif [ -n "${GPU_INSTANCE_PROFILE:-}" ]; then
    MIG_LAYOUT="$(uniform_mig_profile_layout "${GPU_INSTANCE_PROFILE}")" || exit 1
else
    echo "neither NVIDIA_MIG_PROFILE_LAYOUT nor GPU_INSTANCE_PROFILE is set" >&2
    exit 1
fi

nvidia-smi mig -cgi "${MIG_LAYOUT}" || exit $?
nvidia-smi mig -cci
