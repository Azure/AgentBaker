#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Mariner"

aptmarkWALinuxAgent() {
    echo "No aptmark equivalent for DNF"
}

wait_for_apt_locks() {
    echo 'Not waiting. DNF handles locking.'
}
apt_get_update() {
    retries=10
    apt_update_output=/tmp/apt-get-update.out
    for i in $(seq 1 $retries); do
        ! (dnf makecache -y 2>&1 | tee $apt_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $apt_update_output && break || \
        cat $apt_update_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo Executed dnf makecache -y $i times
}
apt_get_install() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        dnf install -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            apt_get_update
        fi
    done
    echo Executed dnf install -y \"$@\" $i times;
}
apt_get_purge() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        dnf remove -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed dnf remove  -y \"$@\" $i times;
}
apt_get_dist_upgrade() {
  retries=10
  apt_dist_upgrade_output=/tmp/apt-get-dist-upgrade.out
  for i in $(seq 1 $retries); do
    ! (dnf update -y --refresh 2>&1 | tee $apt_dist_upgrade_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
    cat $apt_dist_upgrade_output && break || \
    cat $apt_dist_upgrade_output
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed dnf update -y --refresh $i times
}
#HELPERSEOF
