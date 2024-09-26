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
UBUNTU_RELEASE=$(lsb_release -r -s)
OS=$(if ls /etc/*-release 1> /dev/null 2>&1; then sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }'; fi)
SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR="/opt/azure/tlsbootstrap"
SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_VERSION="v0.1.0-alpha.2"
TELEPORTD_PLUGIN_DOWNLOAD_DIR="/opt/teleportd/downloads"
CREDENTIAL_PROVIDER_DOWNLOAD_DIR="/opt/credentialprovider/downloads"
CREDENTIAL_PROVIDER_BIN_DIR="/var/lib/kubelet/credential-provider"
TELEPORTD_PLUGIN_BIN_DIR="/usr/local/bin"
MANIFEST_FILEPATH="/opt/azure/manifest.json"
COMPONENTS_FILEPATH="/opt/azure/components.json"
MAN_DB_AUTO_UPDATE_FLAG_FILEPATH="/var/lib/man-db/auto-update"
CURL_OUTPUT=/tmp/curl_verbose.out
UBUNTU_OS_NAME="UBUNTU"
MARINER_OS_NAME="MARINER"
CPU_ARCH=""
declare -a WASMSHIMPIDS=()
declare -a SPINKUBEPIDS=()

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

installContainerdWithComponentsJson() {
    os=${UBUNTU_OS_NAME}
    if [[ -z "$UBUNTU_RELEASE" ]]; then
        os=${OS}
        os_version="current"
    else
        os_version="${UBUNTU_RELEASE}"
    fi
    
    containerdPackage=$(jq ".Packages" "$COMPONENTS_FILEPATH" | jq ".[] | select(.name == \"containerd\")") || exit $ERR_CONTAINERD_VERSION_INVALID
    PACKAGE_VERSIONS=()
    if [[ "${os}" == "${MARINER_OS_NAME}" && "${IS_KATA}" == "true" ]]; then
        os=${MARINER_KATA_OS_NAME}
    fi
    updatePackageVersions "${containerdPackage}" "${os}" "${os_version}"
    
    #Containerd's versions array is expected to have only one element.
    #If it has more than one element, we will install the last element in the array.
    if [[ ${#PACKAGE_VERSIONS[@]} -gt 1 ]]; then
        echo "WARNING: containerd package versions array has more than one element. Installing the last element in the array."
    fi
    if [[ ${#PACKAGE_VERSIONS[@]} -eq 0 || ${PACKAGE_VERSIONS[0]} == "<SKIP>" ]]; then
        echo "INFO: containerd package versions array is either empty or the first element is <SKIP>. Skipping containerd installation."
        return 0
    fi
    IFS=$'\n' sortedPackageVersions=($(sort -V <<<"${PACKAGE_VERSIONS[*]}"))
    unset IFS
    array_size=${#sortedPackageVersions[@]}
    [[ $((array_size-1)) -lt 0 ]] && last_index=0 || last_index=$((array_size-1))
    packageVersion=${sortedPackageVersions[${last_index}]}
    containerdMajorMinorPatchVersion="$(echo "$packageVersion" | cut -d- -f1)"
    containerdHotFixVersion="$(echo "$packageVersion" | cut -d- -s -f2)"
    if [ -z "$containerdMajorMinorPatchVersion" ] || [ "$containerdMajorMinorPatchVersion" == "null" ] || [ "$containerdHotFixVersion" == "null" ]; then
        echo "invalid containerd version: $packageVersion"
        exit $ERR_CONTAINERD_VERSION_INVALID
    fi
    logs_to_events "AKS.CSE.installContainerRuntime.installStandaloneContainerd" "installStandaloneContainerd ${containerdMajorMinorPatchVersion} ${containerdHotFixVersion}"
    echo "in installContainerRuntime - CONTAINERD_VERSION = ${packageVersion}"

}

installContainerdWithManifestJson() {
    local containerd_version
    if [ -f "$MANIFEST_FILEPATH" ]; then
        local containerd_version
        containerd_version="$(jq -r .containerd.edge "$MANIFEST_FILEPATH")"
        if [ "${UBUNTU_RELEASE}" == "18.04" ]; then
            containerd_version="$(jq -r '.containerd.pinned."1804"' "$MANIFEST_FILEPATH")"
        fi
    else
        echo "WARNING: containerd version not found in manifest, defaulting to hardcoded."
    fi
    containerd_patch_version="$(echo "$containerd_version" | cut -d- -f1)"
    containerd_revision="$(echo "$containerd_version" | cut -d- -f2)"
    if [ -z "$containerd_patch_version" ] || [ "$containerd_patch_version" == "null" ] || [ "$containerd_revision" == "null" ]; then
        echo "invalid container version: $containerd_version"
        exit $ERR_CONTAINERD_INSTALL_TIMEOUT
    fi
    logs_to_events "AKS.CSE.installContainerRuntime.installStandaloneContainerd" "installStandaloneContainerd ${containerd_patch_version} ${containerd_revision}"
    echo "in installContainerRuntime - CONTAINERD_VERSION = ${containerd_patch_version}"
}

installContainerRuntime() {
    echo "in installContainerRuntime - KUBERNETES_VERSION = ${KUBERNETES_VERSION}"
    if [[ "${NEEDS_CONTAINERD}" != "true" ]]; then
        installMoby 
    fi
    if [ -f "$COMPONENTS_FILEPATH" ] && jq '.Packages[] | select(.name == "containerd")' < $COMPONENTS_FILEPATH > /dev/null; then
        echo "Package \"containerd\" exists in $COMPONENTS_FILEPATH."
        installContainerdWithComponentsJson
		return
    fi
    echo "Package \"containerd\" does not exist in $COMPONENTS_FILEPATH."
    installContainerdWithManifestJson
}

installNetworkPlugin() {
    if [[ "${NETWORK_PLUGIN}" = "azure" ]]; then
        installAzureCNI
    fi
    installCNI #reference plugins. Mostly for kubenet but loopback plugin is used by containerd until containerd 2
    rm -rf $CNI_DOWNLOADS_DIR & 
}

downloadCredentialProvider() {
    CREDENTIAL_PROVIDER_DOWNLOAD_URL="${CREDENTIAL_PROVIDER_DOWNLOAD_URL:=}"
    if [[ -n "${CREDENTIAL_PROVIDER_DOWNLOAD_URL}" ]]; then
        cred_version_for_oras=$(echo "$CREDENTIAL_PROVIDER_DOWNLOAD_URL" | grep -oP 'v\d+(\.\d+)*' | sed 's/^v//' | head -n 1)
    fi

    local cred_provider_url=$2
    if [[ -n $cred_provider_url ]]; then
        CREDENTIAL_PROVIDER_DOWNLOAD_URL=$cred_provider_url
    fi

    mkdir -p $CREDENTIAL_PROVIDER_DOWNLOAD_DIR

    BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER:=}"
    if [[ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]]; then
        local credential_provider_download_url_for_oras="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/${K8S_REGISTRY_REPO}/azure-acr-credential-provider:v${cred_version_for_oras}-linux-${CPU_ARCH}"
        CREDENTIAL_PROVIDER_TGZ_TMP="${CREDENTIAL_PROVIDER_DOWNLOAD_URL##*/}" # Use bash builtin #
        retrycmd_get_tarball_from_registry_with_oras 120 5 "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP" "${credential_provider_download_url_for_oras}" || exit $ERR_ORAS_PULL_K8S_FAIL
        return 
    fi

    CREDENTIAL_PROVIDER_TGZ_TMP="${CREDENTIAL_PROVIDER_DOWNLOAD_URL##*/}" # Use bash builtin #
    echo "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP ... $CREDENTIAL_PROVIDER_DOWNLOAD_URL"
    retrycmd_get_tarball 120 5 "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/$CREDENTIAL_PROVIDER_TGZ_TMP" $CREDENTIAL_PROVIDER_DOWNLOAD_URL || exit $ERR_CREDENTIAL_PROVIDER_DOWNLOAD_TIMEOUT
    echo "Credential Provider downloaded successfully"
}

