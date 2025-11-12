#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Ubuntu"


aptmarkWALinuxAgent() {
    echo $(date),$(hostname), startAptmarkWALinuxAgent "$1"
    wait_for_apt_locks
    retrycmd_if_failure 120 5 25 apt-mark $1 walinuxagent || \
    if [ "$1" = "hold" ]; then
        exit $ERR_HOLD_WALINUXAGENT
    elif [ "$1" = "unhold" ]; then
        exit $ERR_RELEASE_HOLD_WALINUXAGENT
    fi
    echo $(date),$(hostname), endAptmarkWALinuxAgent "$1"
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
    shift && shift && shift

    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef

        if apt-get install ${apt_opts} -o Dpkg::Options::="--force-confold" --no-install-recommends "${@}"; then
            echo "Executed apt-get install \"${packages[@]}\" $i times"
            wait_for_apt_locks
            return 0
        fi

        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            apt_get_update
        fi
    done
}
apt_get_install() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    _apt_get_install $retries $wait_sleep "-y" "$@"
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
    apt-mark showhold
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
    DEB_FILE=$1
    wait_for_apt_locks
    retrycmd_if_failure 10 5 600 apt-get -y -f install ${DEB_FILE} --allow-downgrades
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
    if ! _apt_get_install $retries $wait_sleep "${opts}" "${package_name}"; then
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
#EOF
