#!/bin/bash
# ShellSpec invokes the functions and consumes the variables indirectly.
# shellcheck disable=SC2034,SC2329

Describe 'updateAptWithNvidiaPkg'
    setup_nvidia_repo() {
        repo_root=$(mktemp -d)
        mkdir -p "${repo_root}/etc/apt/sources.list.d" "${repo_root}/tmp"
        # Sandbox filesystem paths without changing repository-selection logic.
        sed -e "s|/tmp/|${repo_root}/tmp/|g" \
            -e "s|/etc/apt/|${repo_root}/etc/apt/|g" \
            ./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh > "${repo_root}/install.sh"
        # shellcheck source=/dev/null
        . "${repo_root}/install.sh"
    }

    cleanup_nvidia_repo() {
        rm -rf "${repo_root}"
    }

    BeforeEach setup_nvidia_repo
    AfterEach cleanup_nvidia_repo

    ERR_NVIDIA_GPG_KEY_DOWNLOAD_TIMEOUT=180
    ERR_APT_UPDATE_TIMEOUT=30
    getCPUArch() { echo "${test_arch}"; }
    retrycmd_curl_file() {
        echo "key URL: $5"
        printf 'armored key\n' > "$4"
    }
    gpg() {
        echo "gpg $*" >&2
        cat
    }
    apt_get_update() { echo "apt updated"; }

    configure_and_read_repo() {
        updateAptWithNvidiaPkg
        cat "${repo_root}/etc/apt/sources.list.d/nvidia.list"
    }

    Describe 'supported release and architecture matrix'
        Parameters
            22.04 amd64 ubuntu2204 x86_64 3bf863cc.pub
            22.04 arm64 ubuntu2204 sbsa 3bf863cc.pub
            24.04 amd64 ubuntu2404 x86_64 3bf863cc.pub
            24.04 arm64 ubuntu2404 sbsa 3bf863cc.pub
            26.04 amd64 ubuntu2404 x86_64 3bf863cc.pub
            26.04 arm64 ubuntu2604 sbsa 60DF8A40.pub
        End

        It "uses the matching signed repository and key for Ubuntu $1 $2"
            UBUNTU_RELEASE=$1
            test_arch=$2
            When call configure_and_read_repo
            The status should be success
            The output should include "key URL: https://developer.download.nvidia.com/compute/cuda/repos/$3/$4/$5"
            The output should include "deb [arch=$2 signed-by=${repo_root}/etc/apt/keyrings/nvidia.gpg] https://developer.download.nvidia.com/compute/cuda/repos/$3/$4 /"
            The output should include "apt updated"
            The stderr should equal "gpg --dearmor"
        End
    End

    UBUNTU_RELEASE=26.04
    test_arch=amd64

    It 'can reconfigure a repository without leaking readonly globals'
        configure_twice() {
            updateAptWithNvidiaPkg
            updateAptWithNvidiaPkg
        }
        When call configure_twice
        The status should be success
        The output should include "apt updated"
        The stderr should include "gpg --dearmor"
        The variable nvidia_sources_list_path should be undefined
        The variable nvidia_gpg_keyring_path should be undefined
    End

    It 'fails without refreshing APT when downloading the key fails'
        retrycmd_curl_file() { return 1; }
        When run updateAptWithNvidiaPkg
        The status should equal "${ERR_NVIDIA_GPG_KEY_DOWNLOAD_TIMEOUT}"
        The output should include "Using NVIDIA ubuntu2404 repository"
        The output should not include "apt updated"
    End

    It 'fails without refreshing APT when dearmoring the key fails'
        gpg() { return 1; }
        When run updateAptWithNvidiaPkg
        The status should equal "${ERR_NVIDIA_GPG_KEY_DOWNLOAD_TIMEOUT}"
        The output should include "key URL:"
        The output should not include "apt updated"
    End

    It 'propagates APT update failure instead of accepting an unusable repository'
        apt_get_update() { return 1; }
        When run updateAptWithNvidiaPkg
        The status should equal "${ERR_APT_UPDATE_TIMEOUT}"
        The output should include "key URL:"
        The stderr should equal "gpg --dearmor"
    End
End

Describe 'NVIDIA repository failure handling'
    Include "./parts/linux/cloud-init/artifacts/ubuntu/cse_install_ubuntu.sh"

    # Unsupported inputs must not write repository files or refresh APT.
    mkdir() { :; }
    apt_get_update() { echo "unexpected apt update"; }

    It 'leaves unsupported Ubuntu releases unchanged'
        UBUNTU_RELEASE=20.04
        getCPUArch() { echo amd64; }
        When call updateAptWithNvidiaPkg
        The status should be success
        The output should equal "NVIDIA repo setup is not supported on Ubuntu 20.04"
    End

    It 'leaves unsupported architectures unchanged'
        UBUNTU_RELEASE=26.04
        getCPUArch() { echo riscv64; }
        When call updateAptWithNvidiaPkg
        The status should be success
        The output should equal "Unknown CPU architecture: riscv64"
    End
End
