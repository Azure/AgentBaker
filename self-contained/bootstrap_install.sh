#!/bin/bash

CC_SERVICE_IN_TMP=/opt/azure/containers/cc-proxy.service.in
CC_SOCKET_IN_TMP=/opt/azure/containers/cc-proxy.socket.in
CNI_CONFIG_DIR="/etc/cni/net.d"
CNI_BIN_DIR="/opt/cni/bin"
CNI_DOWNLOADS_DIR="/opt/cni/downloads"
CRICTL_DOWNLOAD_DIR="/opt/crictl/downloads"
CRICTL_BIN_DIR="/usr/local/bin"
CONTAINERD_DOWNLOADS_DIR="/opt/containerd/downloads"
RUNC_DOWNLOADS_DIR="/opt/runc/downloads"
K8S_DOWNLOADS_DIR="/opt/kubernetes/downloads"
UBUNTU_RELEASE=$(lsb_release -r -s)
OS=$(sort -r /etc/*-release | gawk 'match($0, /^(ID_LIKE=(coreos)|ID=(.*))$/, a) { print toupper(a[2] a[3]); exit }')
TELEPORTD_PLUGIN_DOWNLOAD_DIR="/opt/teleportd/downloads"
TELEPORTD_PLUGIN_BIN_DIR="/usr/local/bin"
CONTAINERD_WASM_VERSIONS="v0.3.0 v0.5.1 v0.8.0"
SPIN_KUBE_VERSIONS="v0.15.1"
MANIFEST_FILEPATH="/opt/azure/manifest.json"
MAN_DB_AUTO_UPDATE_FLAG_FILEPATH="/var/lib/man-db/auto-update"
CURL_OUTPUT=/tmp/curl_verbose.out

removeManDbAutoUpdateFlagFile() {
    rm -f $MAN_DB_AUTO_UPDATE_FLAG_FILEPATH
}

createManDbAutoUpdateFlagFile() {
    touch $MAN_DB_AUTO_UPDATE_FLAG_FILEPATH
}

cleanupContainerdDlFiles() {
    rm -rf $CONTAINERD_DOWNLOADS_DIR
}

installContainerRuntime() {
    if [ "${NEEDS_CONTAINERD}" == "true" ]; then
        echo "in installContainerRuntime - KUBERNETES_VERSION = ${KUBERNETES_VERSION}"
        local containerd_version
        if [ -f "$MANIFEST_FILEPATH" ]; then
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
        echo "in installContainerRuntime - CONTAINERD_VERION = ${containerd_patch_version}"
    else
        installMoby
    fi
}

installNetworkPlugin() {
    if [[ "${NETWORK_PLUGIN}" = "azure" ]]; then
        installAzureCNI
    fi
    installCNI  #reference plugins. Mostly for kubenet but loop back used by contaierd until containerd 2
    rm -rf $CNI_DOWNLOADS_DIR &
}

wasmFilesExist() {
    local containerd_wasm_filepath=${1}
    local shim_version=${2}
    local version_suffix=${3}
    local shims_to_download=("${@:4}") # Capture all arguments starting from the fourth indx

    local binary_version="$(echo "${shim_version}" | tr . -)"
    for shim in "${shims_to_download[@]}"; do
        if [ ! -f "${containerd_wasm_filepath}/containerd-shim-${shim}-${binary_version}-${version_suffix}" ]; then
            return 1 # file is missing
        fi
    done
    echo "all wasm files exist for ${containerd_wasm_filepath}/containerd-shim-*-${binary_version}-${version_suffix}"
    return 0 
}

# Install, download, update wasm must all be run from the same function call
# in order to ensure WASMSHIMPIDS persists correctly since in bash a new
# function call from install-dependnecies will create a new shell process.
installContainerdWasmShims(){
    local download_location=${1}
    PACKAGE_DOWNLOAD_URL=${2}
    local package_versions=("${@:2}") # Capture all arguments starting from the second indx
    
    for version in "${package_versions[@]}"; do
        local shims_to_download=("spin" "slight")
        if [[ "$version" == "0.8.0" ]]; then
            shims_to_download+=("wws")
        fi
        containerd_wasm_url=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadContainerdWasmShims $download_location $containerd_wasm_url "v$version" "${shims_to_download[@]}" # adding v to version for simplicity
    done
    # wait for file downloads to complete before updating file permissions
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
    local shims_to_download=("${@:4}") # Capture all arguments starting from the fourth indx

    local binary_version="$(echo "${shim_version}" | tr . -)" # replaces . with - == 1.2.3 -> 1-2-3

    if wasmFilesExist "$containerd_wasm_filepath" "$shim_version" "-v1" "${shims_to_download[@]}"; then
        echo "containerd-wasm-shims already exists in $containerd_wasm_filepath, will not be downloading."
        return
    fi

    # Oras download for WASM for Network Isolated Clusters
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
    local shims_to_download=("${@:3}") # Capture all arguments starting from the third indx

    local binary_version="$(echo "${shim_version}" | tr . -)"

    for shim in "${shims_to_download[@]}"; do
        chmod 755 "$containerd_wasm_filepath/containerd-shim-${shim}-${binary_version}-v1"
    done
}

installSpinKube(){
    local download_location=${1}
    PACKAGE_DOWNLOAD_URL=${2}
    local package_versions=("${@:2}") # Capture all arguments starting from the second indx

    for version in "${package_versions[@]}"; do
        containerd_spinkube_url=$(evalPackageDownloadURL ${PACKAGE_DOWNLOAD_URL})
        downloadSpinKube $download_location $containerd_spinkube_url "v$version" # adding v to version for simplicity
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
    local shims_to_download=("${@:4}") # Capture all arguments starting from the fourth indx

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

downloadAzureCNI() {
    mkdir -p $CNI_DOWNLOADS_DIR
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    retrycmd_get_tarball 120 5 "$CNI_DOWNLOADS_DIR/${CNI_TGZ_TMP}" ${VNET_CNI_PLUGINS_URL} || exit $ERR_CNI_DOWNLOAD_TIMEOUT
}

downloadCrictl() {
    CRICTL_VERSION=$1
    CPU_ARCH=$(getCPUArch) #amd64 or arm64
    mkdir -p $CRICTL_DOWNLOAD_DIR
    CRICTL_DOWNLOAD_URL="https://acs-mirror.azureedge.net/cri-tools/v${CRICTL_VERSION}/binaries/crictl-v${CRICTL_VERSION}-linux-${CPU_ARCH}.tar.gz"
    CRICTL_TGZ_TEMP=${CRICTL_DOWNLOAD_URL##*/}
    retrycmd_curl_file 10 5 60 "$CRICTL_DOWNLOAD_DIR/${CRICTL_TGZ_TEMP}" ${CRICTL_DOWNLOAD_URL}
}

