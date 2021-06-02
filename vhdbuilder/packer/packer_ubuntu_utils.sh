#!/bin/bash

downloadDebPkgToFile() {
    PKG_NAME=$1
    PKG_VERSION=$2
    PKG_DIRECTORY=$3
    mkdir -p $PKG_DIRECTORY
    # shellcheck disable=SC2164
    pushd ${PKG_DIRECTORY}
    retrycmd_if_failure 10 5 600 apt-get download ${PKG_NAME}=${PKG_VERSION}*
    # shellcheck disable=SC2164
    popd
}

downloadContainerdFromBlob() {
    CONTAINERD_VERSION=$1
    # currently upstream maintains the package on a storage endpoint rather than an actual apt repo
    CONTAINERD_DOWNLOAD_URL="https://mobyartifacts.azureedge.net/moby/moby-containerd/${CONTAINERD_VERSION}+azure/bionic/linux_amd64/moby-containerd_${CONTAINERD_VERSION/-/\~}+azure-1_amd64.deb"
    mkdir -p $CONTAINERD_DOWNLOADS_DIR
    CONTAINERD_DEB_TMP=${CONTAINERD_DOWNLOAD_URL##*/}
    retrycmd_curl_file 120 5 60 "$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}" ${CONTAINERD_DOWNLOAD_URL} || exit $ERR_CONTAINERD_DOWNLOAD_TIMEOUT
    CONTAINERD_DEB_FILE="$CONTAINERD_DOWNLOADS_DIR/${CONTAINERD_DEB_TMP}"
}