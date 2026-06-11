#!/bin/bash

# Tests for parts/linux/cloud-init/artifacts/init-aks-custom-cloud-operation-requests-mariner.sh
#
# This is the script that runs in Bleu (and other public-custom clouds that go through
# the operation-requests wireserver API) on Mariner / AzureLinux nodes. It is the script
# whose repo-depot rewrite must succeed to prevent IcM 725845756.
#
# Repo-depot rewrite logic is identical to init-aks-custom-cloud-mariner.sh but the
# cert-fetch path is different (rate-limited operation-requests API). We assert the same
# invariants on the repo files.

Describe 'init-aks-custom-cloud-operation-requests-mariner.sh'
    Include "./parts/linux/cloud-init/artifacts/init-aks-custom-cloud-operation-requests-mariner.sh"
    set +x

    setup() {
        TEST_DIR="$(mktemp -d)"
        export OS_RELEASE_FILE="${TEST_DIR}/os-release"
        export AZURE_CA_CERTS_DIR="${TEST_DIR}/AzureCACertificates"
        export CA_TRUST_ANCHORS_DIR="${TEST_DIR}/ca-trust-anchors"
        export YUM_REPOS_D_DIR="${TEST_DIR}/yum.repos.d"
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

        It 'rewrites Mariner repo baseurls to the depot endpoint with no PMC leakage'
            write_mariner_extras_repo
            When call init_mariner_repo_depot "https://repodepot.bleu.example.com"
            The output should be present
            The status should be success
            The path "${YUM_REPOS_D_DIR}/mariner-extended.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/mariner-cloud-native.repo" should be exist
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should include "https://repodepot.bleu.example.com/mariner/packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should not include "https://packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should not include "developer.download.nvidia.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-nvidia.repo" should not include "https://packages.microsoft.com"
        End
    End

    Describe 'init_azurelinux_repo_depot'
        It 'creates azurelinux repo files all pointing at the depot, no PMC, no NVIDIA URLs'
            When call init_azurelinux_repo_depot "https://repodepot.bleu.example.com"
            The output should be present
            The status should be success
            The path "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should be exist
            The path "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should be exist
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should include "baseurl=https://repodepot.bleu.example.com/azurelinux/"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should not include "packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should not include "developer.download.nvidia.com"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-nvidia.repo" should not include "packages.microsoft.com"
        End
    End

    Describe 'init_repo_depot dispatch'
        setup_mariner_os_release() {
            cat > "${OS_RELEASE_FILE}" <<EOF
NAME="Common Base Linux Mariner"
VERSION="2.0.20250701"
ID=mariner
VERSION_ID="2.0"
EOF
        }
        setup_azurelinux_os_release() {
            cat > "${OS_RELEASE_FILE}" <<EOF
NAME="Microsoft Azure Linux"
VERSION="3.0.20250702"
ID=azurelinux
VERSION_ID="3.0"
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

        It 'rewrites Mariner repos under a Bleu-shaped depot URL'
            setup_mariner_os_release
            write_mariner_extras_repo
            detect_distro
            REPO_DEPOT_ENDPOINT="https://repodepot.bleu.example.com/ubuntu"
            When call init_repo_depot
            The output should be present
            The status should be success
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should include "https://repodepot.bleu.example.com/mariner/packages.microsoft.com"
            The contents of file "${YUM_REPOS_D_DIR}/mariner-extras.repo" should not include "https://packages.microsoft.com"
        End

        It 'rewrites AzureLinux repos under a Bleu-shaped depot URL'
            setup_azurelinux_os_release
            detect_distro
            REPO_DEPOT_ENDPOINT="https://repodepot.bleu.example.com/ubuntu"
            When call init_repo_depot
            The output should be present
            The status should be success
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should include "https://repodepot.bleu.example.com/azurelinux/"
            The contents of file "${YUM_REPOS_D_DIR}/azurelinux-base.repo" should not include "packages.microsoft.com"
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
