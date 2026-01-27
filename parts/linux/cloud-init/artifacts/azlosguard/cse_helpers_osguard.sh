#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for Azure Linux OS Guard"

stub() {
    echo "${FUNCNAME[1]} stub"
}

dnfversionlockWALinuxAgent() {
    stub
}

aptmarkWALinuxAgent() {
    stub
}

dnf_makecache() {
    stub
}
dnf_install() {
    stub
}
dnf_remove() {
    stub
}
dnf_update() {
    stub
}

tdnf_makecache() {
    retries=10
    tdnf_makecache_output=/tmp/tdnf-makecache.out
    for i in $(seq 1 $retries); do
        ! (tdnf makecache -y 2>&1 | tee $tdnf_makecache_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
        cat $tdnf_makecache_output && break || \
        cat $tdnf_makecache_output
        if [ $i -eq $retries ]; then
            return 1
        else sleep 5
        fi
    done
    echo Executed tdnf makecache -y $i times
}

tdnf_download() {
    retries=$1; wait_sleep=$2; timeout=$3; downloadDir=$4; shift && shift && shift && shift
    mkdir -p ${downloadDir}
    for i in $(seq 1 $retries); do
        tdnf install -y ${@} --downloadonly --downloaddir=${downloadDir} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            tdnf_makecache
        fi
    done
    echo Executed tdnf install -y \"$@\" $i times;
}

#EOF
