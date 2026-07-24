#!/bin/bash

stub() {
    echo "${FUNCNAME[1]} stub"
}

downloadSysextFromVersion() {
    local seName=$1
    local seURL=$2
    local downloadDir=${3:-"/opt/${seName}/downloads"}

    if ! retrycmd_if_failure 120 5 60 oras pull --registry-config "${ORAS_REGISTRY_CONFIG_FILE}" --output "${downloadDir}" "${seURL}"; then
        echo "Failed to download ${seName} system extension from ${seURL}"
        return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
    fi

    echo "Succeeded to download ${seName} system extension from ${seURL}"
}

matchLocalSysext() {
    local seName=$1 desiredVer=$2 seArch=$3
    printf "%s\n" "/opt/${seName}/downloads/${seName}-v${desiredVer}"[.~-]*"-${seArch}.raw" | sort -V | tail -n1
}

matchRemoteSysext() {
    local seURL=$1 desiredVer=$2 seArch=$3
    if [ -n "${BOOTSTRAP_PROFILE_CONTAINER_REGISTRY_SERVER}" ]; then
        # For network isolated cluster, acr cache rule does not support oras repo tags.
        # return fixed renovateTag '-1' as workaround
        echo "v${desiredVer}-1-azlinux3-${seArch}"
        return 0
    fi
    retrycmd_silent 120 5 20 oras repo tags --registry-config "${ORAS_REGISTRY_CONFIG_FILE}" "${seURL}" | grep -Ex "v${desiredVer//./\\.}[.~-].*-azlinux3-${seArch}" | sort -V | tail -n1
    test ${PIPESTATUS[0]} -eq 0
}

mergeSysexts() {
    local seArch
    seArch=$(getSystemdArch)

    while [ "${1-}" ]; do
        local seName=$1 seURL=$2 desiredVer=$3 seMatch

        seMatch=$(matchLocalSysext "${seName}" "${desiredVer}" "${seArch}")
        if ! test -f "${seMatch}"; then
            echo "Failed to find valid ${seName} system extension for ${desiredVer} locally"

            seMatch=$(matchRemoteSysext "${seURL}" "${desiredVer}" "${seArch}")
            if [ -z "${seMatch}" ]; then
                echo "Failed to find valid ${seName} system extension for ${desiredVer} remotely"
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi

            if ! downloadSysextFromVersion "${seName}" "${seURL}:${seMatch}"; then
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi

            seMatch=$(matchLocalSysext "${seName}" "${desiredVer}" "${seArch}")
            if ! test -f "${seMatch}"; then
                echo "Failed to find valid ${seName} system extension for ${desiredVer} after downloading"
                return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
            fi
        fi

        ln -snf "${seMatch}" "/etc/extensions/${seName}.raw"
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
    if mergeSysexts kubelet "${2:-mcr.microsoft.com}"/oss/v2/kubernetes/kubelet-sysext "$1" \
                    kubectl "${2:-mcr.microsoft.com}"/oss/v2/kubernetes/kubectl-sysext "$1"; then
        ln -snf /usr/bin/{kubelet,kubectl} /opt/bin/
        # Clean up stale cached binaries that were not used
        rm -f /opt/bin/kubelet-* /opt/bin/kubectl-* &
    else
        installKubeletKubectlFromURL
    fi
}

installKubeletKubectlFromBootstrapProfileRegistry() {
    installKubeletKubectlFromPkg "$2" "$1"
}

installCredentialProviderFromPkg() {
    if mergeSysexts azure-acr-credential-provider "${2:-mcr.microsoft.com}"/oss/v2/kubernetes/azure-acr-credential-provider-sysext "$1"; then
        mkdir -p "${CREDENTIAL_PROVIDER_BIN_DIR}"
        chown -R root:root "${CREDENTIAL_PROVIDER_BIN_DIR}"
        ln -snf /usr/bin/azure-acr-credential-provider "$CREDENTIAL_PROVIDER_BIN_DIR/acr-credential-provider"
    else
        installCredentialProviderFromUrl
    fi
}

installCredentialProviderPackageFromBootstrapProfileRegistry() {
    installCredentialProviderFromPkg "$2" "$1"
}

# Only called at build-time, unlike kubelet or credential provider installation.
# Flatcar's matchLocalSysext glob (name-v${ver}[.~-]*-${arch}.raw) cannot match the
# bootstrap client's filename, which has only a single '-' between the version and arch
# (e.g. aks-secure-tls-bootstrap-client-v1.1.3-2-azlinux3-x86-64.raw). The download has
# already been completed by install-dependencies.sh into a known location with a known
# filename, so activate the sysext directly instead of going through mergeSysexts.
installSecureTLSBootstrapClientSysext() {
    local version=$1
    local seName=aks-secure-tls-bootstrap-client
    local seArch
    seArch=$(getSystemdArch)
    # Normalize to ensure a leading 'v' to match the artifact filename produced by oras pull.
    version="v${version#v}"
    local seFile="/opt/${seName}/downloads/${seName}-${version}-${seArch}.raw"
    if ! test -f "${seFile}"; then
        echo "Failed to find downloaded ${seName} sysext at ${seFile}"
        return "${ERR_ORAS_PULL_SYSEXT_FAIL}"
    fi
    ln -snf "${seFile}" "/etc/extensions/${seName}.raw"
    systemd-sysext --no-reload refresh
    ln -snf "/usr/bin/${seName}" "/opt/bin/${seName}"
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