installCredentialProvider() {
    logs_to_events "AKS.CSE.installCredentialProvider.downloadCredentialProvider" downloadCredentialProvider
    tar -xzf "$CREDENTIAL_PROVIDER_DOWNLOAD_DIR/${CREDENTIAL_PROVIDER_TGZ_TMP}" -C $CREDENTIAL_PROVIDER_DOWNLOAD_DIR
    mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
    chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
    mv "${CREDENTIAL_PROVIDER_DOWNLOAD_DIR}/azure-acr-credential-provider" "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"
    chmod 755 "${CREDENTIAL_PROVIDER_BIN_DIR}/acr-credential-provider"
    rm -rf ${CREDENTIAL_PROVIDER_DOWNLOAD_DIR}
}

downloadSecureTLSBootstrapKubeletExecPlugin() {
    local plugin_url="https://k8sreleases.blob.core.windows.net/aks-tls-bootstrap-client/${SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_VERSION}/linux/amd64/tls-bootstrap-client"
    if [[ $(isARM64) == 1 ]]; then
        plugin_url="https://k8sreleases.blob.core.windows.net/aks-tls-bootstrap-client/${SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_VERSION}/linux/arm64/tls-bootstrap-client"
    fi

    mkdir -p $SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR
    plugin_download_path="${SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR}/tls-bootstrap-client"

    if [ ! -f "$plugin_download_path" ]; then
        retrycmd_if_failure 30 5 60 curl -fSL -o "$plugin_download_path" "$plugin_url" || exit $ERR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_TIMEOUT
        chown -R root:root "$SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR"
        chmod -R 755 "$SECURE_TLS_BOOTSTRAP_KUBELET_EXEC_PLUGIN_DOWNLOAD_DIR"
    fi
}