installCrictl() {
    CPU_ARCH=$(getCPUArch) #amd64 or arm64
    currentVersion=$(crictl --version 2>/dev/null | sed 's/crictl version //g')
    if [[ "${currentVersion}" != "" ]]; then
        echo "version ${currentVersion} of crictl already installed. skipping installCrictl of target version ${KUBERNETES_VERSION%.*}.0"
    else
        # this is only called during cse. VHDs should have crictl binaries pre-cached so no need to download.
        # if the vhd does not have crictl pre-baked, return early
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
        # no arm64 teleport binaries according to owner
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
        # no arm64 teleport binaries according to owner
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

# Reference CNI plugins is used by kubenet and the loopback plugin used by containerd 1.0 (dependency gone in 2.0)
# The version used to be deteremined by RP/toggle but are now just hadcoded in vhd as they rarely change and require a node image upgrade anyways
# Latest VHD should have the untar, older should have the tgz. And who knows will have neither. 
installCNI() {
    #always just use what is listed in components.json so we don't have to sync.
    cniPackage=$(jq ".Packages" "$COMPONENTS_FILEPATH" | jq ".[] | select(.name == \"cni-plugins\")") || exit $ERR_CNI_VERSION_INVALID
    
    #CNI doesn't really care about this but wanted to reuse updatePackageVersions which requires it.
    os=${UBUNTU_OS_NAME} 
    if [[ -z "$UBUNTU_RELEASE" ]]; then
        os=${OS}
        os_version="current"
    fi
    os_version="${UBUNTU_RELEASE}"
    PACKAGE_VERSIONS=()
    updatePackageVersions "${cniPackage}" "${os}" "${os_version}"
    
    #should change to ne
    if [[ ${#PACKAGE_VERSIONS[@]} -gt 1 ]]; then
        echo "WARNING: containerd package versions array has more than one element. Installing the last element in the array."
        exit $ERR_CONTAINERD_VERSION_INVALID
    fi
    packageVersion=${PACKAGE_VERSIONS[0]}

    # Is there a ${arch} variable I can use instead of the iff
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
    CNI_TGZ_TMP=${VNET_CNI_PLUGINS_URL##*/} # Use bash builtin ## to remove all chars ("*") up to the final "/"
    CNI_DIR_TMP=${CNI_TGZ_TMP%.tgz}         # Use bash builtin % to remove the .tgz to look for a folder rather than tgz

    # We want to use the untar azurecni reference first. And if that doesn't exist on the vhd does the tgz?
    # And if tgz is already on the vhd then just untar into CNI_BIN_DIR
    # Latest VHD should have the untar, older should have the tgz. And who knows will have neither.
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
    K8S_VERSION=$1
    KUBE_BINARY_URL=$2

    mkdir -p ${K8S_DOWNLOADS_DIR}
    K8S_TGZ_TMP=${KUBE_BINARY_URL##*/}
    retrycmd_get_tarball 120 5 "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}" ${KUBE_BINARY_URL} || exit $ERR_K8S_DOWNLOAD_TIMEOUT
    tar --transform="s|.*|&-${K8S_VERSION}|" --show-transformed-names -xzvf "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}" \
        --strip-components=3 -C /usr/local/bin kubernetes/node/bin/kubelet kubernetes/node/bin/kubectl
    rm -f "$K8S_DOWNLOADS_DIR/${K8S_TGZ_TMP}"
}

installKubeletKubectlAndKubeProxy() {

    CUSTOM_KUBE_BINARY_DOWNLOAD_URL="${CUSTOM_KUBE_BINARY_URL:=}"
    if [[ ! -z ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL} ]]; then
        # remove the kubelet binaries to make sure the only binary left is from the CUSTOM_KUBE_BINARY_DOWNLOAD_URL
        rm -rf /usr/local/bin/kubelet-* /usr/local/bin/kubectl-*

        # NOTE(mainred): we expect kubelet binary to be under `kubernetes/node/bin`. This suits the current setting of
        # kube binaries used by AKS and Kubernetes upstream.
        # TODO(mainred): let's see if necessary to auto-detect the path of kubelet
        logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${CUSTOM_KUBE_BINARY_DOWNLOAD_URL}

    else
        if [[ ! -f "/usr/local/bin/kubectl-${KUBERNETES_VERSION}" ]]; then
            #TODO: remove the condition check on KUBE_BINARY_URL once RP change is released
            if (($(echo ${KUBERNETES_VERSION} | cut -d"." -f2) >= 17)) && [ -n "${KUBE_BINARY_URL}" ]; then
                logs_to_events "AKS.CSE.installKubeletKubectlAndKubeProxy.extractKubeBinaries" extractKubeBinaries ${KUBERNETES_VERSION} ${KUBE_BINARY_URL}
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
    echo "retaging from ${CONTAINER_IMAGE_URL} to ${RETAG_IMAGE_URL} using ${CLI_TOOL}"
    if [[ ${CLI_TOOL} == "ctr" ]]; then
        ctr --namespace k8s.io image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    elif [[ ${CLI_TOOL} == "crictl" ]]; then
        crictl image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    else
        docker image tag $CONTAINER_IMAGE_URL $RETAG_IMAGE_URL
    fi
}

retagMCRImagesForChina() {
    # retag all the mcr for mooncake
    if [[ "${CONTAINER_RUNTIME}" == "containerd" ]]; then
        # shellcheck disable=SC2016
        allMCRImages=($(ctr --namespace k8s.io images list | grep '^mcr.microsoft.com/' | awk '{print $1}'))
    else
        # shellcheck disable=SC2016
        allMCRImages=($(docker images | grep '^mcr.microsoft.com/' | awk '{str = sprintf("%s:%s", $1, $2)} {print str}'))
    fi
    if [[ "${allMCRImages}" == "" ]]; then
        echo "failed to find mcr images for retag"
        return
    fi
    for mcrImage in ${allMCRImages[@]+"${allMCRImages[@]}"}; do
        # in mooncake, the mcr endpoint is: mcr.azk8s.cn
        # shellcheck disable=SC2001
        retagMCRImage=$(echo ${mcrImage} | sed -e 's/^mcr.microsoft.com/mcr.azk8s.cn/g')
        # can't use CLI_TOOL because crictl doesn't support retagging.
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
        # crictl should always be present
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
            images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep -vE "${KUBERNETES_VERSION}$|${KUBERNETES_VERSION}.[0-9]+$|${KUBERNETES_VERSION}-|${KUBERNETES_VERSION}_" | grep ${targetImage} | tr ' ' '\n')
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
            images_to_delete=$(docker images --format '{{OpenBraces}}.Repository{{CloseBraces}}:{{OpenBraces}}.Tag{{CloseBraces}}' | grep '^mcr.azk8s.cn/' | tr ' ' '\n')
        fi
        if [[ "${images_to_delete}" != "" ]]; then
            echo "${images_to_delete}" | while read image; do
                if [ "${NEEDS_CONTAINERD}" == "true" ]; then
                    # always use ctr, even if crictl is installed.
                    # crictl will remove *ALL* references to a given imageID (SHA), which removes too much.
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
