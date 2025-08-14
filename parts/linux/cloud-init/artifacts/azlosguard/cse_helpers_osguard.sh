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

dnf_download() {
    retries=$1; wait_sleep=$2; timeout=$3; downloadDir=$4; shift && shift && shift && shift
    mkdir -p ${downloadDir}
    for i in $(seq 1 $retries); do
        dnf install -y ${@} --downloadonly --downloaddir=${downloadDir} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
            dnf_makecache
        fi
    done
    echo Executed dnf install -y \"$@\" $i times;
}


#EOF
