#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Ubuntu"


holdWALinuxAgent() {
    echo $(date),$(hostname), startHoldWALinuxAgent "$1"
    wait_for_apt_locks
    if [ "$1" = "hold" ]; then
        # set dpkg selection to 'hold' — prevents apt from upgrading walinuxagent
        retrycmd_if_failure 120 5 25 bash -c 'set -o pipefail; printf "walinuxagent hold\n" | dpkg --set-selections' || exit $ERR_HOLD_WALINUXAGENT
    elif [ "$1" = "unhold" ]; then
        # set dpkg selection back to 'install' (unhold) — allows apt to upgrade walinuxagent again
        retrycmd_if_failure 120 5 25 bash -c 'set -o pipefail; printf "walinuxagent install\n" | dpkg --set-selections' || exit $ERR_RELEASE_HOLD_WALINUXAGENT
    else
        echo "$(date),$(hostname), errorHoldWALinuxAgent invalid argument '$1'" >&2
        exit 1
    fi
    echo $(date),$(hostname), endHoldWALinuxAgent "$1"
}

wait_for_apt_locks() {
    while fuser /var/lib/dpkg/lock /var/lib/apt/lists/lock /var/cache/apt/archives/lock /var/lib/dpkg/lock-frontend >/dev/null 2>&1; do
        echo 'Waiting for release of apt locks'
        sleep 3
    done
}
# Core update function used by apt_get_update and apt_get_install_from_local_repo
_apt_get_update() {
    local retries=$1
    local apt_opts=$2
    local apt_update_output=/tmp/apt-get-update.out

    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        apt-get ${apt_opts} -f -y install
        ! (apt-get ${apt_opts} update 2>&1 | tee $apt_update_output | grep -E "^([WE]:.*)|^([Ee][Rr][Rr][Oo][Rr].*)$") && \
        cat $apt_update_output && break || \
        cat $apt_update_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo Executed apt-get update $i times
    wait_for_apt_locks
}
apt_get_update() {
    _apt_get_update 10 ""
}
apt_get_update_with_opts() {
    local apt_opts=$1
    _apt_get_update 10 "${apt_opts}"
}
_apt_get_install() {
    local retries=$1
    local wait_sleep=$2
    local apt_opts=$3
    local maxBudget=${4:-0}
    shift && shift && shift && shift
    local packages=("${@}")

    local hasBudget=false
    if [ "${maxBudget}" -gt 0 ]; then
        hasBudget=true
    fi

    local opStartTime
    opStartTime=$(date +%s)

    for i in $(seq 1 $retries); do
        # CSE timeout guard
        if ! check_cse_timeout; then
            echo "CSE timeout approaching, exiting apt_get_install early." >&2
            return 2
        fi

        # Per-operation budget check
        if [ "$hasBudget" = true ]; then
            local opElapsed
            opElapsed=$(( $(date +%s) - opStartTime ))
            if [ "$opElapsed" -ge "$maxBudget" ]; then
                echo "apt_get_install budget of ${maxBudget}s exceeded after ${opElapsed}s, exiting early." >&2
                return 2
            fi
        fi

        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef

        # Cap per-attempt timeout to the remaining budget so a single attempt
        # cannot overrun the operation window.
        local install_ok=false
        if [ "$hasBudget" = true ]; then
            local remaining=$(( maxBudget - ( $(date +%s) - opStartTime ) ))
            if [ "$remaining" -lt 1 ]; then
                echo "apt_get_install budget of ${maxBudget}s exceeded, exiting early." >&2
                return 2
            fi
            timeout "$remaining" apt-get install ${apt_opts} -o Dpkg::Options::="--force-confold" --no-install-recommends "${packages[@]}" && install_ok=true
        else
            apt-get install ${apt_opts} -o Dpkg::Options::="--force-confold" --no-install-recommends "${packages[@]}" && install_ok=true
        fi

        if [ "$install_ok" = true ]; then
            echo "Executed apt-get install \"${packages[*]}\" $i times"
            wait_for_apt_locks
            DEBIAN_FRONTEND=noninteractive apt-get clean
            wait_for_apt_locks
            DEBIAN_FRONTEND=noninteractive apt-get clean
            wait_for_apt_locks
            return 0
        fi

        if [ $i -eq $retries ]; then
            return 1
        else
            # Check budget/CSE again before sleeping
            if ! check_cse_timeout; then
                echo "CSE timeout approaching, exiting apt_get_install early." >&2
                return 2
            fi
            if [ "$hasBudget" = true ]; then
                local postElapsed=$(( $(date +%s) - opStartTime ))
                if [ "$postElapsed" -ge "$maxBudget" ]; then
                    echo "apt_get_install budget of ${maxBudget}s exceeded after ${postElapsed}s, exiting early." >&2
                    return 2
                fi
            fi
            sleep $wait_sleep
            apt_get_update
        fi
    done
}
apt_get_install() {
    local retries=$1; local wait_sleep=$2; local timeout=$3; shift && shift && shift
    # Only apply per-operation budget during real CSE runs; during VHD build use no cap.
    local maxBudget=0
    if [ -n "${CSE_STARTTIME_SECONDS:-}" ]; then
        maxBudget=$timeout
    fi
    _apt_get_install "$retries" "$wait_sleep" "-y" "$maxBudget" "$@"
}
apt_get_purge() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        timeout $timeout apt-get purge -o Dpkg::Options::="--force-confold" -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed apt-get purge -y \"$@\" $i times;
    wait_for_apt_locks
}
apt_get_dist_upgrade() {
  retries=10
  apt_dist_upgrade_output=/tmp/apt-get-dist-upgrade.out
  for i in $(seq 1 $retries); do
    wait_for_apt_locks
    export DEBIAN_FRONTEND=noninteractive
    dpkg --configure -a --force-confdef
    apt-get -f -y install
    dpkg --get-selections | awk '$2=="hold"{print $1}'
    ! (apt-get -o Dpkg::Options::="--force-confnew" dist-upgrade -y 2>&1 | tee $apt_dist_upgrade_output | grep -E "^([WE]:.*)|^([Ee][Rr][Rr][Oo][Rr].*)$") && \
    cat $apt_dist_upgrade_output && break || \
    cat $apt_dist_upgrade_output
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed apt-get dist-upgrade $i times
  wait_for_apt_locks
}
installDebPackageFromFile() {
    export DEBIAN_FRONTEND=noninteractive
    local DEB_FILE=$1
    wait_for_apt_locks
    retrycmd_if_failure 3 5 120 dpkg --force-confdef --force-confold -i "${DEB_FILE}" || {
        retrycmd_if_failure 10 5 600 apt-get -y -f install "${DEB_FILE}" --allow-downgrades
    }
    if [ "$?" -ne 0 ]; then
        return 1
    fi
}

