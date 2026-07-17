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

trim_mig_profile() {
    local profile="$1"
    profile="${profile#"${profile%%[![:space:]]*}"}"
    profile="${profile%"${profile##*[![:space:]]}"}"
    echo "$profile"
}

mixed_mig_profiles_layout() {
    local mig_profiles_csv="$1"
    local layout=""
    local profile
    local profile_id

    while [ -n "$mig_profiles_csv" ]; do
        profile="${mig_profiles_csv%%,*}"
        if [ "$profile" = "$mig_profiles_csv" ]; then
            mig_profiles_csv=""
        else
            mig_profiles_csv="${mig_profiles_csv#*,}"
        fi

        profile="$(trim_mig_profile "$profile")"
        if [ -z "${profile}" ]; then
            continue
        fi
        profile_id="$(mig_profile_id "${profile}")" || return 1
        if [ -n "${layout}" ]; then
            layout="${layout},${profile_id}"
        else
            layout="${profile_id}"
        fi
    done

    if [ -z "${layout}" ]; then
        echo "MIG profiles cannot be empty" >&2
        return 1
    fi
    echo "${layout}"
}

if [ -n "${GPU_INSTANCE_PROFILE:-}" ] && [ -n "${NVIDIA_MIG_PROFILES:-}" ]; then
    echo "GPU_INSTANCE_PROFILE and NVIDIA_MIG_PROFILES are mutually exclusive" >&2
    exit 1
fi

if [ -z "${GPU_INSTANCE_PROFILE:-}" ] && [ -z "${NVIDIA_MIG_PROFILES:-}" ]; then
    echo "exactly one of GPU_INSTANCE_PROFILE or NVIDIA_MIG_PROFILES must be set" >&2
    exit 1
fi

if [ -n "${NVIDIA_MIG_PROFILES:-}" ]; then
    SELECTED_MIG_PROFILES="${NVIDIA_MIG_PROFILES}"
else
    SELECTED_MIG_PROFILES="${GPU_INSTANCE_PROFILE}"
fi

if [ "${NVIDIA_MIG_STRATEGY:-}" = "Mixed" ]; then
    MIG_LAYOUT="$(mixed_mig_profiles_layout "${SELECTED_MIG_PROFILES}")" || exit 1
else
    case "${SELECTED_MIG_PROFILES}" in
        *,*)
            echo "Single MIG strategy requires exactly one MIG profile" >&2
            exit 1
            ;;
    esac
    MIG_PROFILE="$(trim_mig_profile "${SELECTED_MIG_PROFILES}")"
    MIG_LAYOUT="$(uniform_mig_profile_layout "${MIG_PROFILE}")" || exit 1
fi

nvidia-smi mig -cgi "${MIG_LAYOUT}"
nvidia-smi mig -cci
