#!/bin/bash

removeContainerd() {
    apt_get_purge 10 5 300 moby-containerd
}

installDeps() {
    wait_for_apt_locks
    retrycmd_silent 120 5 25 curl -fsSL https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/packages-microsoft-prod.deb > /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 60 5 10 dpkg -i /tmp/packages-microsoft-prod.deb || exit $ERR_MS_PROD_DEB_PKG_ADD_FAIL

    aptmarkWALinuxAgent hold
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT

    pkg_list=(apparmor-utils bind9-dnsutils ca-certificates ceph-common cgroup-lite cifs-utils conntrack cracklib-runtime ebtables ethtool glusterfs-client htop init-system-helpers inotify-tools iotop iproute2 ipset iptables nftables jq libpam-pwquality libpwquality-tools mount nfs-common pigz socat sysfsutils sysstat util-linux xz-utils netcat-openbsd zip rng-tools kmod gcc make dkms initramfs-tools linux-headers-$(uname -r) linux-modules-extra-$(uname -r))

    local OSVERSION
    OSVERSION=$(grep DISTRIB_RELEASE /etc/*-release| cut -f 2 -d "=")
    BLOBFUSE_VERSION="1.4.5"
    # Blobfuse2 has been upgraded in upstream, using this version for parity between 22.04 and 24.04
    BLOBFUSE2_VERSION="2.5.1"

    # blobfuse2 is installed for all ubuntu versions, it is included in pkg_list
    # for 22.04, fuse3 is installed. for all others, fuse is installed
    # for all others except 22.04, installed blobfuse1.4.5
    pkg_list+=("blobfuse2=${BLOBFUSE2_VERSION}")
    if [ "${OSVERSION}" = "22.04" ] || [ "${OSVERSION}" = "24.04" ]; then
        pkg_list+=(fuse3)
    else
        pkg_list+=("blobfuse=${BLOBFUSE_VERSION}" fuse)
    fi

    if [ "${OSVERSION}" = "24.04" ]; then
        pkg_list+=(irqbalance)
    fi

    if [ "${OSVERSION}" = "22.04" ] || [ "${OSVERSION}" = "24.04" ]; then
        if [ "$(isARM64)" -eq 0 ]; then
            pkg_list+=("aznfs=0.3.15")
        fi
    fi

    for apt_package in ${pkg_list[*]}; do
        if ! apt_get_install 30 1 600 $apt_package; then
            journalctl --no-pager -u $apt_package
            exit $ERR_APT_INSTALL_TIMEOUT
        fi
    done

    if [ "${OSVERSION}" = "22.04" ] || [ "${OSVERSION}" = "24.04" ]; then
        if [ "$(isARM64)" -eq 0 ]; then
            # disable aznfswatchdog since aznfs install and enable aznfswatchdog and aznfswatchdogv4 services at the same time while we only need aznfswatchdogv4
            systemctl disable aznfswatchdog
            systemctl stop aznfswatchdog
        fi
    fi
}

updateAptWithMicrosoftPkg() {
    retrycmd_silent 120 5 25 curl https://packages.microsoft.com/config/ubuntu/${UBUNTU_RELEASE}/prod.list > /tmp/microsoft-prod.list || exit $ERR_MOBY_APT_LIST_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft-prod.list /etc/apt/sources.list.d/ || exit $ERR_MOBY_APT_LIST_TIMEOUT
    if [ "${UBUNTU_RELEASE}" = "18.04" ]; then {
        echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/18.04/multiarch/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list
    }
    elif [ "${UBUNTU_RELEASE}" = "20.04" ] || [ "${UBUNTU_RELEASE}" = "22.04" ] || [ "${UBUNTU_RELEASE}" = "24.04" ]; then {
        echo "deb [arch=amd64,arm64,armhf] https://packages.microsoft.com/ubuntu/${UBUNTU_RELEASE}/prod testing main" > /etc/apt/sources.list.d/microsoft-prod-testing.list
    }
    fi

    retrycmd_silent 120 5 25 curl https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /tmp/microsoft.gpg || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    retrycmd_if_failure 10 5 10 cp /tmp/microsoft.gpg /etc/apt/trusted.gpg.d/ || exit $ERR_MS_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

updatePMCRepository() {
    local opts="-o Dir::Etc::sourcelist=/etc/apt/sources.list.d/microsoft-prod.list -o Dir::Etc::sourceparts=-"
    apt_get_update_with_opts "${opts}" || exit $ERR_APT_UPDATE_TIMEOUT
}

updateAptWithNvidiaPkg() {
    readonly nvidia_gpg_keyring_path="/etc/apt/keyrings/nvidia.pub"
    mkdir -p "$(dirname "${nvidia_gpg_keyring_path}")"

    readonly nvidia_sources_list_path="/etc/apt/sources.list.d/nvidia.list"
    local cpu_arch=$(getCPUArch)  # Returns amd64 or arm64
    local repo_arch=""
    local nvidia_ubuntu_release=""

    if [ "$cpu_arch" = "amd64" ]; then
        repo_arch="x86_64"
    elif [ "$cpu_arch" = "arm64" ]; then
        repo_arch="sbsa"
    else
        echo "Unknown CPU architecture: ${cpu_arch}"
        return
    fi

    if [ "${UBUNTU_RELEASE}" = "22.04" ]; then
        nvidia_ubuntu_release="ubuntu2204"
    elif [ "${UBUNTU_RELEASE}" = "24.04" ]; then
        nvidia_ubuntu_release="ubuntu2404"
    else
        echo "NVIDIA repo setup is not supported on Ubuntu ${UBUNTU_RELEASE}"
        return
    fi

    # Construct URLs based on detected architecture and Ubuntu version
    echo "deb [arch=${cpu_arch} signed-by=${nvidia_gpg_keyring_path}] https://developer.download.nvidia.com/compute/cuda/repos/${nvidia_ubuntu_release}/${repo_arch} /" > ${nvidia_sources_list_path}

    # Add NVIDIA repository
    local nvidia_gpg_key_url="https://developer.download.nvidia.com/compute/cuda/repos/${nvidia_ubuntu_release}/${repo_arch}/3bf863cc.pub"

    # Download and add the GPG key for the NVIDIA repository
    retrycmd_curl_file 120 5 25 ${nvidia_gpg_keyring_path} ${nvidia_gpg_key_url} || exit $ERR_NVIDIA_GPG_KEY_DOWNLOAD_TIMEOUT
    apt_get_update || exit $ERR_APT_UPDATE_TIMEOUT
}

isPackageInstalled() {
    local packageName="${1}"
    if dpkg -l "${packageName}" 2>/dev/null | grep -q "^ii"; then
        return 0  # Package is installed
    else
        return 1  # Package is not installed
    fi
}

managedGPUPackageList() {
    packages=(
        nvidia-device-plugin
        datacenter-gpu-manager-4-core
        datacenter-gpu-manager-4-proprietary
        datacenter-gpu-manager-exporter
    )
    echo "${packages[@]}"
}

installNvidiaManagedExpPkgFromCache() {
    # Ensure kubelet device-plugins directory exists BEFORE package installation
    mkdir -p /var/lib/kubelet/device-plugins

    for packageName in $(managedGPUPackageList); do
        downloadDir="/opt/${packageName}/downloads"
        if isPackageInstalled "${packageName}"; then
            echo "${packageName} is already installed, skipping."
            rm -rf $(dirname ${downloadDir})
            continue
        fi

        debFile=$(find "${downloadDir}" -maxdepth 1 -name "${packageName}*" -print -quit 2>/dev/null) || debFile=""
        if [ -z "${debFile}" ]; then
            echo "Failed to locate ${packageName} deb"
            exit $ERR_MANAGED_NVIDIA_EXP_INSTALL_FAIL
        fi
        logs_to_events "AKS.CSE.install${packageName}.installDebPackageFromFile" "installDebPackageFromFile ${debFile}" || exit $ERR_APT_INSTALL_TIMEOUT
        rm -rf $(dirname ${downloadDir})
    done
}

cleanUpGPUDrivers() {
    rm -Rf $GPU_DEST /opt/gpu

    for packageName in $(managedGPUPackageList); do
        rm -rf "/opt/${packageName}"
    done
}

installCriCtlPackage() {
    version="${1:-}"
    packageName="kubernetes-cri-tools=${version}"
    if [ -z "$version" ]; then
        echo "Error: No version specified for kubernetes-cri-tools package but it is required. Exiting with error."
        exit 1
    fi
    echo "Installing ${packageName} with apt-get"
    apt_get_install 20 30 120 ${packageName} || exit 1
}

installCredentialProviderFromPMC() {
    k8sVersion="${1:-}"
    os=${UBUNTU_OS_NAME}
    if [ -z "$UBUNTU_RELEASE" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${UBUNTU_RELEASE}"
    fi
    PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
    echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    installPkgWithAptGet "azure-acr-credential-provider" "${packageVersion}" || exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
    mv "/usr/local/bin/azure-acr-credential-provider" "$CREDENTIAL_PROVIDER_BIN_DIR/acr-credential-provider"
}

installKubeletKubectlPkgFromPMC() {
    k8sVersion="${1}"
    installPkgWithAptGet "kubelet" "${k8sVersion}" || exit $ERR_KUBELET_INSTALL_FAIL
    installPkgWithAptGet "kubectl" "${k8sVersion}" || exit $ERR_KUBECTL_INSTALL_FAIL
}

installToolFromLocalRepo() {
    local tool_name=$1
    local tool_download_dir=$2

    # Verify the download directory exists and contains repository metadata
    if [ ! -d "${tool_download_dir}" ]; then
        echo "Download directory ${tool_download_dir} does not exist"
        return 1
    fi

    # Check if this is a self-contained local repository (has Packages.gz)
    if [ ! -f "${tool_download_dir}/Packages.gz" ]; then
        echo "Packages.gz not found in ${tool_download_dir}, not a valid local repository"
        return 1
    fi

    echo "Installing ${tool_name} from local repository at ${tool_download_dir}..."
    if ! apt_get_install_from_local_repo "${tool_download_dir}" "${tool_name}"; then
        echo "Failed to install ${tool_name} from local repository"
        return 1
    fi

    echo "${tool_name} installed successfully from local repository"
    return 0
}

installCredentialProviderPackageFromBootstrapProfileRegistry() {
    bootstrapProfileRegistry="$1"
    k8sVersion="${2:-}"

    os=${UBUNTU_OS_NAME}
    if [ -z "$UBUNTU_RELEASE" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${UBUNTU_RELEASE}"
    fi
    PACKAGE_VERSION=""
    getLatestPkgVersionFromK8sVersion "$k8sVersion" "azure-acr-credential-provider-pmc" "$os" "$os_version"
    packageVersion=$(echo $PACKAGE_VERSION | cut -d "-" -f 1)
    if [ -z "$packageVersion" ]; then
        packageVersion=$(echo "$CREDENTIAL_PROVIDER_DOWNLOAD_URL" | grep -oP 'v\d+(\.\d+)*' | sed 's/^v//' | head -n 1)
        if [ -z "$packageVersion" ]; then
            echo "Failed to determine package version for azure-acr-credential-provider"
            return $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        fi
    fi
    echo "installing azure-acr-credential-provider package version: $packageVersion"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    if ! installToolFromBootstrapProfileRegistry "azure-acr-credential-provider" $bootstrapProfileRegistry "${packageVersion}" "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"; then
        if [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != "true" ] ; then
            # SHOULD_ENFORCE_KUBE_PMC_INSTALL will only be set for e2e tests, which should not fallback to reflect result of package installation behavior
            echo "Fall back to install credential provider from url installation"
            installCredentialProviderFromUrl
        else
            echo "Failed to install credential provider from bootstrap profile registry, and not falling back to package installation"
            exit $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        fi
    fi
}

installPkgWithAptGet() {
    packageName="${1:-}"
    packageVersion="${2}"
    downloadDir="/opt/${packageName}/downloads"
    packagePrefix="${packageName}_${packageVersion}-*"

    # if no deb file with desired version found then try fetching from packages.microsoft repo
    debFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || debFile=""
    if [ -z "${debFile}" ]; then
        if fallbackToKubeBinaryInstall "${packageName}" "${packageVersion}"; then
            echo "Successfully installed ${packageName} version ${packageVersion} from binary fallback"
            rm -rf ${downloadDir}
            return 0
        fi

        # query all package versions and get the latest version for matching k8s version
        updatePMCRepository
        fullPackageVersion=$(apt list ${packageName} --all-versions | grep ${packageVersion}- | awk '{print $2}' | sort -V | tail -n 1)
        if [ -z "${fullPackageVersion}" ]; then
            echo "Failed to find valid ${packageName} version for ${packageVersion}"
            return 1
        fi
        echo "Did not find cached deb file, downloading ${packageName} version ${fullPackageVersion}"
        logs_to_events "AKS.CSE.install${packageName}PkgFromPMC.downloadPkgFromVersion" "downloadPkgFromVersion ${packageName} ${fullPackageVersion} ${downloadDir}"
        debFile=$(find "${downloadDir}" -maxdepth 1 -name "${packagePrefix}" -print -quit 2>/dev/null) || debFile=""
    fi
    if [ -z "${debFile}" ]; then
        echo "Failed to locate ${packageName} deb"
        return 1
    fi

    logs_to_events "AKS.CSE.install${packageName}.installDebPackageFromFile" "installDebPackageFromFile ${debFile}" || exit $ERR_APT_INSTALL_TIMEOUT

    mv "/usr/bin/${packageName}" "/usr/local/bin/${packageName}"
    rm -rf ${downloadDir}
}

downloadPkgFromVersion() {
    packageName="${1:-}"
    packageVersion="${2:-}"
    downloadDir="${3:-"/opt/${packageName}/downloads"}"
    mkdir -p ${downloadDir}
    apt_get_download 20 30 ${packageName}=${packageVersion} || exit $ERR_APT_INSTALL_TIMEOUT
    # Strip epoch (e.g., 1:4.4.1-1 -> 4.4.1-1)
    version_no_epoch="${packageVersion#*:}"
    cp -al "${APT_CACHE_DIR}/${packageName}_${version_no_epoch}"* "${downloadDir}/" || exit $ERR_APT_INSTALL_TIMEOUT
    echo "Succeeded to download ${packageName} version ${packageVersion}"
}

installContainerd() {
    packageVersion="${3:-}"
    containerdMajorMinorPatchVersion="$(echo "$packageVersion" | cut -d- -f1)"
    containerdHotFixVersion="$(echo "$packageVersion" | cut -d- -f2)"
    CONTAINERD_DOWNLOADS_DIR="${1:-$CONTAINERD_DOWNLOADS_DIR}"
    eval containerdOverrideDownloadURL="${2:-}"

    # the user-defined package URL is always picked first, and the other options won't be tried when this one fails
    if [ ! -z "${containerdOverrideDownloadURL}" ]; then
        installContainerdFromOverride ${containerdOverrideDownloadURL} || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi
    installContainerdWithAptGet "${containerdMajorMinorPatchVersion}" "${containerdHotFixVersion}" "${CONTAINERD_DOWNLOADS_DIR}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
}

installContainerdFromOverride() {
    containerdOverrideDownloadURL=$1
    echo "Installing containerd from user input: ${containerdOverrideDownloadURL}"
    # we'll use a user-defined containerd package to install containerd even though it's the same version as
    # the one already installed on the node considering the source is built by the user for hotfix or test
    logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd
    logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromURL" downloadContainerdFromURL "${containerdOverrideDownloadURL}"
    logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${CONTAINERD_DEB_FILE}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    echo "Succeeded to install containerd from user input: ${containerdOverrideDownloadURL}"
    return 0
}

installContainerdWithAptGet() {
    local containerdMajorMinorPatchVersion="${1}"
    local containerdHotFixVersion="${2}"
    CONTAINERD_DOWNLOADS_DIR="${3:-$CONTAINERD_DOWNLOADS_DIR}"
    # azure-built runtimes have a "+azure" suffix in their version strings (i.e 1.4.1+azure). remove that here.
    currentVersion=""
    if command -v containerd &> /dev/null; then
        currentVersion=$(containerd -version | cut -d " " -f 3 | sed 's|v||' | cut -d "+" -f 1)
    fi
    # v1.4.1 is our lowest supported version of containerd

    if [ -z "$currentVersion" ]; then
        currentVersion="0.0.0"
    fi

    currentMajorMinor="$(echo $currentVersion | tr '.' '\n' | head -n 2 | paste -sd.)"
    desiredMajorMinor="$(echo $containerdMajorMinorPatchVersion | tr '.' '\n' | head -n 2 | paste -sd.)"
    semverCompare "$currentVersion" "$containerdMajorMinorPatchVersion"
    hasGreaterVersion="$?"

    if [ "$hasGreaterVersion" = "0" ] && [ "$currentMajorMinor" = "$desiredMajorMinor" ]; then
        echo "currently installed containerd version ${currentVersion} matches major.minor with higher patch ${containerdMajorMinorPatchVersion}. skipping installStandaloneContainerd."
    else
        echo "installing containerd version ${containerdMajorMinorPatchVersion}"
        logs_to_events "AKS.CSE.installContainerRuntime.removeContainerd" removeContainerd

        # if containerd version has been overriden then there should exist a local .deb file for it on aks VHDs (best-effort)
        # if no files found then try fetching from packages.microsoft repo
        containerdDebFile=$(find "${CONTAINERD_DOWNLOADS_DIR}" -maxdepth 1 -name "moby-containerd_${containerdMajorMinorPatchVersion}*" -print -quit 2>/dev/null) || containerdDebFile=""
        if [ -n "${containerdDebFile}" ]; then
            logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${containerdDebFile}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
            return 0
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.downloadContainerdFromVersion" "downloadContainerdFromVersion ${containerdMajorMinorPatchVersion} ${containerdHotFixVersion}"
        containerdDebFile=$(find "${CONTAINERD_DOWNLOADS_DIR}" -maxdepth 1 -name "moby-containerd_${containerdMajorMinorPatchVersion}*" -print -quit 2>/dev/null) || containerdDebFile=""
        if [ -z "${containerdDebFile}" ]; then
            echo "Failed to locate cached containerd deb"
            exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        fi
        logs_to_events "AKS.CSE.installContainerRuntime.installDebPackageFromFile" "installDebPackageFromFile ${containerdDebFile}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi
}

# CSE+VHD can dictate the containerd version, users don't care as long as it works
installStandaloneContainerd() {
    UBUNTU_RELEASE=$(lsb_release -r -s)
    UBUNTU_CODENAME=$(lsb_release -c -s)
    CONTAINERD_VERSION=$1
    # we always default to the .1 patch versons
    CONTAINERD_PATCH_VERSION="${2:-1}"

    # the user-defined package URL is always picked first, and the other options won't be tried when this one fails
    CONTAINERD_PACKAGE_URL="${CONTAINERD_PACKAGE_URL:=}"
    if [ ! -z "${CONTAINERD_PACKAGE_URL}" ]; then
        installContainerdFromOverride ${CONTAINERD_PACKAGE_URL} || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
        return 0
    fi

    #if there is no containerd_version input from RP, use hardcoded version
    if [ -z "${CONTAINERD_VERSION}" ]; then
        # pin 18.04 to 1.7.1
        CONTAINERD_VERSION="1.7.15"
        if [ "${UBUNTU_RELEASE}" = "18.04" ]; then
            CONTAINERD_VERSION="1.7.1"
        fi
        CONTAINERD_PATCH_VERSION="1"
        echo "Containerd Version not specified, using default version: ${CONTAINERD_VERSION}-${CONTAINERD_PATCH_VERSION}"
    else
        echo "Using specified Containerd Version: ${CONTAINERD_VERSION}-${CONTAINERD_PATCH_VERSION}"
    fi

    installContainerdWithAptGet "${CONTAINERD_VERSION}" "${CONTAINERD_PATCH_VERSION}" || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
}

downloadContainerdFromVersion() {
    # Patch version isn't used here...?
    CONTAINERD_VERSION=$1
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    # Adding updateAptWithMicrosoftPkg since AB e2e uses an older image version with uncached containerd 1.6 so it needs to download from testing repo.
    # And RP no image pull e2e has apt update restrictions that prevent calls to packages.microsoft.com in CSE
    # This won't be called for new VHDs as they have containerd 1.6 cached
    updateAptWithMicrosoftPkg
    apt_get_download 20 30 moby-containerd=${CONTAINERD_VERSION}* || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    cp -al ${APT_CACHE_DIR}moby-containerd_${CONTAINERD_VERSION}* $CONTAINERD_DOWNLOADS_DIR/ || exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    echo "Succeeded to download containerd version ${CONTAINERD_VERSION}"
}

downloadContainerdFromURL() {
    CONTAINERD_DOWNLOAD_URL=$1
    logs_to_events "AKS.CSE.logDownloadURL" "echo $CONTAINERD_DOWNLOAD_URL"
    CONTAINERD_DOWNLOAD_URL=$(update_base_url $CONTAINERD_DOWNLOAD_URL)
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}

ensureRunc() {
    RUNC_PACKAGE_URL=${2:-""}
    RUNC_DOWNLOADS_DIR=${3:-$RUNC_DOWNLOADS_DIR}
    # the user-defined runc package URL is always picked first, and the other options won't be tried when this one fails
    if [ ! -z "${RUNC_PACKAGE_URL}" ]; then
        echo "Installing runc from user input: ${RUNC_PACKAGE_URL}"
        mkdir -p $RUNC_DOWNLOADS_DIR
        RUNC_DEB_TMP=${RUNC_PACKAGE_URL##*/}
        RUNC_DEB_FILE="$RUNC_DOWNLOADS_DIR/${RUNC_DEB_TMP}"
        retrycmd_curl_file 120 5 60 ${RUNC_DEB_FILE} ${RUNC_PACKAGE_URL} || exit $ERR_RUNC_DOWNLOAD_TIMEOUT
        # we'll use a user-defined containerd package to install containerd even though it's the same version as
        # the one already installed on the node considering the source is built by the user for hotfix or test
        installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
        echo "Succeeded to install runc from user input: ${RUNC_PACKAGE_URL}"
        return 0
    fi

    TARGET_VERSION=${1:-""}

    if [ "$(isARM64)" -eq 1 ]; then
        if [ "${TARGET_VERSION}" = "1.0.0-rc92" ] || [ "${TARGET_VERSION}" = "1.0.0-rc95" ]; then
            # only moby-runc-1.0.3+azure-1 exists in ARM64 ubuntu repo now, no 1.0.0-rc92 or 1.0.0-rc95
            return
        fi
    fi

    CPU_ARCH=$(getCPUArch)  #amd64 or arm64
    CURRENT_VERSION=""
    if command -v runc &> /dev/null; then
        CURRENT_VERSION=$(runc --version | head -n1 | sed 's/runc version //')
    fi
    CLEANED_TARGET_VERSION=${TARGET_VERSION}

    # after upgrading to 1.1.9, CURRENT_VERSION will also include the patch version (such as 1.1.9-1), so we trim it off
    # since we only care about the major and minor versions when determining if we need to install it
    CURRENT_VERSION=${CURRENT_VERSION%-*} # removes the -1 patch version (or similar)
    CLEANED_TARGET_VERSION=${CLEANED_TARGET_VERSION%-*} # removes the -ubuntu22.04u1 (or similar)

    if [ "${CURRENT_VERSION}" = "${CLEANED_TARGET_VERSION}" ]; then
        echo "target moby-runc version ${CLEANED_TARGET_VERSION} is already installed. skipping installRunc."
        return
    fi
    # if on a vhd-built image, first check if we've cached the deb file
    if [ -f "$VHD_LOGS_FILEPATH" ]; then
        RUNC_DEB_PATTERN="moby-runc_*.deb"
        RUNC_DEB_FILES=()
        RUNC_DEB_FILE=""
        while IFS= read -r file; do
            RUNC_DEB_FILES+=("$file")
        done < <(find "${RUNC_DOWNLOADS_DIR}" -type f -iname "${RUNC_DEB_PATTERN}" 2>/dev/null)
        if [ ${#RUNC_DEB_FILES[@]} -gt 0 ]; then
            RUNC_DEB_FILE=$(printf "%s\n" "${RUNC_DEB_FILES[@]}" | sort -V | tail -n1)
        fi
        if [ -n "${RUNC_DEB_FILE}" ] && [ -f "${RUNC_DEB_FILE}" ]; then
            echo "Found cached runc deb file: ${RUNC_DEB_FILE}"
            installDebPackageFromFile ${RUNC_DEB_FILE} || exit $ERR_RUNC_INSTALL_TIMEOUT
            return 0
        fi
    fi
    echo "No cached runc deb file is found. Using apt-get to install runc."
    apt_get_install 20 30 120 moby-runc=${TARGET_VERSION}* --allow-downgrades || exit $ERR_RUNC_INSTALL_TIMEOUT
}

#EOF
