#!/bin/bash

CC_SERVICE_IN_TMP=/opt/azure/containers/cc-proxy.service.in
CC_SOCKET_IN_TMP=/opt/azure/containers/cc-proxy.socket.in
CNI_CONFIG_DIR="/etc/cni/net.d"
CNI_BIN_DIR="/opt/cni/bin"
#TODO pull this out of componetns.json too?
CNI_DOWNLOADS_DIR="/opt/cni/downloads"
CRICTL_DOWNLOAD_DIR="/opt/crictl/downloads"
CRICTL_BIN_DIR="/usr/local/bin"
CONTAINERD_DOWNLOADS_DIR="/opt/containerd/downloads"
RUNC_DOWNLOADS_DIR="/opt/runc/downloads"
K8S_DOWNLOADS_DIR="/opt/kubernetes/downloads"
K8S_PRIVATE_PACKAGES_CACHE_DIR="/opt/kubernetes/downloads/private-packages"
K8S_REGISTRY_REPO="oss/binaries/kubernetes"
UBUNTU_RELEASE=$(lsb_release -r -s 2>/dev/null || echo "")
# For Mariner 2.0, this returns "MARINER" and for AzureLinux 3.0, this returns "AZURELINUX"
OS=$(if ls /etc/*-release 1> /dev/null 2>&1; then sort -r /etc/*-release | gawk 'match($0, /^(ID=(.*))$/, a) { print toupper(a[2]); exit }'; fi)
OS_VARIANT=$(if ls /etc/*-release 1> /dev/null 2>&1; then sort -r /etc/*-release | gawk 'match($0, /^(VARIANT_ID=(.*))$/, a) { print toupper(a[2]); exit }' | tr -d '"'; fi)
SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR="/opt/aks-secure-tls-bootstrap-client/downloads"
SECURE_TLS_BOOTSTRAP_CLIENT_BIN_DIR="/usr/local/bin"
TELEPORTD_PLUGIN_DOWNLOAD_DIR="/opt/teleportd/downloads"
CREDENTIAL_PROVIDER_DOWNLOAD_DIR="/opt/credentialprovider/downloads"
CREDENTIAL_PROVIDER_BIN_DIR="/var/lib/kubelet/credential-provider"
TELEPORTD_PLUGIN_BIN_DIR="/usr/local/bin"
MANIFEST_FILEPATH="/opt/azure/manifest.json"
COMPONENTS_FILEPATH="/opt/azure/components.json"
VHD_LOGS_FILEPATH="/opt/azure/vhd-install.complete"
MAN_DB_AUTO_UPDATE_FLAG_FILEPATH="/var/lib/man-db/auto-update"
CURL_OUTPUT=/tmp/curl_verbose.out
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
CPU_ARCH=""

setCPUArch() {
    CPU_ARCH=$(getCPUArch)
}

removeManDbAutoUpdateFlagFile() {
    rm -f $MAN_DB_AUTO_UPDATE_FLAG_FILEPATH
}

createManDbAutoUpdateFlagFile() {
    touch $MAN_DB_AUTO_UPDATE_FLAG_FILEPATH
}

cleanupContainerdDlFiles() {
    rm -rf $CONTAINERD_DOWNLOADS_DIR
}

# After the centralized packages changes, the containerd versions are only available in the components.json.
installContainerdWithComponentsJson() {
    os=${UBUNTU_OS_NAME}
    if [ -z "$UBUNTU_RELEASE" ]; then
        os=${OS}
        os_version="current"
    else
        os_version="${UBUNTU_RELEASE}"
    fi

    containerdPackage=$(jq ".Packages" "$COMPONENTS_FILEPATH" | jq ".[] | select(.name == \"containerd\")") || exit $ERR_CONTAINERD_VERSION_INVALID
    PACKAGE_VERSIONS=()
    if isMariner "${OS}" && [ "${IS_KATA}" = "true" ]; then
        os=${MARINER_KATA_OS_NAME}
    fi
    if isAzureLinux "${OS}" && [ "${IS_KATA}" = "true" ]; then
        os=${AZURELINUX_KATA_OS_NAME}
    fi
    updatePackageVersions "${containerdPackage}" "${os}" "${os_version}"

    #Containerd's versions array is expected to have only one element.
    #If it has more than one element, we will install the last element in the array.
    # shellcheck disable=SC3010
    if [[ ${#PACKAGE_VERSIONS[@]} -gt 1 ]]; then
        echo "WARNING: containerd package versions array has more than one element. Installing the last element in the array."
    fi
    # shellcheck disable=SC3010
    if [[ ${#PACKAGE_VERSIONS[@]} -eq 0 || ${PACKAGE_VERSIONS[0]} == "<SKIP>" ]]; then
        echo "INFO: containerd package versions array is either empty or the first element is <SKIP>. Skipping containerd installation."
        return 0
    fi
    # sort the array from lowest to highest version before getting the last element
    IFS=$'\n' sortedPackageVersions=($(sort -V <<<"${PACKAGE_VERSIONS[*]}"))
    unset IFS
    array_size=${#sortedPackageVersions[@]}
    if [ "$((array_size - 1))" -lt 0 ]; then
        last_index=0
    else
        last_index=$((array_size - 1))
    fi
    packageVersion=${sortedPackageVersions[${last_index}]}
    # containerd version is expected to be in the format major.minor.patch-hotfix
    # e.g., 1.4.3-1. Then containerdMajorMinorPatchVersion=1.4.3 and containerdHotFixVersion=1
    containerdMajorMinorPatchVersion="$(echo "$packageVersion" | cut -d- -f1)"
    containerdHotFixVersion="$(echo "$packageVersion" | cut -d- -s -f2)"
    if [ -z "$containerdMajorMinorPatchVersion" ] || [ "$containerdMajorMinorPatchVersion" = "null" ] || [ "$containerdHotFixVersion" = "null" ]; then
        echo "invalid containerd version: $packageVersion"
        exit $ERR_CONTAINERD_VERSION_INVALID
    fi
    logs_to_events "AKS.CSE.installContainerRuntime.installStandaloneContainerd" "installStandaloneContainerd ${containerdMajorMinorPatchVersion} ${containerdHotFixVersion}"
    echo "in installContainerRuntime - CONTAINERD_VERSION = ${packageVersion}"

}

# containerd versions definitions are only available in the manifest file before the centralized packages changes, before around early July 2024.
# After the centralized packages changes, the containerd versions are only available in the components.json.
installContainerdWithManifestJson() {
    local containerd_version
    if [ -f "$MANIFEST_FILEPATH" ]; then
        local containerd_version
        containerd_version="$(jq -r .containerd.edge "$MANIFEST_FILEPATH")"
    else
        echo "WARNING: containerd version not found in manifest, defaulting to hardcoded."
    fi
    containerd_patch_version="$(echo "$containerd_version" | cut -d- -f1)"
    containerd_revision="$(echo "$containerd_version" | cut -d- -f2)"
    if [ -z "$containerd_patch_version" ] || [ "$containerd_patch_version" = "null" ] || [ "$containerd_revision" = "null" ]; then
        echo "invalid container version: $containerd_version"
        exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    fi
    logs_to_events "AKS.CSE.installContainerRuntime.installStandaloneContainerd" "installStandaloneContainerd ${containerd_patch_version} ${containerd_revision}"
    echo "in installContainerRuntime - CONTAINERD_VERSION = ${containerd_patch_version}"
}

installContainerRuntime() {
    echo "in installContainerRuntime - KUBERNETES_VERSION = ${KUBERNETES_VERSION}"
    if [ -f "$COMPONENTS_FILEPATH" ] && jq '.Packages[] | select(.name == "containerd")' < $COMPONENTS_FILEPATH > /dev/null; then
        echo "Package \"containerd\" exists in $COMPONENTS_FILEPATH."
        # if the containerd package is available in the components.json, use the components.json to install containerd
        installContainerdWithComponentsJson
		return
    fi
    echo "Package \"containerd\" does not exist in $COMPONENTS_FILEPATH."
    # if the containerd package is not available in the components.json, use the manifest.json to install containerd
    # we don't support Kata case for this old route with manifest.json
    installContainerdWithManifestJson
}

installNetworkPlugin() {
    if [ "${NETWORK_PLUGIN}" = "azure" ]; then
        installAzureCNI
    fi
    installCNI #reference plugins. Mostly for kubenet but loopback plugin is used by containerd until containerd 2
    rm -rf $CNI_DOWNLOADS_DIR &
}

# downloadCredentialProvider is always called during build time by install-dependencies.sh.
# It can also be called during node provisioning by cse_config.sh, meaning CREDENTIAL_PROVIDER_DOWNLOAD_URL is set by a passed in linuxCredentialProviderURL.
downloadCredentialProvider() {
    CREDENTIAL_PROVIDER_DOWNLOAD_URL="${CREDENTIAL_PROVIDER_DOWNLOAD_URL:=}"
    if [ -n "${CREDENTIAL_PROVIDER_DOWNLOAD_URL}" ]; then
        # CREDENTIAL_PROVIDER_DOWNLOAD_URL is set by linuxCredentialProviderURL
        # The version in the URL is unknown. An acs-mirror or registry URL could be passed meaning the version must be extracted from the URL.
        cred_version_for_oras=$(echo "$CREDENTIAL_PROVIDER_DOWNLOAD_URL" | grep -oP 'v\d+(\.\d+)*' | sed 's/^v//' | head -n 1)
    fi

    local cred_provider_url=$2
    if [ -n "$cred_provider_url" ]; then
        # CREDENTIAL_PROVIDER_DOWNLOAD_URL is passed in through install-dependencies.sh
        CREDENTIAL_PROVIDER_DOWNLOAD_URL=$cred_provider_url
    fi

    logs_to_events "AKS.CSE.logDownloadURL" "echo $CREDENTIAL_PROVIDER_DOWNLOAD_URL"
    CREDENTIAL_PROVIDER_DOWNLOAD_URL=$(update_base_url $CREDENTIAL_PROVIDER_DOWNLOAD_URL)

    mkdir -p $CREDENTIAL_PROVIDER_DOWNLOAD_DIR

    # if there is a container registry then oras is needed to download
    BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER:=}"
    # if BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER is set to non-empty string
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        local credential_provider_download_url_for_oras="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/${K8S_REGISTRY_REPO}/azure-acr-credential-provider:v${cred_version_for_oras}-linux-${CPU_ARCH}"
        CREDENTIAL_PROVIDER_TGZ_TMP="${CREDENTIAL_PROVIDER_DOWNLOAD_URL##*/}" # Use bash builtin ## to remove all chars ("*") up to the final "/"
        retrycmd_get_tarball_from_registry_with_oras 120 5 "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP" "${credential_provider_download_url_for_oras}" || exit $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        return
    elif isRegistryUrl "${CREDENTIAL_PROVIDER_DOWNLOAD_URL}"; then
        # if the URL is a registry URL, then download the credential provider using oras
        # extract version v1.30.0 from format like mcr.microsoft.com/oss/binaries/kubernetes/azure-acr-credential-provider:v1.30.0-linux-amd64
        local cred_version=$(echo "$CREDENTIAL_PROVIDER_DOWNLOAD_URL" | grep -oP 'v\d+(\.\d+)*' | head -n 1)
        CREDENTIAL_PROVIDER_TGZ_TMP="azure-acr-credential-provider-linux-${CPU_ARCH}-${cred_version}.tar.gz"
        retrycmd_get_tarball_from_registry_with_oras 120 5 "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP" "${CREDENTIAL_PROVIDER_DOWNLOAD_URL}" || exit $ERR_ORAS_PULL_CREDENTIAL_PROVIDER
        return
    fi

    CREDENTIAL_PROVIDER_TGZ_TMP="${CREDENTIAL_PROVIDER_DOWNLOAD_URL##*/}" # Use bash builtin ## to remove all chars ("*") up to the final "/"
    echo "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP ... $CREDENTIAL_PROVIDER_DOWNLOAD_URL"
    retrycmd_get_tarball 120 5 "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP" $CREDENTIAL_PROVIDER_DOWNLOAD_URL || exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
    echo "Credential Provider downloaded successfully"
}

