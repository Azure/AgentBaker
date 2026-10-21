#!/bin/bash
# Installs the tools the ACL VHD->COSI convert job needs on its build agent:
# docker (privileged), az CLI, azcopy, make/jq/curl, and Go. The arm64 pool
# image is minimal Azure Linux and the amd64 pool image is Ubuntu, so every
# install is guarded by `command -v` and is a no-op when the tool is present.
#
# Runs from the root of the extracted AgentBaker source bundle, so the apt
# path can install the repository's pinned Go through hack/setup_golang.sh.
set -euo pipefail

AZCOPY_VERSION="10.22.1"
AZCOPY_AMD64_SHA256="7549424d56ab2d8b4033c84c2a9bb167dc2dcbb23998acd7fffb37bc1a71a267"
AZCOPY_ARM64_SHA256="4db9a4b48abc7775f1a5d6d928afc42361dcc57bbfcde23ac82e4c419a0dc8fc"

PKG=""

log_error() {
    echo "##vso[task.logissue type=error]$*"
}

# Sets PKG to apt, tdnf, or dnf, preferring apt.
detect_package_manager() {
    if command -v apt-get >/dev/null 2>&1; then
        PKG=apt
    elif command -v tdnf >/dev/null 2>&1; then
        PKG=tdnf
    elif command -v dnf >/dev/null 2>&1; then
        PKG=dnf
    else
        log_error "No supported package manager (apt/tdnf/dnf) on this agent"
        return 1
    fi
    echo "Package manager: ${PKG}"
}

require_passwordless_sudo() {
    if ! sudo -n true >/dev/null 2>&1; then
        log_error "This agent user does not have passwordless sudo; cannot install tools"
        return 1
    fi
}

pkg_install() {
    case "${PKG}" in
        apt)  sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "$@" ;;
        tdnf) sudo tdnf install -y "$@" ;;
        dnf)  sudo dnf install -y "$@" ;;
        *)    log_error "pkg_install called before detect_package_manager"; return 1 ;;
    esac
}

install_base_utilities() {
    if [ "${PKG}" = apt ]; then
        sudo DEBIAN_FRONTEND=noninteractive apt-get update -y
    fi
    # cosi-upload uses Microsoft's cgo-less OpenSSL backend (see packer.mk), so no
    # C toolchain or OpenSSL development packages are needed.
    pkg_install ca-certificates curl tar gnupg make jq openssl
    # git isn't used by the convert scripts, but agent-side cleanup checkouts need it.
    if ! command -v git >/dev/null 2>&1; then
        pkg_install git
    fi
}

# Docker must support --privileged for ImageCustomizer's loopback mounts.
install_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        case "${PKG}" in
            apt)      pkg_install docker.io ;;
            tdnf|dnf) pkg_install moby-engine moby-cli || pkg_install docker ;;
        esac
    fi
    sudo systemctl start docker 2>/dev/null || sudo service docker start 2>/dev/null || true
    # Give the agent user access to the docker socket for the duration of the job.
    sudo chown "$(id -u):$(id -g)" /var/run/docker.sock 2>/dev/null || true
    docker version
}

install_azure_cli() {
    if command -v az >/dev/null 2>&1; then
        az version
        return 0
    fi
    case "${PKG}" in
        apt)
            # Install from the Microsoft apt repository rather than piping a remote
            # script into `sudo bash`, so the install path is auditable.
            pkg_install ca-certificates curl gnupg apt-transport-https lsb-release
            sudo install -d -m 0755 /etc/apt/keyrings
            local tmp
            tmp="$(mktemp -d)"
            curl -fsSL https://packages.microsoft.com/keys/microsoft.asc -o "${tmp}/microsoft.asc"
            gpg --dearmor < "${tmp}/microsoft.asc" > "${tmp}/microsoft.gpg"
            sudo install -m 0644 "${tmp}/microsoft.gpg" /etc/apt/keyrings/microsoft.gpg
            rm -rf "${tmp}"
            echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/microsoft.gpg] https://packages.microsoft.com/repos/azure-cli/ $(lsb_release -cs) main" \
                | sudo tee /etc/apt/sources.list.d/azure-cli.list >/dev/null
            sudo apt-get update
            pkg_install azure-cli
            ;;
        tdnf|dnf)
            pkg_install azure-cli
            ;;
    esac
    az version
}

# Prints "<url> <sha256>" for the pinned azcopy release matching the machine.
azcopy_release_for_machine() {
    local machine="$1" arch sha
    case "${machine}" in
        aarch64|arm64) arch=arm64; sha="${AZCOPY_ARM64_SHA256}" ;;
        x86_64|amd64)  arch=amd64; sha="${AZCOPY_AMD64_SHA256}" ;;
        *) log_error "Unsupported machine architecture for azcopy: ${machine}"; return 1 ;;
    esac
    # The azureedge.net CDN was retired; azcopy releases are served from GitHub.
    echo "https://github.com/Azure/azure-storage-azcopy/releases/download/v${AZCOPY_VERSION}/azcopy_linux_${arch}_${AZCOPY_VERSION}.tar.gz ${sha}"
}

install_azcopy() {
    if ! command -v azcopy >/dev/null 2>&1; then
        local release url sha tmp azcopy_bin
        release="$(azcopy_release_for_machine "$(uname -m)")"
        url="${release% *}"
        sha="${release#* }"
        tmp="$(mktemp -d)"
        curl -fsSL "${url}" -o "${tmp}/azcopy.tgz"
        if ! echo "${sha}  ${tmp}/azcopy.tgz" | sha256sum --check --status; then
            log_error "azcopy archive checksum mismatch for ${url}"
            rm -rf "${tmp}"
            return 1
        fi
        tar -xzf "${tmp}/azcopy.tgz" -C "${tmp}"
        azcopy_bin="$(find "${tmp}" -maxdepth 2 -type f -name azcopy | head -n1)"
        if [ -z "${azcopy_bin}" ] || [ ! -x "${azcopy_bin}" ]; then
            log_error "azcopy archive did not contain an executable binary"
            rm -rf "${tmp}"
            return 1
        fi
        sudo install -m 0755 "${azcopy_bin}" /usr/local/bin/azcopy
        rm -rf "${tmp}"
    fi
    azcopy --version
}

# packer.mk's build-cosi-upload rejects Go toolchains without Microsoft system
# crypto. Ubuntu agents may ship a stock Go, so always install the repository's
# pinned Microsoft Go there; Azure Linux's golang package is already Microsoft Go.
install_go() {
    if [ "${PKG}" = apt ]; then
        ./hack/setup_golang.sh
    elif ! command -v go >/dev/null 2>&1; then
        pkg_install golang
    fi
    go version
}

main() {
    echo "==== Bootstrapping COSI convert tooling on $(uname -a) ===="
    detect_package_manager
    require_passwordless_sudo
    install_base_utilities
    install_docker
    install_azure_cli
    install_azcopy
    install_go
    echo "==== Tooling bootstrap complete ===="
}

${__SOURCED__:+return}

main "$@"
