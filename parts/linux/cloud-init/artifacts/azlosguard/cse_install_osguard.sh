#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

installKubeletKubectlPkgFromPMC() {
    local desiredVersion="${1}"
	installRPMPackageFromFile "kubelet" $desiredVersion || exit $ERR_KUBELET_INSTALL_FAIL
    installRPMPackageFromFile "kubectl" $desiredVersion || exit $ERR_KUBECTL_INSTALL_FAIL
}

installRPMPackageFromFile() {
    local packageName="${1}"
    local desiredVersion="${2}"

    echo "installing ${packageName} version ${desiredVersion} by manually unpacking the RPM"
    if [ "${packageName}" != "kubelet" ] && [ "${packageName}" != "kubectl" ]; then
        echo "Error: Unsupported package ${packageName}. Only kubelet and kubectl installs are allowed on OSGuard."
        exit 1
    fi
    echo "installing ${packageName} version ${desiredVersion}"
    downloadDir="/opt/${packageName}/downloads"
    packagePrefix="${packageName}-${desiredVersion}-*"

    rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || rpmFile=""
    if [ -z "${rpmFile}" ]; then
        if ! fallbackToKubeBinaryInstall "${packageName}" "${desiredVersion}"; then
            echo "Successfully installed ${packageName} version ${desiredVersion} from binary fallback"
            exit 0
        fi
        # query all package versions and get the latest version for matching k8s version
        fullPackageVersion=$(tdnf list ${packageName} | grep ${desiredVersion}- | awk '{print $2}' | sort -V | tail -n 1)
        if [ -z "${fullPackageVersion}" ]; then
            echo "Failed to find valid ${packageName} version for ${desiredVersion}"
            exit 1
        fi
        echo "Did not find cached rpm file, downloading ${packageName} version ${fullPackageVersion}"
        downloadPkgFromVersion "${packageName}" ${fullPackageVersion} "${downloadDir}"
        rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || rpmFile=""
    fi
    if [ -z "${rpmFile}" ]; then
        echo "Failed to locate ${packageName} rpm"
        exit 1
    fi

    echo "Unpacking usr/bin/${packageName} from ${downloadDir}/${packageName}-${desiredVersion}*"
    pushd ${downloadDir} || exit 1
    rpm2cpio "${rpmFile}" | cpio -idmv
    mv "usr/bin/${packageName}" "/usr/local/bin/${packageName}"
    popd || exit 1
	rm -rf ${downloadDir}
}

downloadPkgFromVersion() {
    packageName="${1:-}"
    packageVersion="${2:-}"
    downloadDir="${3:-"/opt/${packageName}/downloads"}"
    mkdir -p ${downloadDir}
    tdnf_download 30 1 600 ${downloadDir} ${packageName}=${packageVersion} || exit $ERR_APT_INSTALL_TIMEOUT
    echo "Succeeded to download ${packageName} version ${packageVersion}"
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

cleanUpGPUDrivers() {
    stub
}

installToolFromLocalRepo() {
    stub
    return 1
}

#EOF
