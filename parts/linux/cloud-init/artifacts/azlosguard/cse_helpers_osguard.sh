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

tdnf_download() {
    retries=$1; wait_sleep=$2; timeout=$3; downloadDir=$4; shift && shift && shift && shift
    mkdir -p ${downloadDir}
    for i in $(seq 1 $retries); do
        tdnf install -y ${@} --downloadonly --downloaddir=${downloadDir} && break || \
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed tdnf install -y \"$@\" $i times;
}

tdnf_update() {
  retries=10
  tdnf_update_output=/tmp/tdnf-update.out
  for i in $(seq 1 $retries); do
    ! (tdnf update -y --refresh 2>&1 | tee $tdnf_update_output | grep -E "^([WE]:.*)|([eE]rr.*)$") && \
    cat $tdnf_update_output && break || \
    cat $tdnf_update_output
    if [ $i -eq $retries ]; then
      return 1
    else sleep 5
    fi
  done
  echo Executed tdnf update -y --refresh $i times
}

#EOF
