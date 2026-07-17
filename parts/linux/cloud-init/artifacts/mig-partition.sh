#!/bin/bash

#NOTE: Currently, Nvidia library mig-parted (https://github.com/NVIDIA/mig-parted) cannot work properly because of the outdated GPU driver version
#TODO: Use mig-parted library to do the partition after the above issue is fixed

gpu_instance_profile_id() {
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
            echo "not a valid GPU instance profile: $1" >&2
            return 1
            ;;
    esac
}

uniform_gpu_instance_profile_layout() {
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
            echo "not a valid GPU instance profile: $1" >&2
            return 1
            ;;
    esac
}

trim_profile() {
    local profile="$1"
    profile="${profile#"${profile%%[![:space:]]*}"}"
    profile="${profile%"${profile##*[![:space:]]}"}"
    echo "$profile"
}

mixed_gpu_instance_profiles_layout() {
    local profiles_csv="$1"
    local layout=""
    local profile
    local profile_id

    while [ -n "$profiles_csv" ]; do
        profile="${profiles_csv%%,*}"
        if [ "$profile" = "$profiles_csv" ]; then
            profiles_csv=""
        else
            profiles_csv="${profiles_csv#*,}"
        fi

        profile="$(trim_profile "$profile")"
        if [ -z "${profile}" ]; then
            continue
        fi
        profile_id="$(gpu_instance_profile_id "${profile}")" || return 1
        if [ -n "${layout}" ]; then
            layout="${layout},${profile_id}"
        else
            layout="${profile_id}"
        fi
    done

    if [ -z "${layout}" ]; then
        echo "GPU instance profiles cannot be empty" >&2
        return 1
    fi
    echo "${layout}"
}

MIG_PROFILES="${MIG_PROFILES:-${GPU_INSTANCE_PROFILE:-}}"

for profile_arg in "$@"; do
    case "${profile_arg}" in
        *,*)
            if [ -z "${MIG_PROFILES}" ]; then
                MIG_PROFILES="${profile_arg}"
                continue
            fi
            ;;
    esac
    if [ -z "${MIG_PROFILES}" ] && [ -n "${profile_arg}" ]; then
        MIG_PROFILES="${profile_arg}"
    fi
done

if [ "${NVIDIA_MIG_STRATEGY:-}" = "Mixed" ]; then
    MIG_LAYOUT="$(mixed_gpu_instance_profiles_layout "${MIG_PROFILES}")" || exit 1
else
    MIG_PROFILE="$(trim_profile "${MIG_PROFILES%%,*}")"
    MIG_LAYOUT="$(uniform_gpu_instance_profile_layout "${MIG_PROFILE}")" || exit 1
fi

nvidia-smi mig -cgi "${MIG_LAYOUT}"
nvidia-smi mig -cci
