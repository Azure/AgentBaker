#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Mariner"

dnfversionlockWALinuxAgent() {
    echo "No aptmark equivalent for DNF by default. If this is necessary add support for dnf versionlock plugin"
}

aptmarkWALinuxAgent() {
    echo "No aptmark equivalent for DNF by default. If this is necessary add support for dnf versionlock plugin"
}

dnf_makecache() {
    retries=10
    dnf_makecache_output=/tmp/dnf-makecache.out
    for i in $(seq 1 $retries); do
        ! (dnf makecache -y 2>&1 | tee $dnf_makecache_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $dnf_makecache_output && break || \
        cat $dnf_makecache_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo Executed dnf makecache -y $i times
}

dnf_install() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        dnf install -y ${@} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            dnf_makecache
        fi
    done
    echo Executed dnf install -y \"$@\" $i times;
}
dnf_remove() {
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
dnf_update() {
  retries=10
  dnf_update_output=/tmp/dnf-update.out
  for i in $(seq 1 $retries); do
    ! (dnf update --exclude mshv-linuxloader --exclude kernel-mshv -y --refresh 2>&1 | tee $dnf_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
    cat $dnf_update_output && break || \
    cat $dnf_update_output
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed dnf update -y --refresh $i times
}
dnf_download() {
    retries=$1; wait_sleep=$2; timeout=$3; downloadDir=$4; shift && shift && shift && shift
    mkdir -p "${downloadDir}"
    for i in $(seq 1 $retries); do
        dnf download --downloaddir="${downloadDir}" "$@" && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep "$wait_sleep"
            dnf_makecache
        fi
    done
    echo Executed dnf download --downloaddir="\"${downloadDir}\"" "$@" $i times;
}
#EOF
