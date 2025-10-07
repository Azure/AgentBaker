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
apt_get_update() {
    retries=10
    apt_update_output=/tmp/apt-get-update.out
    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        apt-get -f -y install
        ! (apt-get update 2>&1 | tee $apt_update_output | grep -E "^([WE]:.*)|^([Ee][Rr][Rr][Oo][Rr].*)$") && \
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
apt_get_install() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        wait_for_apt_locks
        export DEBIAN_FRONTEND=noninteractive
        dpkg --configure -a --force-confdef
        apt-get install -o Dpkg::Options::="--force-confold" --no-install-recommends -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            apt_get_update
        fi
    done
    echo Executed apt-get install --no-install-recommends -y \"$@\" $i times;
    wait_for_apt_locks
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

    # Update apt cache with local repo
    if ! apt-get ${opts} update 2>&1; then
        echo "Failed to update apt cache from local repo ${local_repo_dir}"
        rm -f "${tmp_list}"
        rmdir "${tmp_dir}"
        return 1
    fi

    # Install package from local repo (no download needed)
    export DEBIAN_FRONTEND=noninteractive
    if ! apt-get ${opts} --no-download install -y "${package_name}"; then
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
