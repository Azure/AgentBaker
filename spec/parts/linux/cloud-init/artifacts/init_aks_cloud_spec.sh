#!/bin/bash

# Tests for parts/linux/cloud-init/artifacts/init-aks-cloud.sh
#
# Covers two areas:
# 1. Structural wiring tests (grep-based) for the ca-refresh mode added by #8096.
# 2. Functional tests that source the script and exercise:
#    - repo-depot helpers (init_ubuntu_main_repo_depot, init_ubuntu_pmc_repo_depot,
#      init_mariner_repo_depot, init_azurelinux_repo_depot, check_url)
#    - cloud mode selection helper (determine_cert_endpoint_mode)

Describe 'init-aks-cloud.sh refresh mode wiring'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-cloud.sh'

    It 'defaults the action to init'
        When run grep -Fq 'if [ "${1:-init}" = "ca-refresh" ]; then' "$script_path"
        The status should eq 0
    End

    It 'uses arg2 as location fallback when invoked as ca-refresh'
        When run grep -Eq '^refresh_location="\$\{2:-\$\{LOCATION\}\}"$' "$script_path"
        The status should eq 0
    End

    It 'derives cert endpoint mode from the selected location'
        When run grep -Eq '^[[:space:]]+cert_endpoint_mode=\$\(determine_cert_endpoint_mode "\$refresh_location"\)$' "$script_path"
        The status should eq 0
    End

    run_entry() (
        LOCATION=ussec-fallback
        refresh_certs() { echo "install:$1"; return "${INSTALL_STATUS:-0}"; }
        refresh_certs_and_containerd() { echo "scheduled:$1"; return "${INSTALL_STATUS:-0}"; }
        # Run the actual dispatch block, stopping before host schedule writes.
        # shellcheck disable=SC1090
        . <(awk '/^refresh_location=/{copy=1}
            /^if \[ "\$IS_UBUNTU" -eq 1 \] \|\|/{exit}
            copy' "$script_path") "$@"
        echo schedule-and-init
    )

    It 'runs the coordinator only for scheduled refresh and exits before init'
        When run run_entry ca-refresh eastus
        The status should be success
        The output should eq scheduled:eastus
    End

    It 'does not restart or jitter during initial provisioning'
        When run run_entry init eastus
        The status should be success
        The line 1 should eq install:eastus
        The line 2 should eq schedule-and-init
    End

    It 'uses the live provisioning location when no explicit location is supplied'
        When run run_entry
        The status should be success
        The line 1 should eq install:ussec-fallback
        The line 2 should eq schedule-and-init
    End

    It 'does not schedule or continue init after opt-out'
        INSTALL_STATUS=3
        When run run_entry init eastus
        The status should be success
        The output should eq install:eastus
    End

    It 'propagates coordinator failure'
        INSTALL_STATUS=7
        When run run_entry ca-refresh eastus
        The status should eq 7
        The output should eq scheduled:eastus
    End

    It 'does not double the coordinator jitter in systemd'
        When run grep -F 'RandomizedDelaySec=' "$script_path"
        The status should be success
        The output should eq RandomizedDelaySec=0
    End

    It 'passes LOCATION directly into cron refresh command'
        When run grep -Eq 'ca-refresh \\"\$LOCATION\\"' "$script_path"
        The status should eq 0
    End

    It 'passes LOCATION directly into systemd refresh command'
        When run grep -Eq '^ExecStart=\$script_path ca-refresh \$LOCATION$' "$script_path"
        The status should eq 0
    End
End

Describe 'init-aks-cloud.sh functional tests'
    setup() {
        TEST_DIR="$(mktemp -d)"
        export OS_RELEASE_FILE="${TEST_DIR}/os-release"
        export APT_SOURCES_LIST="${TEST_DIR}/apt/sources.list"
        export APT_SOURCES_LIST_D_DIR="${TEST_DIR}/apt/sources.list.d"
        export APT_KEYRINGS_DIR="${TEST_DIR}/apt/keyrings"
        export APT_BACKUP_DIR="${TEST_DIR}/apt/backup"
        export SSL_CERTS_DIR="${TEST_DIR}/ssl-certs"
        export SSL_CERT_TARGET="${TEST_DIR}/ssl-cert-target.pem"
        mkdir -p "${APT_SOURCES_LIST_D_DIR}" "${APT_KEYRINGS_DIR}" \
                 "${APT_BACKUP_DIR}" "${SSL_CERTS_DIR}" \
                 "$(dirname "${APT_SOURCES_LIST}")"
        # ca-certificates.crt is referenced when copying the bundle
        echo "fake-bundle" > "${SSL_CERTS_DIR}/ca-certificates.crt"
        # shellcheck disable=SC1090
        __SOURCED__=1 . "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
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
            When call init_ubuntu_main_repo_depot "https://repodepot.example.com"
            The output should be present
            The status should be success
            The path "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should be exist
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should include "URIs: https://repodepot.example.com/ubuntu"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should include "jammy jammy-updates jammy-backports jammy-security"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should not include "archive.ubuntu.com"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should not include "security.ubuntu.com"
            The contents of file "${APT_SOURCES_LIST_D_DIR}/ubuntu.sources" should not include "packages.microsoft.com"
        End

        It 'backs up the existing sources.list and sources.list.d files'
            write_ubuntu_os_release
            echo "deb http://archive.ubuntu.com/ubuntu jammy main" > "${APT_SOURCES_LIST}"
            echo "deb http://packages.microsoft.com/repos/azure-cli/ jammy main" > "${APT_SOURCES_LIST_D_DIR}/azure-cli.list"
            When call init_ubuntu_main_repo_depot "https://repodepot.example.com"
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
            repodepot_endpoint="https://repodepot.example.com"
            When call init_ubuntu_pmc_repo_depot "${repodepot_endpoint}"
            The output should be present
            The status should be success
            The path "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should be exist
            The path "${APT_SOURCES_LIST_D_DIR}/microsoft-prod-testing.sources" should be exist
            The contents of file "${APT_SOURCES_LIST_D_DIR}/microsoft-prod.sources" should include "URIs: https://repodepot.example.com/microsoft/ubuntu/22.04/prod"
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
            When call check_url "https://repodepot.example.com/ubuntu/dists/jammy/Release"
            The status should be success
            The stdout should include "Checking url"
        End
    End

    # Cloud coverage:
    #   legacy : ussec (US Secret), usnat (US Nationwide), incl. suffixed regions
    #   rcv1p  : fairfax/usgovvirginia (US Gov), mooncake/chinaeast2 (China),
    #            bleu/francesouth (France), and empty location
    Describe 'determine_cert_endpoint_mode'
        It 'returns legacy for ussec region'
            When call determine_cert_endpoint_mode "ussec"
            The output should eq "legacy"
        End

        It 'returns legacy for usnat region'
            When call determine_cert_endpoint_mode "usnat"
            The output should eq "legacy"
        End

        It 'returns legacy for ussec with suffix (e.g. ussecwest)'
            When call determine_cert_endpoint_mode "USSecWest"
            The output should eq "legacy"
        End

        It 'returns legacy for usnat with suffix (e.g. usnateast)'
            When call determine_cert_endpoint_mode "USNatEast"
            The output should eq "legacy"
        End

        It 'returns rcv1p for fairfax (USGov)'
            When call determine_cert_endpoint_mode "usgovvirginia"
            The output should eq "rcv1p"
        End

        It 'returns rcv1p for mooncake (China)'
            When call determine_cert_endpoint_mode "chinaeast2"
            The output should eq "rcv1p"
        End

        It 'returns rcv1p for bleu (EU sovereign)'
            When call determine_cert_endpoint_mode "francesouth"
            The output should eq "rcv1p"
        End

        It 'returns rcv1p for empty location'
            When call determine_cert_endpoint_mode ""
            The output should eq "rcv1p"
        End
    End

    Describe 'init_mariner_repo_depot'
        It 'creates extended, nvidia, and cloud-native repos and points all at RepoDepot'
            export YUM_REPOS_DIR="${TEST_DIR}/yum.repos.d"
            mkdir -p "${YUM_REPOS_DIR}"
            # Seed the extras repo that the function copies from
            cat > "${YUM_REPOS_DIR}/mariner-extras.repo" <<'REPO'
[mariner-official-extras]
name=CBL-Mariner Official Extras 2.0 x86_64
baseurl=https://packages.microsoft.com/cbl-mariner/2.0/prod/extras/x86_64
gpgcheck=1
enabled=1
REPO
            When call init_mariner_repo_depot "https://repodepot.example.com"
            The output should be present
            The status should be success
            The path "${YUM_REPOS_DIR}/mariner-extended.repo" should be exist
            The path "${YUM_REPOS_DIR}/mariner-nvidia.repo" should be exist
            The path "${YUM_REPOS_DIR}/mariner-cloud-native.repo" should be exist
            The contents of file "${YUM_REPOS_DIR}/mariner-extended.repo" should include "repodepot.example.com/mariner/packages.microsoft.com"
            The contents of file "${YUM_REPOS_DIR}/mariner-extended.repo" should not include "https://packages.microsoft.com/cbl-mariner"
            The contents of file "${YUM_REPOS_DIR}/mariner-nvidia.repo" should include "repodepot.example.com/mariner/packages.microsoft.com"
        End
    End

    Describe 'init_azurelinux_repo_depot'
        It 'creates all expected repo files for Azure Linux'
            export YUM_REPOS_DIR="${TEST_DIR}/yum.repos.d"
            mkdir -p "${YUM_REPOS_DIR}"
            When call init_azurelinux_repo_depot "https://repodepot.example.com"
            The output should be present
            The status should be success
            The path "${YUM_REPOS_DIR}/azurelinux-amd.repo" should be exist
            The path "${YUM_REPOS_DIR}/azurelinux-base.repo" should be exist
            The path "${YUM_REPOS_DIR}/azurelinux-cloud-native.repo" should be exist
            The path "${YUM_REPOS_DIR}/azurelinux-extended.repo" should be exist
            The path "${YUM_REPOS_DIR}/azurelinux-ms-non-oss.repo" should be exist
            The path "${YUM_REPOS_DIR}/azurelinux-ms-oss.repo" should be exist
            The path "${YUM_REPOS_DIR}/azurelinux-nvidia.repo" should be exist
            The contents of file "${YUM_REPOS_DIR}/azurelinux-base.repo" should include "baseurl=https://repodepot.example.com/azurelinux/"
            The contents of file "${YUM_REPOS_DIR}/azurelinux-base.repo" should not include "packages.microsoft.com"
        End
    End
End