apt_get_install_from_local_repo() {
    local local_repo_dir=$1
    local package_name=$2

    # Convert to absolute path
    local_repo_dir=$(realpath "${local_repo_dir}")

    if [ ! -d "${local_repo_dir}" ]; then
        echo "Local repo directory ${local_repo_dir} does not exist"
        return 1
    fi

    # Check if Packages.gz exists in the repo
    if [ ! -f "${local_repo_dir}/Packages.gz" ]; then
        echo "Packages.gz not found in ${local_repo_dir}"
        return 1
    fi

    wait_for_apt_locks

    local tmp_list=$(mktemp)
    local tmp_dir=$(mktemp -d)

    # Create temporary sources.list pointing to local repo
    printf 'deb [trusted=yes] file:%s ./\n' "${local_repo_dir}" > "${tmp_list}"

    local opts="-o Dir::Etc::sourcelist=${tmp_list} -o Dir::Etc::sourceparts=${tmp_dir}"

    # Update apt cache with local repo using core update function
    if ! _apt_get_update 10 "${opts}"; then
        echo "Failed to update apt cache from local repo ${local_repo_dir}"
        rm -f "${tmp_list}"
        rmdir "${tmp_dir}"
        return 1
    fi

    # Install package from local repo using core installation function
    local retries=10
    local wait_sleep=5
    # maxBudget=0: no per-operation time cap for local repo installs (no network download involved)
    if ! _apt_get_install $retries $wait_sleep "${opts}" 0 "${package_name}"; then
        echo "Failed to install ${package_name} from local repo"
        rm -f "${tmp_list}"
        rmdir "${tmp_dir}"
        return 1
    fi

    # Cleanup
    rm -f "${tmp_list}"
    rmdir "${tmp_dir}"

    return 0
}

apt_get_download() {
  retries=$1; wait_sleep=$2; shift && shift;
  local ret=0
  pushd $APT_CACHE_DIR || return 1
  for i in $(seq 1 "$retries"); do
    dpkg --configure -a --force-confdef
    wait_for_apt_locks

    # Pull the first quoted URL from --print-uris
    url="$(apt-get --print-uris -o Dpkg::Options::=--force-confold download -y -- "$@" \
           | awk -F"'" 'NR==1 && $2 {print $2}')"
    if [ -n "$url" ]; then
      # This avoids issues with the naming in the package. `apt-get download`
      # encodes the package names with special characters and does not decode
      # them when saving to disk, but `curl -J` handles the names correctly.
      if curl -fLJO -- "$url"; then ret=0; break; fi
    fi

    if [ "$i" -eq "$retries" ]; then ret=1; else sleep "$wait_sleep"; fi
  done
  popd || return 1
  return "$ret"
}
#EOF
