#!/bin/bash

set -uxo pipefail

DEFAULT_VERSION="v0.1.0-alpha.2"

ERR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT_TIMEOUT=169 

DOWNLOAD_URL="${1-:https://k8sreleases.blob.core.windows.net/aks-tls-bootstrap-client/${DEFAULT_VERSION}/linux/amd64/tls-bootstrap-client}"
DOWNLOAD_DIR="${2-:/opt/azure/tlsbootstrap}"

retrycmd_if_failure() {
    retries=$1; wait_sleep=$2; timeout=$3; shift && shift && shift
    for i in $(seq 1 $retries); do
        timeout $timeout "${@}" && break || \
        if [ $i -eq $retries ]; then
            echo Executed \"$@\" $i times;
            return 1
        else
            sleep $wait_sleep
        fi
    done
    echo Executed \"$@\" $i times;
}

downloadClient() {
    BINARY_PATH="${DOWNLOAD_DIR}/tls-bootstrap-client"

    [ -f "$BINARY_PATH" ] && exit 0

    retrycmd_if_failure 30 5 60 curl -fSL -o "$BINARY_PATH" "$DOWNLOAD_URL" || exit $ERR_DOWNLOAD_SECURE_TLS_BOOTSTRAP_CLIENT_TIMEOUT
    chown -R root:root "$DOWNLOAD_DIR"
    chmod -R 755 "$DOWNLOAD_DIR"
}

downloadClient "$@"

#EOF