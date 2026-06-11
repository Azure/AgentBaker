#!/bin/bash

# Tests for parts/linux/cloud-init/artifacts/init-aks-custom-cloud.sh
#
# Custom-cloud provision-time script for Ubuntu / Flatcar / ACL on USSecCloud / USNatCloud
# (Mooncake / Fairfax). Asserts that after the repo-depot rewrite:
#   1. Generated apt sources files point at the mocked depot, not packages.microsoft.com /
#      archive.ubuntu.com / security.ubuntu.com / etc.
#   2. No third-party (e.g. developer.download.nvidia.com) URLs leak in.
#   3. Old/baked sources.list[.d] entries are backed up rather than left in place.
# This is the regression class behind IcM 725845756: stale repo URLs on a booted VHD
# cause downstream tdnf/apt operations to hang.

Describe 'init-aks-custom-cloud.sh'
    Include "./parts/linux/cloud-init/artifacts/init-aks-custom-cloud.sh"
    set +x

    setup() {
        TEST_DIR="$(mktemp -d)"
        export OS_RELEASE_FILE="${TEST_DIR}/os-release"
        export AZURE_CA_CERTS_DIR="${TEST_DIR}/AzureCACertificates"
        export CA_TRUST_ANCHORS_DIR="${TEST_DIR}/ca-trust-anchors"
        export SSL_CERTS_DIR="${TEST_DIR}/ssl-certs"
        export LOCAL_SHARE_CA_CERTS_DIR="${TEST_DIR}/local-share-ca-certs"
        export OPENSSL_CERT_FILE="${TEST_DIR}/openssl-cert.pem"
        export APT_SOURCES_LIST="${TEST_DIR}/apt/sources.list"
        export APT_SOURCES_LIST_D_DIR="${TEST_DIR}/apt/sources.list.d"
        export APT_KEYRINGS_DIR="${TEST_DIR}/apt/keyrings"
        export APT_BACKUP_DIR="${TEST_DIR}/apt/backup"
        export SYSTEMD_SYSTEM_DIR="${TEST_DIR}/systemd"
        export CHRONY_CONF_FILE="${TEST_DIR}/chrony.conf"
        export WIRESERVER_ENDPOINT="http://wireserver.local"
        mkdir -p "${AZURE_CA_CERTS_DIR}" "${CA_TRUST_ANCHORS_DIR}" "${SSL_CERTS_DIR}" \
                 "${LOCAL_SHARE_CA_CERTS_DIR}" "${APT_SOURCES_LIST_D_DIR}" \
                 "${APT_KEYRINGS_DIR}" "${APT_BACKUP_DIR}" "${SYSTEMD_SYSTEM_DIR}" \
                 "$(dirname "${APT_SOURCES_LIST}")"
        # ca-certificates.crt is referenced when copying the bundle to OPENSSL_CERT_FILE
        echo "fake-bundle" > "${SSL_CERTS_DIR}/ca-certificates.crt"
    }
    cleanup() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    write_ubuntu_os_release() {
        cat > "${OS_RELEASE_FILE}" <<EOF
NAME="Ubuntu"
VERSION="22.04.5 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
VERSION_ID="22.04"
VERSION_CODENAME=jammy
EOF
    }

    Describe 'init_ubuntu_main_repo_depot'
        It 'writes a ubuntu.sources file pointing at the depot, with no upstream URLs'
            write_ubuntu_os_release
            When call init_ubuntu_main_repo_depot "https://repodepot.bleu.example.com"
            The output should be present
            The status should be success
            The path "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should be exist
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should include "URIs: https://repodepot.bleu.example.com/ubuntu"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should include "jammy jammy-updates jammy-backports jammy-security"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should not include "archive.ubuntu.com"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should not include "security.ubuntu.com"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should not include "packages.microsoft.com"
        End

        It 'backs up the existing sources.list and sources.list.d files'
            write_ubuntu_os_release
            echo "deb http://archive.ubuntu.com/ubuntu jammy main" > "${APT_SOURCES_LIST}"
            echo "deb http://packages.microsoft.com/repos/azure-cli/ jammy main" > "${APT_SOURCES_LIST_D_DIR}/azure-cli.list"
            When call init_ubuntu_main_repo_depot "https://repodepot.bleu.example.com"
            The output should be present
            The status should be success
            The path "${APT_BACKUP_DIR}/sources.list" should be exist
            The path "${APT_BACKUP_DIR}/azure-cli.list" should be exist
            The path "${APT_SOURCES_LIST}" should not be exist
            The path "${APT_SOURCES_LIST_D_DIR}/azure-cli.list" should not be exist
        End
    End

    Describe 'init_ubuntu_pmc_repo_depot'
        Mock curl
            printf 'HTTP/1.1 200 OK\n'
        End
        Mock wget
            echo 'fake-key-data'
        End
        Mock gpg
            cat
        End
        Mock lsb_release
            echo "Codename:	jammy"
        End

        It 'writes microsoft-prod sources files pointing at the depot only'
            ubuntuRel=22.04
            repodepot_endpoint="https://repodepot.bleu.example.com"
            When call init_ubuntu_pmc_repo_depot "${repodepot_endpoint}"
            The output should be present
            The status should be success
            The path "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should be exist
            The path "${APT_SOURCES_LIST_D_DIR}/microsoft-prod-testing.sources" should be exist
            The contents of file "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should include "URIs: https://repodepot.bleu.example.com/microsoft/ubuntu/22.04/prod"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should not include "https://packages.microsoft.com"
            The path "${APT_KEYRINGS_DIR}/microsoft.asc.gpg" should be exist
            The path "${APT_KEYRINGS_DIR}/msopentech.asc.gpg" should be exist
        End
    End

    Describe 'check_url'
        Mock curl
            printf 'HTTP/1.1 200 OK\n'
        End

        It 'passes for a 200 response'
            When call check_url "https://repodepot.bleu.example.com/ubuntu/dists/jammy/Release"
            The status should be success
            The stdout should include "Checking url"
        End
    End
End
