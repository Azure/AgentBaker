#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

installDeps() {
    stub
}

installCriCtlPackage() {
    stub
}

installStandaloneContainerd() {
    stub
}

ensureRunc() {
    stub
}

removeNvidiaRepos() {
    stub
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

installToolFromLocalRepo() {
    stub
    return 1
}

#EOF
