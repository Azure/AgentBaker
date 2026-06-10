#!/bin/bash

# Tests for parts/linux/cloud-init/artifacts/init-aks-custom-cloud-mariner.sh
#
# Custom-cloud (Bleu, AGC, etc. — clouds that aren't Mooncake/Fairfax) provision-time
# script for Mariner/AzureLinux. We assert that after the repo-depot rewrite:
#   1. All baseurl entries point at the mocked depot, not packages.microsoft.com.
#   2. No third-party (e.g. developer.download.nvidia.com) URLs leak into the repo files.
#
# This is the regression class that caused IcM 725845756 (Bleu) — stale repo URLs on
# the booted VHD caused AzSecPack tdnf calls to hang on unreachable endpoints.

Describe 'init-aks-custom-cloud-mariner.sh'
    Include "./parts/linux/cloud-init/artifacts/init-aks-custom-cloud-mariner.sh"
    set +x

    setup() {
        TEST_DIR="$(mktemp -d)"
        export OS_RELEASE_FILE="${TEST_DIR}/os-release"
        export AZURE_CA_CERTS_DIR="${TEST_DIR}/AzureCACertificates"
        export CA_TRUST_ANCHORS_DIR="${TEST_DIR}/ca-trust-anchors"
        export YUM_REPOS_D_DIR="${TEST_DIR}/yum.repos.d"
        export CHRONY_CONF_FILE="${TEST_DIR}/chrony.conf"
        export WIRESERVER_ENDPOINT="http://wireserver.local"
        mkdir -p "${AZURE_CA_CERTS_DIR}" "${CA_TRUST_ANCHORS_DIR}" "${YUM_REPOS_D_DIR}"
    }
    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    Describe 'init_mariner_repo_depot'
        write_mariner_extras_repo() {
            cat > "${YUM_REPOS_D_DIR}/mariner-extras.repo" <<'EOF'
[mariner-official-extras]
name=CBL-Mariner Official Extras $releasever $basearch
baseurl=https://packages.microsoft.com/cbl-mariner/$releasever/prod/extras/$basearch
gpgkey=file:///etc/pki/rpm-gpg/MICROSOFT-RPM-GPG-KEY
gpgcheck=1
enabled=1
EOF
        }

        It 'rewrites Mariner repo baseurls to the depot endpoint and creates extended/nvidia/cloud-native repos'
            write_mariner_extras_repo
            When call init_mariner_repo_depot "https://repodepot.bleu.example.com"
            The status should be success
            The path "${YUM_REPOS_D_DIR}/mariner-extended.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/mariner-cloud-native.repo" should be exist
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should include "https://repodepot.bleu.example.com/mariner/packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should not include "https://packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should include "https://repodepot.bleu.example.com/mariner/packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should not include "https://packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should not include "developer.download.nvidia.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-cloud-native.repo" should include "https://repodepot.bleu.example.com/mariner/packages.microsoft.com"
        End

        It 'derives the nvidia repo from extras with case-preserving section/name updates'
            write_mariner_extras_repo
            When call init_mariner_repo_depot "https://repodepot.bleu.example.com"
            The status should be success
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should include "[mariner-official-nvidia]"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should include "name=CBL-Mariner Official Nvidia"
        End
    End

    Describe 'init_azurelinux_repo_depot'
        It 'creates all seven azurelinux repo files pointing at the depot'
            When call init_azurelinux_repo_depot "https://repodepot.bleu.example.com"
            The status should be success
            The path "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-cloud-native.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-amd.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-extended.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-ms-non-oss.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-ms-oss.repo" should be exist
        End

        It 'writes baseurls that point at the depot and never at packages.microsoft.com'
            When call init_azurelinux_repo_depot "https://repodepot.bleu.example.com"
            The status should be success
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should include "baseurl=https://repodepot.bleu.example.com/azurelinux/"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should not include "packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should include "baseurl=https://repodepot.bleu.example.com/azurelinux/"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should not include "developer.download.nvidia.com"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should not include "packages.microsoft.com"
        End

        It 'removes any pre-existing azurelinux*.repo files before creating new ones'
            echo "stale" > "${YUM_REPOS_D_DIR}/azurelinux-leftover.repo"
            When call init_azurelinux_repo_depot "https://repodepot.bleu.example.com"
            The status should be success
            The path "${YUM_REPOS_D_DIR}/azurelinux-leftover.repo" should not be exist
        End
    End

    Describe 'init_repo_depot dispatch'
        setup_mariner_os_release() {
            cat > "${OS_RELEASE_FILE}" <<EOF
NAME="Common Base Linux Mariner"
VERSION="2.0.20250701"
ID=mariner
VERSION_ID="2.0"
PRETTY_NAME="CBL-Mariner/Linux"
EOF
        }
        setup_azurelinux_os_release() {
            cat > "${OS_RELEASE_FILE}" <<EOF
NAME="Microsoft Azure Linux"
VERSION="3.0.20250702"
ID=azurelinux
VERSION_ID="3.0"
PRETTY_NAME="Microsoft Azure Linux 3.0"
EOF
        }
        write_mariner_extras_repo() {
            cat > "${YUM_REPOS_D_DIR}/mariner-extras.repo" <<'EOF'
[mariner-official-extras]
baseurl=https://packages.microsoft.com/cbl-mariner/$releasever/prod/extras/$basearch
EOF
        }

        Mock dnf_makecache
            true
        End

        It 'strips trailing /ubuntu from REPO_DEPOT_ENDPOINT and runs Mariner path'
            setup_mariner_os_release
            write_mariner_extras_repo
            detect_distro
            REPO_DEPOT_ENDPOINT="https://repodepot.bleu.example.com/ubuntu"
            When call init_repo_depot
            The status should be success
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should include "https://repodepot.bleu.example.com/mariner/packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should not include "https://packages.microsoft.com"
        End

        It 'runs AzureLinux path and writes only depot URLs'
            setup_azurelinux_os_release
            detect_distro
            REPO_DEPOT_ENDPOINT="https://repodepot.bleu.example.com/ubuntu"
            When call init_repo_depot
            The status should be success
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should include "https://repodepot.bleu.example.com/azurelinux/"
        End

        It 'warns when REPO_DEPOT_ENDPOINT is empty'
            setup_mariner_os_release
            detect_distro
            REPO_DEPOT_ENDPOINT=""
            When call init_repo_depot
            The status should be success
            The stderr should include "repo depot endpoint empty"
        End
    End
End
