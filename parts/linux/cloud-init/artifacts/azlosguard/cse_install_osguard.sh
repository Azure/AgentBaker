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
    local targetBinDir="${3:-"/opt/bin"}"

    echo "installing ${packageName} version ${desiredVersion} by manually unpacking the RPM"
    if [ "${packageName}" != "kubelet" ] && [ "${packageName}" != "kubectl" ] && [ "${packageName}" != "azure-acr-credential-provider" ]; then
        echo "Error: Unsupported package ${packageName}. Only kubelet, kubectl, and azure-acr-credential-provider installs are allowed on OSGuard."
        exit 1
    fi
    echo "installing ${packageName} version ${desiredVersion}"
    downloadDir="/opt/${packageName}/downloads"
    packagePrefix="${packageName}-${desiredVersion}-*"

    rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || rpmFile=""
    if [ -z "${rpmFile}" ] && { [ "${packageName}" = "kubelet" ] || [ "${packageName}" = "kubectl" ]; } && fallbackToKubeBinaryInstall "${packageName}" "${desiredVersion}"; then
        echo "Successfully installed ${packageName} version ${desiredVersion} from binary fallback"
        rm -rf ${downloadDir}
        return 0
    fi
    if [ -z "${rpmFile}" ]; then
        fullPackageVersion=$(tdnf list ${packageName} | grep ${desiredVersion}- | awk '{print $2}' | sort -V | tail -n 1)
        if [ -z "${fullPackageVersion}" ]; then
            echo "Failed to find valid ${packageName} version for ${desiredVersion}"
            return 1
        fi
        echo "Did not find cached rpm file, downloading ${packageName} version ${fullPackageVersion}"
        downloadPkgFromVersion "${packageName}" ${fullPackageVersion} "${downloadDir}"
        rpmFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || rpmFile=""
    fi
    if [ -z "${rpmFile}" ]; then
        echo "Failed to locate ${packageName} rpm"
        return 1
    fi

    local rpmBinaryName="${packageName}"
    local targetBinaryName="${packageName}"
    if [ "${packageName}" = "azure-acr-credential-provider" ]; then
        targetBinaryName="acr-credential-provider"
    fi

    echo "Unpacking usr/bin/${rpmBinaryName} from ${downloadDir}/${packageName}-${desiredVersion}*"
    mkdir -p "${targetBinDir}"
    # This assumes that the binary will either be in /usr/bin or /usr/local/bin, but not both.
    rpm2cpio "${rpmFile}" | cpio -i --to-stdout "./usr/bin/${rpmBinaryName}" "./usr/local/bin/${rpmBinaryName}" | install -m0755 /dev/stdin "${targetBinDir}/${targetBinaryName}"
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

installCredentialProviderFromPMC() {
    k8sVersion="${1:-}"
    os=${AZURELINUX_OS_NAME}
    if [ -z "$OS_VERSION" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${OS_VERSION}"
    fi
   	PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version" "${OS_VARIANT}"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
	echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    installRPMPackageFromFile "azure-acr-credential-provider" "${packageVersion}" "${CREDENTIAL_PROVIDER_BIN_DIR}" || exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
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
    stub
}

installToolFromLocalRepo() {
    stub
    return 1
}

#EOF