wasmFilesExist() {
    local containerd_wasm_filepath=${1}
    local shim_version=${2}
    local version_suffix=${3}
    local shims_to_download=("${@:4}") 

    local binary_version="$(echo "${shim_version}" | tr . -)"
    for shim in "${shims_to_download[@]}"; do
        if [ ! -f "${containerd_wasm_filepath}/containerd-shim-${shim}-${binary_version}-${version_suffix}" ]; then
            return 1 
        fi
    done
    echo "all wasm files exist for ${containerd_wasm_filepath}/containerd-shim-*-${binary_version}-${version_suffix}"
    return 0 
}

installContainerdWasmShims(){
    local download_location=${1}
    PACKAGE_DOWNLOAD_URL=${2} 
    local package_versions=("${@:2}") 
    
    for version in "${package_versions[@]}"; do
        local shims_to_download=("spin" "slight")
        if [[ "$version" == "0.8.0" ]]; then
            shims_to_download+=("wws")
        fi
        containerd_wasm_url=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadContainerdWasmShims $download_location $containerd_wasm_url "v$version" "${shims_to_download[@]}" 
    done
    wait ${WASMSHIMPIDS[@]}
    for version in "${package_versions[@]}"; do
        local shims_to_download=("spin" "slight")
        if [[ "$version" == "0.8.0" ]]; then
            shims_to_download+=("wws")
        fi
        updateContainerdWasmShimsPermissions $download_location "v$version" "${shims_to_download[@]}"
    done
}

downloadContainerdWasmShims() {
    local containerd_wasm_filepath=${1}
    local containerd_wasm_url=${2}
    local shim_version=${3}
    local shims_to_download=("${@:4}") 

    local binary_version="$(echo "${shim_version}" | tr . -)" 

    if wasmFilesExist "$containerd_wasm_filepath" "$shim_version" "-v1" "${shims_to_download[@]}"; then
        echo "containerd-wasm-shims already exists in $containerd_wasm_filepath, will not be downloading."
        return
    fi

    BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER:=}"
    if [[ ! -z ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ]]; then
        local registry_url="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/oss/binaries/deislabs/containerd-wasm-shims:${shim_version}-linux-${CPU_ARCH}"
        local wasm_shims_tgz_tmp=$containerd_wasm_filepath/containerd-wasm-shims-linux-${CPU_ARCH}.tar.gz

        retrycmd_get_tarball_from_registry_with_oras 120 5 "${wasm_shims_tgz_tmp}" ${registry_url} || exit $ERR_ORAS_PULL_CONTAINERD_WASM
        tar -zxf "$wasm_shims_tgz_tmp" -C $containerd_wasm_filepath
        mv "$containerd_wasm_filepath/containerd-shim-*-${shim_version}-v1" "$containerd_wasm_filepath/containerd-shim-*-${binary_version}-v1"
        rm -f "$wasm_shims_tgz_tmp"
        return
    fi

    for shim in "${shims_to_download[@]}"; do
        retrycmd_if_failure 30 5 60 curl -fSLv -o "$containerd_wasm_filepath/containerd-shim-${shim}-${binary_version}-v1" "$containerd_wasm_url/containerd-shim-${shim}-v1" 2>&1 | tee $CURL_OUTPUT >/dev/null | grep -E "^(curl:.*)|([eE]rr.*)$" && (cat $CURL_OUTPUT && exit $ERR_KRUSTLET_DOWNLOAD_TIMEOUT) &
        WASMSHIMPIDS+=($!)
    done
}

