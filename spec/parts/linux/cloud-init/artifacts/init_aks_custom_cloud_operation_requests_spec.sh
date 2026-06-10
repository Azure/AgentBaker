#!/bin/bash

# Tests for parts/linux/cloud-init/artifacts/init-aks-custom-cloud-operation-requests.sh
#
# This is the script that runs in Bleu (and other public-custom clouds that go through
# the operation-requests wireserver API) on Ubuntu / Flatcar / ACL nodes. It is the script
# whose repo-depot rewrite must succeed to prevent IcM 725845756.
#
# Repo-depot rewrite logic is identical to init-aks-custom-cloud.sh but the cert-fetch
# path is different (rate-limited operation-requests API). We assert the same invariants
# on the apt sources files.

Describe 'init-aks-custom-cloud-operation-requests.sh'
    Include "./parts/linux/cloud-init/artifacts/init-aks-custom-cloud-operation-requests.sh"
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
        export WIRESERVER_ENDPOINT="http://wireserver.local"
        mkdir -p "${AZURE_CA_CERTS_DIR}" "${CA_TRUST_ANCHORS_DIR}" "${SSL_CERTS_DIR}" \
                 "${LOCAL_SHARE_CA_CERTS_DIR}" "${APT_SOURCES_LIST_D_DIR}" \
                 "${APT_KEYRINGS_DIR}" "${APT_BACKUP_DIR}" "${SYSTEMD_SYSTEM_DIR}" \
                 "$(dirname "${APT_SOURCES_LIST}")"
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
VERSION_ID="22.04"
VERSION_CODENAME=jammy
EOF
    }

    Describe 'init_ubuntu_main_repo_depot'
        It 'writes a ubuntu.sources file pointing at the Bleu-shaped depot only'
            write_ubuntu_os_release
            When call init_ubuntu_main_repo_depot "https://repodepot.bleu.example.com"
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

        It 'writes microsoft-prod sources files pointing at the Bleu-shaped depot only'
            ubuntuRel=22.04
            repodepot_endpoint="https://repodepot.bleu.example.com"
            When call init_ubuntu_pmc_repo_depot "${repodepot_endpoint}"
            The status should be success
            The path "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should be exist
            The path "${APT_SOURCES_LIST_D_DIR}/microsoft-prod-testing.sources" should be exist
            The contents of file "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should include "URIs: https://repodepot.bleu.example.com/microsoft/ubuntu/22.04/prod"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should not include "https://packages.microsoft.com"
            The path "${APT_KEYRINGS_DIR}/microsoft.asc.gpg" should be exist
            The path "${APT_KEYRINGS_DIR}/msopentech.asc.gpg" should be exist
        End
    End
End
