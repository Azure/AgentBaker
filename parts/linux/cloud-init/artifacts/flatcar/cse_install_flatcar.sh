#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

downloadSysextFromVersion() {
    local sysextName=$1
    local sysextURL=$2
    local downloadDir=${3:-"/opt/${sysextName}/downloads"}

    local tries=0
    while ! timeout 60 oras pull --output "${downloadDir}" "${sysextURL}"; do
        if [[ $(( ++tries )) -eq 120 ]]; then
            echo "Failed to download ${sysextName} system extension from ${sysextURL}"
            return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
        fi
        sleep 5
    done

    echo "Succeeded to download ${sysextName} system extension from ${sysextURL}"
}

matchLocalSysext() {
    local sysextName=$1 desiredVersion=$2 sysextArch=$3
    printf "%s\n" "/opt/${sysextName}/downloads/${sysextName}-v${desiredVersion}"[.~-]*"-${sysextArch}.raw" | sort -V | tail -n1
}

matchRemoteSysext() {
    local sysextURL=$1 desiredVersion=$2 sysextArch=$3 tries=0
    while ! { timeout 20 oras repo tags "${sysextURL}" | grep -Ex "v${desiredVersion//./\\.}[.~-].*-azlinux3-${sysextArch}" | sort -V | tail -n1; test ${PIPESTATUS[0]} -eq 0; }; do
        [[ $(( ++tries )) -eq 120 ]] && return 1
        sleep 5
    done
}

mergeSysexts() {
    local sysextArch
    sysextArch=$(getSystemdArch)

    while [[ ${1-} ]]; do
        local sysextName=$1 sysextURL=$2 desiredVersion=$3 sysextMatch

        sysextMatch=$(matchLocalSysext "${sysextName}" "${desiredVersion}" "${sysextArch}")
        if ! test -f "${sysextMatch}"; then
            echo "Failed to find valid ${sysextName} system extension for ${desiredVersion} locally"

            sysextMatch=$(matchRemoteSysext "${sysextURL}" "${desiredVersion}" "${sysextArch}")
            if [[ -z ${sysextMatch} ]]; then
                echo "Failed to find valid ${sysextName} system extension for ${desiredVersion} remotely"
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi

            if ! downloadSysextFromVersion "${sysextName}" "${sysextURL}:${sysextMatch}"; then
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi

            sysextMatch=$(matchLocalSysext "${sysextName}" "${desiredVersion}" "${sysextArch}")
            if ! test -f "${sysextMatch}"; then
                echo "Failed to find valid ${sysextName} system extension for ${desiredVersion} after downloading"
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi
        fi

        ln -snf "${sysextMatch}" "/etc/extensions/${sysextName}.raw"
        shift 3
    done

    systemd-sysext --no-reload refresh
}

installDeps() {
    stub
}

installCriCtlPackage() {
    stub
}

installKubeletKubectlFromPkg() {
    if mergeSysexts kubelet mcr.microsoft.com/oss/v2/kubernetes/kubelet-sysext "$1" \
                    kubectl mcr.microsoft.com/oss/v2/kubernetes/kubectl-sysext "$1"; then
        ln -snf /usr/bin/{kubelet,kubectl} /opt/bin/
    else
        installKubeletKubectlFromURL
    fi
}

installStandaloneContainerd() {
    stub
}

ensureRunc() {
    stub
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu
}

#EOF