updateContainerdWasmShimsPermissions() {
    local containerd_wasm_filepath=${1}
    local shim_version=${2}
    local shims_to_download=("${@:3}") 

    local binary_version="$(echo "${shim_version}" | tr . -)"

    for shim in "${shims_to_download[@]}"; do
        chmod 755 "$containerd_wasm_filepath/containerd-shim-${shim}-${binary_version}-v1"
    done
}

installSpinKube(){
    local download_location=${1}
    PACKAGE_DOWNLOAD_URL=${2}
    local package_versions=("${@:2}") 

    for version in "${package_versions[@]}"; do
        containerd_spinkube_url=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadSpinKube $download_location $containerd_spinkube_url "v$version" 
    done
    wait ${SPINKUBEPIDS[@]}
    for version in "${package_versions[@]}"; do
        chmod 755 "$download_location/containerd-shim-spin-v2"
    done
}

downloadSpinKube(){
    local containerd_spinkube_filepath=${1}
    local containerd_spinkube_url=${2}
    local shim_version=${3}
    local shims_to_download=("${@:4}") 

    if [ -f "$containerd_spinkube_filepath/containerd-shim-spin-v2" ]; then
        echo "containerd-shim-spin-v2 already exists in $containerd_spinkube_filepath, will not be downloading."
        return
    fi

    BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER:=}"
    if [[ ! -z ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ]]; then
        local registry_url="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/oss/binaries/spinkube/containerd-shim-spin:${shim_version}-linux-${CPU_ARCH}"
        local wasm_shims_tgz_tmp="${containerd_spinkube_filepath}/containerd-shim-spin-v2"
        retrycmd_get_binary_from_registry_with_oras 120 5 "${wasm_shims_tgz_tmp}" "${registry_url}" || exit $ERR_ORAS_PULL_CONTAINERD_WASM
        rm -f "$wasm_shims_tgz_tmp"
        return 
    fi
    
    retrycmd_if_failure 30 5 60 curl -fSLv -o "$containerd_spinkube_filepath/containerd-shim-spin-v2" "$containerd_spinkube_url/containerd-shim-spin-v2" 2>&1 | tee $CURL_OUTPUT >/dev/null | grep -E "^(curl:.*)|([eE]rr.*)$" && (cat $CURL_OUTPUT && exit $ERR_KRUSTLET_DOWNLOAD_TIMEOUT) &
    SPINKUBEPIDS+=($!)
}