installCredentialProviderFromUrl() {
    logs_to_events "AKS.CSE.installCredentialProviderFromUrl.downloadCredentialProvider" downloadCredentialProvider
    extract_tarball "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/${CREDENTIAL_PROVIDER_TGZ_TMP}" "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR"
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    mv "${CREDENTIAL_PROVIDER_DOWNLOAD_DIR}/azure-acr-credential-provider" "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"
    chmod 755 "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"
    rm -rf ${CREDENTIAL_PROVIDER_DOWNLOAD_DIR}
}

# TODO (alburgess) have oras version managed by dependant or Renovate
installOras() {
    ORAS_DOWNLOAD_DIR="/opt/oras/downloads"
    ORAS_EXTRACTED_DIR=${1} # Use components.json var for /usr/local/bin for linux-vhd-content-test.sh binary file checks.
    ORAS_DOWNLOAD_URL=${2}
    ORAS_VERSION=${3}

    mkdir -p $ORAS_DOWNLOAD_DIR

    echo "Installing Oras version $ORAS_VERSION..."
    ORAS_TMP=${ORAS_DOWNLOAD_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" ${ORAS_DOWNLOAD_URL} || exit $ERR_ORAS_DOWNLOAD_ERROR

    if [ ! -f "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" ]; then
        echo "File $ORAS_DOWNLOAD_DIR/${ORAS_TMP} does not exist."
        exit $ERR_ORAS_DOWNLOAD_ERROR
    fi

    echo "File $ORAS_DOWNLOAD_DIR/${ORAS_TMP} exists."
    # no-same-owner because the files in the tarball are owned by 1001:admin
    extract_tarball "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" "$ORAS_EXTRACTED_DIR/"
    rm -r "$ORAS_DOWNLOAD_DIR"
    echo "Oras version $ORAS_VERSION installed successfully."
}

# this is called called during node provisioning -
# if secure TLS bootstrapping is disabled, this will simply remove the client binary from disk.
# otherwise, if a custom URL is provided, it will use the custom URL to overwrite the existing installation
installSecureTLSBootstrapClient() {
    # TODO(cameissner): can probably remove this once we get to preview
    if [ "${ENABLE_SECURE_TLS_BOOTSTRAPPING}" != "true" ]; then
        echo "secure TLS bootstrapping is disabled, will remove secure TLS bootstrap client binary installation"
        rm -f "${SECURE_TLS_BOOTSTRAP_CLIENT_BIN_DIR}/aks-secure-tls-bootstrap-client" &
        rm -rf "${SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR}" &
        return 0
    fi

    # this is mainly for development purposes so we can test different versions of the bootstrap client
    # without having to tag new versions of AgentBaker, in the end we probably won't honor custom URLs specified
    # by the bootstrapper for this particular binary. In the end, if we do decide to support this, we will need
    # to make sure to use oras to download the client binary and ensure the binary itself is hosted within MCR.
    if [ -z "${CUSTOM_SECURE_TLS_BOOTSTRAPPING_CLIENT_DOWNLOAD_URL}" ]; then
        echo "secure TLS bootstrapping is enabled but no custom client download URL was provided, nothing to download"
        return 0
    fi

    downloadSecureTLSBootstrapClient "${SECURE_TLS_BOOTSTRAP_CLIENT_BIN_DIR}" "${CUSTOM_SECURE_TLS_BOOTSTRAPPING_CLIENT_DOWNLOAD_URL}" || exit $ERR_SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_ERROR
}

downloadSecureTLSBootstrapClient() {
    # TODO(cameissner): have this managed by renovate, migrate from github to MCR/packages.microsoft.com

    local CLIENT_EXTRACTED_DIR=${1-$:SECURE_TLS_BOOTSTRAP_CLIENT_BIN_DIR}
    local CLIENT_DOWNLOAD_URL=$2

    mkdir -p $SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR
    mkdir -p $CLIENT_EXTRACTED_DIR

    CLIENT_DOWNLOAD_URL=$(update_base_url $CLIENT_DOWNLOAD_URL)

    echo "installing aks-secure-tls-bootstrap-client from: $CLIENT_DOWNLOAD_URL"
    CLIENT_TGZ_TMP=${CLIENT_DOWNLOAD_URL##*/}
    retrycmd_get_tarball 120 5 "${SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR}/${CLIENT_TGZ_TMP}" ${CLIENT_DOWNLOAD_URL} || exit $ERR_SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_ERROR

    if [ -f "${CLIENT_EXTRACTED_DIR}/aks-secure-tls-bootstrap-client" ]; then
        echo "aks-secure-tls-bootstrap-client already exists in $CLIENT_EXTRACTED_DIR, will overwrite existing aks-secure-tls-bootstrap-client installation at ${CLIENT_EXTRACTED_DIR}/aks-secure-tls-bootstrap-client"
        rm -f "${CLIENT_EXTRACTED_DIR}/aks-secure-tls-bootstrap-client"
    fi

    extract_tarball "${SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR}/${CLIENT_TGZ_TMP}" "${SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR}/"
    mv "${SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR}/aks-secure-tls-bootstrap-client" "${CLIENT_EXTRACTED_DIR}/aks-secure-tls-bootstrap-client"
    chmod 755 "${CLIENT_EXTRACTED_DIR}/aks-secure-tls-bootstrap-client" || exit $ERR_SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_ERROR

    rm -rf "${SECURE_TLS_BOOTSTRAP_CLIENT_DOWNLOAD_DIR}"
    echo "aks-secure-tls-bootstrap-client installed successfully"
}

evalPackageDownloadURL() {
    local url=${1:-}
    if [ -n "$url" ]; then
         eval "result=${url}"
         echo $result
         return
    fi
    echo ""
}

downloadAzureCNI() {
    mkdir -p ${1-$:CNI_DOWNLOADS_DIR}
    # At VHD build time, the VNET_CNI_PLUGINS_URL is usually not set.
    # So, we will get the URL passed from install-depenencies.sh which is actually from components.json
    # At node provisioning time, if AKS-RP sets the VNET_CNI_PLUGINS_URL, then we will use that.
    VNET_CNI_PLUGINS_URL=${2:-$VNET_CNI_PLUGINS_URL}
    if [ -z "$VNET_CNI_PLUGINS_URL" ]; then
        echo "VNET_CNI_PLUGINS_URL is not set. Exiting..."
        return
    fi

    logs_to_events "AKS.CSE.logDownloadURL" "echo $VNET_CNI_PLUGINS_URL"
    VNET_CNI_PLUGINS_URL=$(update_base_url $VNET_CNI_PLUGINS_URL)

    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${VNET_CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}

downloadCrictl() {
    #if $1 is empty, take ${CRICTL_DOWNLOAD_DIR} as default value. Otherwise take $1 as the value
    downloadDir=${1:-${CRICTL_DOWNLOAD_DIR}}
    mkdir -p $downloadDir
    url=${2}
    logs_to_events "AKS.CSE.logDownloadURL" "echo $url"
    url=$(update_base_url $url)
    crictlTgzTmp=${url##*/}
    retrycmd_curl_file 10 5 60 "$downloadDir/${crictlTgzTmp}" ${url} || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT
}

installCrictl() {
    CPU_ARCH=$(getCPUArch)
    currentVersion=$(crictl --version 2>/dev/null | sed 's/crictl version //g')
    if [ -n "${currentVersion}" ]; then
        echo "version ${currentVersion} of crictl already installed. skipping installCrictl of target version ${KUBERNETES_VERSION%.*}.0"
    else
        # this is only called during cse. VHDs should have crictl binaries pre-cached so no need to download.
        # if the vhd does not have crictl pre-baked, return early
        CRICTL_TGZ_TEMP="crictl-v${CRICTL_VERSION}-linux-${CPU_ARCH}.tar.gz"
        if [ ! -f "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ]; then
            rm -rf ${CRICTL_DOWNLOAD_DIR}
            echo "pre-cached crictl not found: skipping installCrictl"
            return 1
        fi
        echo "Unpacking crictl into ${CRICTL_BIN_DIR}"
        extract_tarball "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" "${CRICTL_BIN_DIR}"
        chown root:root $CRICTL_BIN_DIR/crictl
        chmod 755 $CRICTL_BIN_DIR/crictl
    fi
}

downloadTeleportdPlugin() {
    DOWNLOAD_URL=$1
    TELEPORTD_VERSION=$2
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    if [ -z "${DOWNLOAD_URL}" ]; then
        echo "download url parameter for downloadTeleportdPlugin was not given"
        exit $ERR_TELEPORTD_DOWNLOAD_ERR
    fi
    if [ -z "${TELEPORTD_VERSION}" ]; then
        echo "teleportd version not given"
        exit $ERR_TELEPORTD_DOWNLOAD_ERR
    fi
    mkdir -p $TELEPORTD_PLUGIN_DOWNLOAD_DIR
    retrycmd_curl_file 10 5 60 "${TELEPORTD_PLUGIN_DOWNLOAD_DIR}/teleportd-v${TELEPORTD_VERSION}" "${DOWNLOAD_URL}/v${TELEPORTD_VERSION}/teleportd" || exit ${ERR_TELEPORTD_DOWNLOAD_ERR}
}

installTeleportdPlugin() {
    if [ "$(isARM64)" -eq 1 ]; then
        return
    fi

    CURRENT_VERSION=$(teleportd --version 2>/dev/null | sed 's/teleportd version v//g')
    local TARGET_VERSION="0.8.0"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${TARGET_VERSION}; then
        echo "currently installed teleportd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${TARGET_VERSION}. skipping installTeleportdPlugin."
    else
        logs_to_events "AKS.CSE.logDownloadURL" "echo $TELEPORTD_PLUGIN_DOWNLOAD_URL"
        TELEPORTD_PLUGIN_DOWNLOAD_URL=$(update_base_url $TELEPORTD_PLUGIN_DOWNLOAD_URL)
        downloadTeleportdPlugin ${TELEPORTD_PLUGIN_DOWNLOAD_URL} ${TARGET_VERSION}
        mv "${TELEPORTD_PLUGIN_DOWNLOAD_DIR}/teleportd-v${TELEPORTD_VERSION}" "${TELEPORTD_PLUGIN_BIN_DIR}/teleportd" || exit ${ERR_TELEPORTD_INSTALL_ERR}
        chmod 755 "${TELEPORTD_PLUGIN_BIN_DIR}/teleportd" || exit ${ERR_TELEPORTD_INSTALL_ERR}
    fi
    rm -rf ${TELEPORTD_PLUGIN_DOWNLOAD_DIR}
}

setupCNIDirs() {
    mkdir -p $CNI_BIN_DIR
    chown -R root:root $CNI_BIN_DIR
    chmod -R 755 $CNI_BIN_DIR

    mkdir -p $CNI_CONFIG_DIR
    chown -R root:root $CNI_CONFIG_DIR
    chmod 755 $CNI_CONFIG_DIR
}


# Reference CNI plugins is used by kubenet and the loopback plugin used by containerd 1.0 (dependency gone in 2.0)
# The version used to be deteremined by RP/toggle but are now just hadcoded in vhd as they rarely change and require a node image upgrade anyways
# Latest VHD should have the untar, older should have the tgz. And who knows will have neither.
installCNI() {
    # Old versions of VHDs will not have components.json. If it does not exist, we will fall back to the hardcoded download for CNI.
    # Network Isolated Cluster / Bring Your Own ACR will not work with a vhd that requres a hardcoded CNI download.
    if [ ! -f "$COMPONENTS_FILEPATH" ] || ! jq '.Packages[] | select(.name == "cni-plugins")' < $COMPONENTS_FILEPATH > /dev/null; then
        echo "WARNING: no cni-plugins components present falling back to hard coded download of 1.4.1. This should error eventually"
        # could we fail if not Ubuntu2204Gen2ContainerdPrivateKubePkg vhd? Are there others?
        # definitely not handling arm here.
        retrycmd_get_tarball 120 5 "${CNI_DOWNLOADS_DIR}/refcni.tar.gz" "https://${PACKAGE_DOWNLOAD_BASE_URL}/cni-plugins/v1.4.1/binaries/cni-plugins-linux-amd64-v1.4.1.tgz" || exit $ERR_CNI_DOWNLOAD_TIMEOUT
        extract_tarball "${CNI_DOWNLOADS_DIR}/refcni.tar.gz" "$CNI_BIN_DIR"
        return
    fi

    #always just use what is listed in components.json so we don't have to sync.
    cniPackage=$(jq ".Packages" "$COMPONENTS_FILEPATH" | jq ".[] | select(.name == \"cni-plugins\")") || exit $ERR_CNI_VERSION_INVALID

    #CNI doesn't really care about this but wanted to reuse updatePackageVersions which requires it.
    os=${UBUNTU_OS_NAME}
    if [ -z "$UBUNTU_RELEASE" ]; then
        os=${OS}
        os_version="current"
    fi
    os_version="${UBUNTU_RELEASE}"
    if isMarinerOrAzureLinux "${OS}" && [ "${IS_KATA}" = "true" ]; then
        os=${MARINER_KATA_OS_NAME}
    fi
    PACKAGE_VERSIONS=()
    updatePackageVersions "${cniPackage}" "${os}" "${os_version}"

    #should change to ne
    # shellcheck disable=SC3010
    if [[ ${#PACKAGE_VERSIONS[@]} -gt 1 ]]; then
        echo "WARNING: containerd package versions array has more than one element. Installing the last element in the array."
        exit $ERR_CONTAINERD_VERSION_INVALID
    fi
    packageVersion=${PACKAGE_VERSIONS[0]}

    # Is there a ${arch} variable I can use instead of the iff
    if [ "$(isARM64)" -eq 1 ]; then
        CNI_DIR_TMP="cni-plugins-linux-arm64-v${packageVersion}"
    else
        CNI_DIR_TMP="cni-plugins-linux-amd64-v${packageVersion}"
    fi

    if [ -d "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" ]; then
        #not clear to me when this would ever happen. assume its related to the line above Latest VHD should have the untar, older should have the tgz.
        mv ${CNI_DOWNLOADS_DIR}/${CNI_DIR_TMP}/* $CNI_BIN_DIR
    else
        echo "CNI tarball should already be unzipped by components.json"
        exit $ERR_CNI_VERSION_INVALID
    fi

    chown -R root:root $CNI_BIN_DIR
}

installAzureCNI() {
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}         # Use bash builtin % to remove the .tgz to look for a folder rather than tgz

    if [ -d "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" ]; then
        mv ${CNI_DOWNLOADS_DIR}/${CNI_DIR_TMP}/* $CNI_BIN_DIR
    else
        if [ ! -f "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ]; then
            logs_to_events "AKS.CSE.installAzureCNI.downloadAzureCNI" downloadAzureCNI
        fi

        extract_tarball "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" "$CNI_BIN_DIR"
    fi

    chown -R root:root $CNI_BIN_DIR
}

# extract the cached or downloaded kube package and remove
extractKubeBinariesToUsrLocalBin() {
    local k8s_tgz_tmp=$1
    local k8s_version=$2
    local is_private_url=$3

    extract_tarball "${k8s_tgz_tmp}" "/usr/local/bin" \
        --transform="s|.*|&-${k8s_version}|" --show-transformed-names --strip-components=3 \
        kubernetes/node/bin/kubelet kubernetes/node/bin/kubectl || exit $ERR_K8S_INSTALL_ERR
    if [ ! -f "/usr/local/bin/kubectl-${k8s_version}" ] || [ ! -f "/usr/local/bin/kubelet-${k8s_version}" ]; then
        exit $ERR_K8S_INSTALL_ERR
    fi
    if [ "$is_private_url" = "false" ]; then
        rm -f "${k8s_tgz_tmp}"
    fi
}

extractKubeBinaries() {
    local k8s_version="$1"
    # Remove leading 'v' if it exists
    k8s_version="${k8s_version#v}"
    local kube_binary_url="$2"
    local is_private_url="$3"
    local k8s_downloads_dir=${4:-"/opt/kubernetes/downloads"}

    logs_to_events "AKS.CSE.logDownloadURL" "echo $kube_binary_url"
    kube_binary_url=$(update_base_url $kube_binary_url)

    local k8s_tgz_tmp_filename=${kube_binary_url##*/}

    # if the private URL is specified and if the kube package is cached already, extract the package, return otherwise
    # if the private URL is not specified, download and extract the kube package from the given URL
    if [ "$is_private_url" = "true" ]; then
        k8s_tgz_tmp="${K8S_PRIVATE_PACKAGES_CACHE_DIR}/${k8s_tgz_tmp_filename}"

        if [ ! -f "${k8s_tgz_tmp}" ]; then
            echo "cached package ${k8s_tgz_tmp} not found"
            return 1
        fi

        echo "cached package ${k8s_tgz_tmp} found, will extract that"
        # remove the current kubelet and kubectl binaries before extracting new binaries from the cached package
        rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-*
    else
        k8s_tgz_tmp="${k8s_downloads_dir}/${k8s_tgz_tmp_filename}"
        mkdir -p ${k8s_downloads_dir}

        # if the url is a registry url, use oras to pull the artifact instead of curl
        if isRegistryUrl "${kube_binary_url}"; then
            echo "detect kube_binary_url, ${kube_binary_url}, as registry url, will use oras to pull artifact binary"
            # download the kube package from registry as oras artifact
            k8s_tgz_tmp="${k8s_downloads_dir}/kubernetes-node-linux-${CPU_ARCH}.tar.gz"
            retrycmd_get_tarball_from_registry_with_oras 120 5 "${k8s_tgz_tmp}" ${kube_binary_url} || exit $ERR_ORAS_PULL_K8S_FAIL
            if [ ! -f "${k8s_tgz_tmp}" ]; then
                exit "$ERR_ORAS_PULL_K8S_FAIL"
            fi
        else
            # download the kube package from the default URL
            retrycmd_get_tarball 120 5 "${k8s_tgz_tmp}" ${kube_binary_url} || exit $ERR_K8S_DOWNLOAD_TIMEOUT
            if [ ! -f "${k8s_tgz_tmp}" ] ; then
                exit "$ERR_K8S_DOWNLOAD_TIMEOUT"
            fi
        fi
    fi

    extractKubeBinariesToUsrLocalBin "${k8s_tgz_tmp}" "${k8s_version}" "${is_private_url}"
}

installToolFromBootstrapProfileRegistry() {
    local tool_name=$1
    local registry_server=$2
    local version=$3
    local install_path=$4

    # Try to pull distro-specific packages (e.g., .deb for Ubuntu) from registry
    local download_root="/tmp/kubernetes/downloads" # /opt folder will return permission error

    tool_package_url="${registry_server}/aks/packages/kubernetes/${tool_name}:v${version}"
    tool_download_dir="${download_root}/${tool_name}"
    mkdir -p "${tool_download_dir}"

    # Construct platform string for ORAS pull
    if [ -z "${OS_VERSION}" ]; then
        echo "OS_VERSION is not set"
        return 1
    fi
    platform_flag="--platform=linux/${CPU_ARCH}:${OS,,} ${OS_VERSION}"

    echo "Attempting to pull ${tool_name} package from ${tool_package_url} with platform ${platform_flag}"
    # retrycmd_pull_from_registry_with_oras will pull all artifacts to the directory with platform selection
    if ! retrycmd_pull_from_registry_with_oras 10 5 "${tool_download_dir}" "${tool_package_url}" "${platform_flag}"; then
        echo "Failed to pull ${tool_name} package from registry"
        rm -rf "${tool_download_dir}"
        return 1
    fi

    echo "Successfully pulled ${tool_name} package"

    if ! installToolFromLocalRepo "${tool_name}" "${tool_download_dir}"; then
        echo "Failed to install ${tool_name} from local repo ${tool_download_dir}"
        rm -rf "${tool_download_dir}"
        return 1
    fi
    if [ -n "$install_path" ]; then
        mv $(which ${tool_name}) $install_path
    fi

    # All tools installed successfully
    rm -rf "${download_root}"
    return 0
}

installKubeletKubectlFromBootstrapProfileRegistry() {
    local registry_server=$1
    local kubernetes_version=$2
    for tool_name in $(get_kubernetes_tools); do
        install_path="/usr/local/bin/${tool_name}"
        if ! installToolFromBootstrapProfileRegistry "${tool_name}" "${registry_server}" "${kubernetes_version}" "${install_path}"; then
            # SHOULD_ENFORCE_KUBE_PMC_INSTALL will only be set for e2e tests, which should not fallback to reflect result of package installation behavior
            # TODO: remove SHOULD_ENFORCE_KUBE_PMC_INSTALL check when the test cluster supports > 1.34.0 case
            # if install from bootstrap profile registry fails, fallback to install from URL
            if [ "${SHOULD_ENFORCE_KUBE_PMC_INSTALL}" != "true" ];then
                logs_to_events "AKS.CSE.configureKubeletAndKubectl.installKubeletKubectlFromURL-Fallback" installKubeletKubectlFromURL
                return
            else
                echo "Failed to install k8s tools from bootstrap profile registry, and not falling back to binary installation due to SHOULD_ENFORCE_KUBE_PMC_INSTALL=true"
                exit $ERR_ORAS_PULL_K8S_FAIL
            fi
        fi
    done
}

installKubeletKubectlFromURL() {
    # when both, custom and private urls for kubernetes packages are set, custom url will be used and private url will be ignored
    CUSTOM_KUBE_BINARY_DOWNLOAD_URL="${CUSTOM_KUBE_BINARY_URL:=}"
    PRIVATE_KUBE_BINARY_DOWNLOAD_URL="${PRIVATE_KUBE_BINARY_URL:=}"
    echo "using private url: ${PRIVATE_KUBE_BINARY_DOWNLOAD_URL}, custom url: ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}"
    install_default_if_missing=true

    if [ ! -z "${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}" ]; then
        # remove the kubelet and kubectl binaries to make sure the only binary left is from the CUSTOM_KUBE_BINARY_DOWNLOAD_URL
        rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-*

        # NOTE(mainred): we expect kubelet binary to be under `kubernetes/node/bin`. This suits the current setting of
        # kube binaries used by AKS and Kubernetes upstream.
        # TODO(mainred): let's see if necessary to auto-detect the path of kubelet
        logs_to_events "AKS.CSE.installKubeletKubectlFromURL.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL} false
        install_default_if_missing=false
    elif [ ! -z "${PRIVATE_KUBE_BINARY_DOWNLOAD_URL}" ]; then
        # extract new binaries from the cached package if exists (cached at build-time)
        logs_to_events "AKS.CSE.installKubeletKubectlFromURL.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${PRIVATE_KUBE_BINARY_DOWNLOAD_URL} true
    fi

    # if the custom url is not specified and the required kubectl/kubelet-version via private url is not installed, install using the default url/package
    if [ ! -f "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" ] || [ ! -f "/usr/local/bin/kubelet-${KUBERNETES_VERSION}" ]; then
        if [ "$install_default_if_missing" = "true" ]; then
            if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
                # network isolated cluster
                echo "Detect Bootstrap profile artifact is Cache, will use oras to pull artifact binary"
                updateKubeBinaryRegistryURL

                K8S_DOWNLOADS_TEMP_DIR_FROM_REGISTRY="/tmp/kubernetes/downloads" # /opt folder will return permission error
                logs_to_events "AKS.CSE.installKubeletKubectlFromURL.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} "${KUBE_BINARY_REGISTRY_URL:-}" false ${K8S_DOWNLOADS_TEMP_DIR_FROM_REGISTRY}
                # no egress traffic, default install will fail
                # will exit if the download fails

            #TODO: remove the condition check on KUBE_BINARY_URL once RP change is released
            elif (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)) && [ -n "${KUBE_BINARY_URL}" ]; then
                logs_to_events "AKS.CSE.installKubeletKubectlFromURL.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${KUBE_BINARY_URL} false
            fi
        fi
    fi
    mv "/usr/local/bin/kubelet-${KUBERNETES_VERSION}" "/usr/local/bin/kubelet"
    mv "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" "/usr/local/bin/kubectl"

    chmod a+x /usr/local/bin/kubelet /usr/local/bin/kubectl
    rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-* /home/hyperkube-downloads &
}

pullContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    PULL_RETRIES=10
    PULL_WAIT_SLEEP_SECONDS=1
    PULL_TIMEOUT_SECONDS=600 # 10 minutes

    echo "pulling the image ${CONTAINER_IMAGE_URL} using ${CLI_TOOL} with a timeout of ${PULL_TIMEOUT_SECONDS}s"

    if [ "${CLI_TOOL,,}" = "ctr" ]; then
        retrycmd_if_failure $PULL_RETRIES $PULL_WAIT_SLEEP_SECONDS $PULL_TIMEOUT_SECONDS ctr --namespace k8s.io image pull $CONTAINER_IMAGE_URL
        code=$?
    elif [ "${CLI_TOOL,,}" = "crictl" ]; then
        retrycmd_if_failure $PULL_RETRIES $PULL_WAIT_SLEEP_SECONDS $PULL_TIMEOUT_SECONDS crictl pull $CONTAINER_IMAGE_URL
        code=$?
    else
        retrycmd_if_failure $PULL_RETRIES $PULL_WAIT_SLEEP_SECONDS $PULL_TIMEOUT_SECONDS docker pull $CONTAINER_IMAGE_URL
        code=$?
    fi

    if [ "$code" -ne 0 ]; then
        if [ "$code" -ne 124 ]; then
            echo "failed to pull image ${CONTAINER_IMAGE_URL} using ${CLI_TOOL}, exit code: $code"
            return $code
        fi
        echo "timed out pulling image ${CONTAINER_IMAGE_URL} via ${CLI_TOOL}"
        if [ "${CLI_TOOL,,}" = "ctr" ]; then
            return $ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT
        elif [ "${CLI_TOOL,,}" = "crictl" ]; then
            return $ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT
        else
            return $ERR_CONTAINERD_DOCKER_IMG_PULL_TIMEOUT
        fi
    fi

    echo "successfully pulled image ${CONTAINER_IMAGE_URL} using ${CLI_TOOL}"
}

retagContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    RETAG_IMAGE_URL=$3
    echo "retagging from ${CONTAINER_IMAGE_URL} to ${RETAG_IMAGE_URL} using ${CLI_TOOL}"
    if [ "${CLI_TOOL}" = "ctr" ]; then
        ctr --namespace k8s.io image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    elif [ "${CLI_TOOL}" = "crictl" ]; then
        crictl image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    else
        docker image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    fi
}

labelContainerImage() {
    # ctr must be used to label container images, as docker and crictl do not support labeling.
    CONTAINER_IMAGE_URL=$1
    LABEL_KEY=$2
    LABEL_VALUE=$3
    echo "labeling image ${CONTAINER_IMAGE_URL} with ${LABEL_KEY}=${LABEL_VALUE} using ctr"
    ctr --namespace k8s.io image label $CONTAINER_IMAGE_URL $LABEL_KEY=$LABEL_VALUE
}

retagMCRImagesForChina() {
    # shellcheck disable=SC2016
        allMCRImages=($(ctr --namespace k8s.io images list | grep '^mcr.microsoft.com/' | awk '{print $1}'))
    if [ -z "${allMCRImages}" ]; then
        echo "failed to find mcr images for retag"
        return
    fi
    for mcrImage in ${allMCRImages[@]+"${allMCRImages[@]}"}; do
        # in mooncake, the mcr endpoint is: mcr.azk8s.cn
        # shellcheck disable=SC2001
        retagMCRImage=$(echo ${mcrImage} | sed -e 's/^mcr.microsoft.com/mcr.azk8s.cn/g')
        # can't use CLI_TOOL because crictl doesn't support retagging.
        retagContainerImage "ctr" ${mcrImage} ${retagMCRImage}
    done
}

removeContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    # crictl should always be present
    crictl rmi $CONTAINER_IMAGE_URL
}

cleanUpImages() {
    local targetImage=$1
    export targetImage
    # shellcheck disable=SC2329 # Function is invoked indirectly via bash -c
    function cleanupImagesRun() {
        if [ "${CLI_TOOL}" = "crictl" ]; then
            images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        else
            images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        fi
        local exit_code=$?
        if [ "$exit_code" -ne 0 ]; then
            exit $exit_code
        elif [ -n "${images_to_delete}" ]; then
            echo "${images_to_delete}" | while read -r image; do
                removeContainerImage ${CLI_TOOL} ${image}
            done
        fi
    }
    export -f cleanupImagesRun
    retrycmd_if_failure 10 5 120 bash -c cleanupImagesRun
}

cleanUpKubeProxyImages() {
    echo $(date),$(hostname), startCleanUpKubeProxyImages
    cleanUpImages "kube-proxy"
    echo $(date),$(hostname), endCleanUpKubeProxyImages
}

cleanupRetaggedImages() {
    if [ "${TARGET_CLOUD}" != "AzureChinaCloud" ]; then
        if [ "${CLI_TOOL}" = "crictl" ]; then
            images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
        else
            images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
        fi
        if [ -n "${images_to_delete}" ]; then
            echo "${images_to_delete}" | while read -r image; do
                removeContainerImage ${CLI_TOOL} ${image}
            done
        fi
    else
        echo "skipping container cleanup for AzureChinaCloud"
    fi
}

cleanUpContainerImages() {
    export KUBERNETES_VERSION
    export CLI_TOOL
    export -f retrycmd_if_failure
    export -f removeContainerImage
    export -f cleanUpImages
    export -f cleanUpKubeProxyImages
    bash -c cleanUpKubeProxyImages &
}

cleanUpContainerd() {
    rm -Rf $CONTAINERD_DOWNLOADS_DIR
}

getInstallModeAndCleanupContainerImages() {
    local SKIP_BINARY_CLEANUP=$1
    local IS_VHD=$2

    # shellcheck disable=SC3010
    if [ ! -f "$VHD_LOGS_FILEPATH" ] && [ "${IS_VHD,,}" = "true" ]; then
        echo "Using VHD distro but file $VHD_LOGS_FILEPATH not found"
        exit $ERR_VHD_FILE_NOT_FOUND
    fi

    FULL_INSTALL_REQUIRED=true
    if [ "${SKIP_BINARY_CLEANUP}" = "true" ]; then
        echo "binaries will not be cleaned up"
        echo "${FULL_INSTALL_REQUIRED,,}"
        return
    fi

    if [ -f $VHD_LOGS_FILEPATH ]; then
        echo "detected golden image pre-install"
        logs_to_events "AKS.CSE.cleanUpContainerImages" cleanUpContainerImages
        FULL_INSTALL_REQUIRED=false
    else
        echo "the file $VHD_LOGS_FILEPATH does not exist and IS_VHD is "${IS_VHD,,}", full install requred"
    fi

    echo "${FULL_INSTALL_REQUIRED,,}"
}

overrideNetworkConfig() {
    CONFIG_FILEPATH="/etc/cloud/cloud.cfg.d/80_azure_net_config.cfg"
    mkdir -p "${CONFIG_FILEPATH%/*}"
    touch ${CONFIG_FILEPATH}
    cat <<EOF >>${CONFIG_FILEPATH}
datasource:
    Azure:
        apply_network_config: false
EOF
}

#EOF