installOras() {
    ORAS_DOWNLOAD_DIR="/opt/oras/downloads"
    ORAS_EXTRACTED_DIR=${1} 
    ORAS_DOWNLOAD_URL=${2}
    ORAS_VERSION=${3}

    mkdir -p $ORAS_DOWNLOAD_DIR

    echo "Installing Oras version $ORAS_VERSION..."
    ORAS_TMP=${ORAS_DOWNLOAD_URL##*/} # Use bash builtin #
    retrycmd_get_tarball 120 5 "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" ${ORAS_DOWNLOAD_URL} || exit $ERR_ORAS_DOWNLOAD_ERROR

    if [ ! -f "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" ]; then
        echo "File $ORAS_DOWNLOAD_DIR/${ORAS_TMP} does not exist."
        exit $ERR_ORAS_DOWNLOAD_ERROR
    fi

    echo "File $ORAS_DOWNLOAD_DIR/${ORAS_TMP} exists."
    sudo tar -zxf "$ORAS_DOWNLOAD_DIR/${ORAS_TMP}" -C $ORAS_EXTRACTED_DIR/
    rm -r "$ORAS_DOWNLOAD_DIR"
    echo "Oras version $ORAS_VERSION installed successfully."
}

evalPackageDownloadURL() {
    local url=${1:-}
    if [[ -n "$url" ]]; then
         eval "result=${url}"
         echo $result
         return
    fi
    echo ""
}

downloadAzureCNI() {
    mkdir -p ${1-$:CNI_DOWNLOADS_DIR}
    VNET_CNI_PLUGINS_URL=${2:-$VNET_CNI_PLUGINS_URL}
    if [[ -z "$VNET_CNI_PLUGINS_URL" ]]; then
        echo "VNET_CNI_PLUGINS_URL is not set. Exiting..."
        return
    fi
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin #
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${VNET_CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}

downloadCrictl() {
    #if $1 is empty, take ${CRICTL_DOWNLOAD_DIR} as default value. Otherwise take $1 as the value
    downloadDir=${1:-${CRICTL_DOWNLOAD_DIR}}
    mkdir -p $downloadDir
    url=${2}
    crictlTgzTmp=${url##*/}
    retrycmd_curl_file 10 5 60 "$downloadDir/${crictlTgzTmp}" ${url} || exit $ERR_CRICTL_DOWNLOAD_TIMEOUT
}

installCrictl() {
    CPU_ARCH=$(getCPUArch)
    currentVersion=$(crictl --version 2>/dev/null | sed 's/crictl version //g')
    if [[ "${currentVersion}" != "" ]]; then
        echo "version ${currentVersion} of crictl already installed. skipping installCrictl of target version ${KUBERNETES_VERSION%.*}.0"
    else
        CRICTL_TGZ_TEMP="crictl-v${CRICTL_VERSION}-linux-${CPU_ARCH}.tar.gz"
        if [[ ! -f "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ]]; then
            rm -rf ${CRICTL_DOWNLOAD_DIR}
            echo "pre-cached crictl not found: skipping installCrictl"
            return 1
        fi
        echo "Unpacking crictl into ${CRICTL_BIN_DIR}"
        tar zxvf "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" -C ${CRICTL_BIN_DIR}
        chown root:root $CRICTL_BIN_DIR/crictl
        chmod 755 $CRICTL_BIN_DIR/crictl
    fi
}

downloadTeleportdPlugin() {
    DOWNLOAD_URL=$1
    TELEPORTD_VERSION=$2
    if [[ $(isARM64) == 1 ]]; then
        return
    fi

    if [[ -z ${DOWNLOAD_URL} ]]; then
        echo "download url parameter for downloadTeleportdPlugin was not given"
        exit $ERR_TELEPORTD_DOWNLOAD_ERR
    fi
    if [[ -z ${TELEPORTD_VERSION} ]]; then
        echo "teleportd version not given"
        exit $ERR_TELEPORTD_DOWNLOAD_ERR
    fi
    mkdir -p $TELEPORTD_PLUGIN_DOWNLOAD_DIR
    retrycmd_curl_file 10 5 60 "${TELEPORTD_PLUGIN_DOWNLOAD_DIR}/teleportd-v${TELEPORTD_VERSION}" "${DOWNLOAD_URL}/v${TELEPORTD_VERSION}/teleportd" || exit ${ERR_TELEPORTD_DOWNLOAD_ERR}
}

installTeleportdPlugin() {
    if [[ $(isARM64) == 1 ]]; then
        return
    fi

    CURRENT_VERSION=$(teleportd --version 2>/dev/null | sed 's/teleportd version v//g')
    local TARGET_VERSION="0.8.0"
    if semverCompare ${CURRENT_VERSION:-"0.0.0"} ${TARGET_VERSION}; then
        echo "currently installed teleportd version ${CURRENT_VERSION} is greater than (or equal to) target base version ${TARGET_VERSION}. skipping installTeleportdPlugin."
    else
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


installCNI() {
    if [ ! -f "$COMPONENTS_FILEPATH" ] || ! jq '.Packages[] | select(.name == "cni-plugins")' < $COMPONENTS_FILEPATH > /dev/null; then
        echo "WARNING: no cni-plugins components present falling back to hard coded download of 1.4.1. This should error eventually" 
        retrycmd_get_tarball 120 5 "${CNI_DOWNLOADS_DIR}/refcni.tar.gz" "https://acs-mirror.azureedge.net/cni-plugins/v1.4.1/binaries/cni-plugins-linux-amd64-v1.4.1.tgz" || exit
        tar -xzf "${CNI_DOWNLOADS_DIR}/refcni.tar.gz" -C $CNI_BIN_DIR
        return 
    fi

    #always just use what is listed in components.json so we don't have to sync.
    cniPackage=$(jq ".Packages" "$COMPONENTS_FILEPATH" | jq ".[] | select(.name == \"cni-plugins\")") || exit $ERR_CNI_VERSION_INVALID
    
    #CNI doesn't really care about this but wanted to reuse updatePackageVersions which requires it.
    os=${UBUNTU_OS_NAME} 
    if [[ -z "$UBUNTU_RELEASE" ]]; then
        os=${OS}
        os_version="current"
    fi
    os_version="${UBUNTU_RELEASE}"
    if [[ "${os}" == "${MARINER_OS_NAME}" && "${IS_KATA}" == "true" ]]; then
        os=${MARINER_KATA_OS_NAME}
    fi
    PACKAGE_VERSIONS=()
    updatePackageVersions "${cniPackage}" "${os}" "${os_version}"
    
    #should change to ne
    if [[ ${#PACKAGE_VERSIONS[@]} -gt 1 ]]; then
        echo "WARNING: containerd package versions array has more than one element. Installing the last element in the array."
        exit $ERR_CONTAINERD_VERSION_INVALID
    fi
    packageVersion=${PACKAGE_VERSIONS[0]}

    if [[ $(isARM64) == 1 ]]; then 
        CNI_DIR_TMP="cni-plugins-linux-arm64-v${packageVersion}"
    else 
        CNI_DIR_TMP="cni-plugins-linux-amd64-v${packageVersion}"
    fi
    
    if [[ -d "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" ]]; then
        #not clear to me when this would ever happen. assume its related to the line above Latest VHD should have the untar, older should have the tgz. 
        mv ${CNI_DOWNLOADS_DIR}/${CNI_DIR_TMP}/* $CNI_BIN_DIR 
    else
        echo "CNI tarball should already be unzipped by components.json"
        exit $ERR_CNI_VERSION_INVALID
    fi

    chown -R root:root $CNI_BIN_DIR
}

installAzureCNI() {
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin #
    CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}         

    if [[ -d "$CNI_DOWNLOADS_DIR/${CNI_DIR_TMP}" ]]; then
        mv ${CNI_DOWNLOADS_DIR}/${CNI_DIR_TMP}/* $CNI_BIN_DIR
    else
        if [[ ! -f "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ]]; then
            logs_to_events "AKS.CSE.installAzureCNI.downloadAzureCNI" downloadAzureCNI
        fi

        tar -xzf "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" -C $CNI_BIN_DIR
    fi

    chown -R root:root $CNI_BIN_DIR
}

extractKubeBinaries() {
    local k8s_version="$1"
    local kube_binary_url="$2"
    local is_private_url="$3"
    local k8s_downloads_dir="$4"

    local k8s_tgz_tmp_filename=${kube_binary_url##*/}

    if [[ $is_private_url == true ]]; then
        k8s_tgz_tmp="${K8S_PRIVATE_PACKAGES_CACHE_DIR}/${k8s_tgz_tmp_filename}"

        if [[ ! -f "${k8s_tgz_tmp}" ]]; then
            echo "cached package ${k8s_tgz_tmp} not found"
            return 1
        fi

        echo "cached package ${k8s_tgz_tmp} found, will extract that"
        rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-*
    else
        k8s_tgz_tmp="${k8s_downloads_dir}/${k8s_tgz_tmp_filename}"
        mkdir -p ${k8s_downloads_dir}
        if isRegistryUrl "${kube_binary_url}"; then
            echo "detect kube_binary_url, ${kube_binary_url}, as registry url, will use oras to pull artifact binary"
            k8s_tgz_tmp="${k8s_downloads_dir}/kubernetes-node-linux-${CPU_ARCH}.tar.gz"
            retrycmd_get_tarball_from_registry_with_oras 120 5 "${k8s_tgz_tmp}" ${kube_binary_url} || exit $ERR_ORAS_PULL_K8S_FAIL
            if [[ ! -f "${k8s_tgz_tmp}" ]]; then
                exit "$ERR_ORAS_PULL_K8S_FAIL"
            fi
        else
            retrycmd_get_tarball 120 5 "${k8s_tgz_tmp}" ${kube_binary_url} || exit $ERR_K8S_DOWNLOAD_TIMEOUT
            if [[ ! -f "${k8s_tgz_tmp}" ]]; then
                exit "$ERR_K8S_DOWNLOAD_TIMEOUT"
            fi
        fi
    fi

    tar --transform="s|.*|&-${k8s_version}|" --show-transformed-names -xzvf "${k8s_tgz_tmp}" \
        --strip-components=3 -C /usr/local/bin kubernetes/node/bin/kubelet kubernetes/node/bin/kubectl || exit $ERR_K8S_INSTALL_ERR
    if [[ ! -f /usr/local/bin/kubectl-${k8s_version} ]] || [[ ! -f /usr/local/bin/kubelet-${k8s_version} ]]; then
        exit $ERR_K8S_INSTALL_ERR
    fi

    if [[ $is_private_url == false ]]; then
        rm -f "${k8s_tgz_tmp}"
    fi
}

installKubeletKubectlAndKubeProxy() {
    CUSTOM_KUBE_BINARY_DOWNLOAD_URL="${CUSTOM_KUBE_BINARY_URL:=}"
    PRIVATE_KUBE_BINARY_DOWNLOAD_URL="${PRIVATE_KUBE_BINARY_URL:=}"
    echo "using private url: ${PRIVATE_KUBE_BINARY_DOWNLOAD_URL}, custom url: ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}"
    install_default_if_missing=true

    if [[ ! -z ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL} ]]; then
        rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-*

        logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL} false
        install_default_if_missing=false
    elif [[ ! -z ${PRIVATE_KUBE_BINARY_DOWNLOAD_URL} ]]; then
        logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${PRIVATE_KUBE_BINARY_DOWNLOAD_URL} true
    fi

    if [[ ! -f "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" ]] || [[ ! -f "/usr/local/bin/kubelet-${KUBERNETES_VERSION}" ]]; then
        if [[ "$install_default_if_missing" == true ]]; then
            if [[ ! -z ${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER} ]]; then
                echo "Detect Bootstrap profile artifact is Cache, will use oras to pull artifact binary"
                registry_url="${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}/${K8S_REGISTRY_REPO}/kubernetes-node:v${KUBERNETES_VERSION}-linux-${CPU_ARCH}"
                K8S_DOWNLOADS_TEMP_DIR_FROM_REGISTRY="/tmp/kubernetes/downloads" 
                logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} $registry_url false ${K8S_DOWNLOADS_TEMP_DIR_FROM_REGISTRY}

            #TODO: remove the condition check on KUBE_BINARY_URL once RP change is released
            elif (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)) && [ -n "${KUBE_BINARY_URL}" ]; then
                logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${KUBE_BINARY_URL} false
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
    echo "pulling the image ${CONTAINER_IMAGE_URL} using ${CLI_TOOL}"
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        logs_to_events "AKS.CSE.imagepullctr.${CONTAINER_IMAGE_URL}" "retrycmd_if_failure 2 1 120 ctr --namespace k8s.io image pull $CONTAINER_IMAGE_URL" || (echo "timed out pulling image ${CONTAINER_IMAGE_URL} via ctr" && exit $ERR_CONTAINERD_CTR_IMG_PULL_TIMEOUT)
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        logs_to_events "AKS.CSE.imagepullcrictl.${CONTAINER_IMAGE_URL}" "retrycmd_if_failure 2 1 120 crictl pull $CONTAINER_IMAGE_URL" || (echo "timed out pulling image ${CONTAINER_IMAGE_URL} via crictl" && exit $ERR_CONTAINERD_CRICTL_IMG_PULL_TIMEOUT)
    else
        logs_to_events "AKS.CSE.imagepull.${CONTAINER_IMAGE_URL}" "retrycmd_if_failure 2 1 120 docker pull $CONTAINER_IMAGE_URL" || (echo "timed out pulling image ${CONTAINER_IMAGE_URL} via docker" && exit $ERR_DOCKER_IMG_PULL_TIMEOUT)
    fi
}

retagContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    RETAG_IMAGE_URL=$3
    echo "retagging from ${CONTAINER_IMAGE_URL} to ${RETAG_IMAGE_URL} using ${CLI_TOOL}"
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        ctr --namespace k8s.io image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        crictl image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    else
        docker image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    fi
}

retagMCRImagesForChina() {
    if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
        allMCRImages=($(ctr --namespace k8s.io images list | grep '^mcr.microsoft.com/' | awk '{print $1}'))
    else
        allMCRImages=($(docker images | grep '^mcr.microsoft.com/' | awk '{str = sprintf("%s:%s", $1, $2)} {print str}'))
    fi
    if [[ "${allMCRImages}" == "" ]]; then
        echo "failed to find mcr images for retag"
        return
    fi
    for mcrImage in ${allMCRImages[@]+"${allMCRImages[@]}"}; do
        retagMCRImage=$(echo ${mcrImage} | sed -e 's/^mcr.microsoft.com/mcr.azk8s.cn/g')
        if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
            retagContainerImage "ctr" ${mcrImage} ${retagMCRImage}
        else
            retagContainerImage "docker" ${mcrImage} ${retagMCRImage}
        fi
    done
}

removeContainerImage() {
    CLI_TOOL=$1
    CONTAINER_IMAGE_URL=$2
    if [[ "${CLI_TOOL}" == "docker" ]]; then
        docker image rm $CONTAINER_IMAGE_URL
    else
        crictl rmi $CONTAINER_IMAGE_URL
    fi
}

cleanUpImages() {
    local targetImage=$1
    export targetImage
    function cleanupImagesRun() {
        if [ "${NEEDS_CONTAINERD}" == "true" ]; then
            if [[ "${CLI_TOOL}" == "crictl" ]]; then
                images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
            else
                images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
            fi
        else
            images_to_delete=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
        fi
        local exit_code=$?
        if [[ $exit_code != 0 ]]; then
            exit $exit_code
        elif [[ "${images_to_delete}" != "" ]]; then
            echo "${images_to_delete}" | while read image; do
                if [ "${NEEDS_CONTAINERD}" == "true" ]; then
                    removeContainerImage ${CLI_TOOL} ${image}
                else
                    removeContainerImage "docker" ${image}
                fi
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
    if [[ "${TARGET_CLOUD}" != "AzureChinaCloud" ]]; then
        if [ "${NEEDS_CONTAINERD}" == "true" ]; then
            if [[ "${CLI_TOOL}" == "crictl" ]]; then
                images_to_delete=$(crictl images | awk '{print $1":"$2}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
            else
                images_to_delete=$(ctr --namespace k8s.io images list | awk '{print $1}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
            fi
        else
            images_to_delete=$(docker images --format '{{.Repository}}:{{.Tag}}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
        fi
        if [[ "${images_to_delete}" != "" ]]; then
            echo "${images_to_delete}" | while read image; do
                if [ "${NEEDS_CONTAINERD}" == "true" ]; then
                    removeContainerImage "ctr" ${image}
                else
                    removeContainerImage "docker" ${image}
                fi
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

overrideNetworkConfig() {
    CONFIG_FILEPATH="/etc/cloud/cloud.cfg.d/80_azure_net_config.cfg"
    touch ${CONFIG_FILEPATH}
    cat <<EOF >>${CONFIG_FILEPATH}
datasource:
    Azure:
        apply_network_config: false
EOF
}

#EOF
