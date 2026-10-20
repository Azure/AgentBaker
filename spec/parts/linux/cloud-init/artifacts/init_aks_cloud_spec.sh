#!/bin/bash

# Tests for parts/linux/cloud-init/artifacts/init-aks-cloud.sh
#
# Covers three areas:
# 1. Structural wiring tests (grep-based) for the ca-refresh mode added by #8096.
# 2. Repo initialization dispatch, independent of certificate refresh scheduling.
# 3. Functional tests that source the script and exercise:
#    - repo-depot helpers (init_ubuntu_main_repo_depot, init_ubuntu_pmc_repo_depot,
#      init_mariner_repo_depot, init_azurelinux_repo_depot, check_url)
#    - cloud mode selection helper (determine_cert_endpoint_mode)

Describe 'init-aks-cloud.sh refresh mode wiring'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-cloud.sh'

    It 'parses action argument after deriving location, with init default'
        When run grep -Eq '^action=\$\{1:-init\}$' "$script_path"
        The status should eq 0
    End

    It 'uses arg2 as location fallback when invoked as ca-refresh'
        When run grep -Eq '^refresh_location="\$\{2:-\$\{LOCATION\}\}"$' "$script_path"
        The status should eq 0
    End

    It 'always derives cert endpoint mode from refresh_location'
        When run grep -Eq '^location_normalized="\$\{refresh_location,,\}"$' "$script_path"
        The status should eq 0
    End

    It 'passes refresh_location (not the raw positional arg) into determine_cert_endpoint_mode'
        When run grep -Eq '^cert_endpoint_mode=\$\(determine_cert_endpoint_mode "\$refresh_location"\)$' "$script_path"
        The status should eq 0
    End

    It 'initializes refresh schedule installation as disabled'
        When run grep -Eq '^install_ca_refresh_schedule=0$' "$script_path"
        The status should eq 0
    End

    It 'enables refresh schedule installation for eligible certificate modes'
        When run grep -Eq '^[[:space:]]*install_ca_refresh_schedule=1$' "$script_path"
        The status should eq 0
    End

    It 'gates refresh schedule installation on install_ca_refresh_schedule'
        When run grep -Eq '\[ "\$install_ca_refresh_schedule" -eq 0 \]' "$script_path"
        The status should eq 0
    End

    It 'checks for ca-refresh mode after certificate refresh logic'
        When run grep -Eq '^if \[ "\$action" = "ca-refresh" \] \|\| \[ "\$install_ca_refresh_schedule" -eq 0 \]; then$' "$script_path"
        The status should eq 0
    End

    It 'exits early in ca-refresh mode after certificate refresh logic'
        When run grep -Eq '^[[:space:]]*exit 0$' "$script_path"
        The status should eq 0
    End

    It 'passes LOCATION directly into cron refresh command'
        When run grep -Eq 'ca-refresh \\"\$LOCATION\\"' "$script_path"
        The status should eq 0
    End

    It 'passes LOCATION directly into systemd refresh command'
        When run grep -Eq '^ExecStart=\$script_path ca-refresh \$LOCATION$' "$script_path"
        The status should eq 0
    End

    It 'does not wait for cloud-init completion'
        When run grep -Eq '^[[:space:]]*cloud-init status --wait' "$script_path"
        The status should eq 1
    End
End

Describe 'init-aks-cloud.sh repo initialization dispatch'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-cloud.sh'

    setup_repo_dispatch() {
        IS_UBUNTU=0
        IS_MARINER=0
        IS_AZURELINUX=0
        install_ca_refresh_schedule=0
        REPO_DEPOT_ENDPOINT='https://repodepot.example.com/ubuntu'
    }
    BeforeEach 'setup_repo_dispatch'

    run_repo_dispatch() {
        # Run the actual action dispatch and early-exit guard, but not host scheduling.
        . <(sed -n '/^action=/,/^if \[ "\$IS_UBUNTU"/p' "$script_path" | sed '$d') "$@"
        echo 'refresh scheduling reached'
    }

    Mock lsb_release
        case "$1" in
            --release) printf 'Release:\t22.04\n' ;;
            -c) printf 'Codename:\tjammy\n' ;;
            *) exit 1 ;;
        esac
    End

    Mock init_ubuntu_main_repo_depot
        echo "ubuntu-main $1"
    End

    Mock init_ubuntu_pmc_repo_depot
        echo "ubuntu-pmc $1"
    End

    Mock init_mariner_repo_depot
        echo "mariner $1"
    End

    Mock init_azurelinux_repo_depot
        echo "azurelinux $1"
    End

    Mock aptget_update
        echo 'apt update'
    End

    Mock dnf_makecache
        echo 'dnf cache'
    End

    Mock emit_event
        echo "event $1"
    End

    It 'initializes Ubuntu repos without certificate opt-in when action is omitted'
        IS_UBUNTU=1

        When run run_repo_dispatch
        The output should include 'ubuntu-main https://repodepot.example.com'
        The output should include 'ubuntu-pmc https://repodepot.example.com'
        The output should include 'apt update'
        The output should not include 'dnf cache'
        The output should not include 'refresh scheduling reached'
        The status should be success
    End

    Describe 'explicit init'
        Parameters
            1 0 0 'ubuntu-main' 'apt update' 0
            1 0 0 'ubuntu-main' 'apt update' 1
            0 1 0 'mariner' 'dnf cache' 0
            0 1 0 'mariner' 'dnf cache' 1
            0 0 1 'azurelinux' 'dnf cache' 0
            0 0 1 'azurelinux' 'dnf cache' 1
        End

        It 'routes repo setup independently of certificate refresh eligibility'
            IS_UBUNTU=$1
            IS_MARINER=$2
            IS_AZURELINUX=$3
            install_ca_refresh_schedule=$6

            When run run_repo_dispatch init
            The output should include "$4 https://repodepot.example.com"
            The output should include "$5"
            if [ "$6" -eq 1 ]; then
                The output should include 'refresh scheduling reached'
            else
                The output should not include 'refresh scheduling reached'
            fi
            The status should be success
        End
    End

    Describe 'ca-refresh'
        Parameters
            0
            1
        End

        It 'skips repo initialization and scheduling regardless of certificate opt-in'
            IS_UBUNTU=1
            install_ca_refresh_schedule=$1

            When run run_repo_dispatch ca-refresh
            The output should be blank
            The status should be success
        End
    End

    Describe 'empty endpoint'
        Parameters
            1 0 0
            0 1 0
            0 0 1
        End

        It 'skips repo initialization and package cache updates'
            IS_UBUNTU=$1
            IS_MARINER=$2
            IS_AZURELINUX=$3
            REPO_DEPOT_ENDPOINT=''

            When run run_repo_dispatch init
            The output should equal 'repo depot endpoint empty, skipping repodepot initialization'
            The status should be success
        End
    End

    Describe 'non-fatal package cache failure'
        Parameters
            1 0 0 0
            1 0 0 1
            0 1 0 0
            0 1 0 1
            0 0 1 0
            0 0 1 1
        End

        run_failed_repo_dispatch() {
            aptget_update() { return 1; }
            dnf_makecache() { return 1; }
            run_repo_dispatch init
        }

        It 'continues to refresh scheduling only for eligible nodes after cache failure'
            IS_UBUNTU=$1
            IS_MARINER=$2
            IS_AZURELINUX=$3
            install_ca_refresh_schedule=$4

            When run run_failed_repo_dispatch
            The output should be present
            if [ "$4" -eq 1 ]; then
                The output should include 'refresh scheduling reached'
            else
                The output should not include 'refresh scheduling reached'
            fi
            The status should be success
        End
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

    Describe 'package cache retries'
        setup_cache_retries() {
            RETRY_ATTEMPTS_FILE="${TEST_DIR}/attempts"
            RETRY_SLEEPS_FILE="${TEST_DIR}/sleeps"
            RETRY_EVENTS_FILE="${TEST_DIR}/events"
            printf '0' > "$RETRY_ATTEMPTS_FILE"
            printf '0' > "$RETRY_SLEEPS_FILE"
            : > "$RETRY_EVENTS_FILE"
            RETRY_FAILURES=0
            RETRY_FAILURE_STATUS=1
            RETRY_FAILURE_TEXT='Error: 404 Not Found'
            # Keep DNF's output file inside the fixture rather than shared /tmp.
            __SOURCED__=1 . <(sed "s|/tmp/dnf-makecache.out|${TEST_DIR}/dnf-makecache.out|g" \
                ./parts/linux/cloud-init/artifacts/init-aks-cloud.sh)
        }
        BeforeEach 'setup_cache_retries'

        cache_attempt() {
            local attempt
            attempt=$(cat "$RETRY_ATTEMPTS_FILE")
            attempt=$((attempt + 1))
            printf '%s' "$attempt" > "$RETRY_ATTEMPTS_FILE"
            if [ "$attempt" -le "$RETRY_FAILURES" ]; then
                echo "$RETRY_FAILURE_TEXT"
                return "$RETRY_FAILURE_STATUS"
            fi
            return 0
        }

        run_cache_update() {
            apt-get() { cache_attempt; }
            dnf() { cache_attempt; }
            sleep() {
                local sleeps
                sleeps=$(cat "$RETRY_SLEEPS_FILE")
                printf '%s' "$((sleeps + 1))" > "$RETRY_SLEEPS_FILE"
            }
            emit_event() { echo "$1" >> "$RETRY_EVENTS_FILE"; }
            "$1"
        }

        Describe 'successful updates'
            Parameters
                aptget_update 0 1 0
                aptget_update 2 3 2
                aptget_update 9 10 9
                dnf_makecache 0 1 0
                dnf_makecache 2 3 2
                dnf_makecache 9 10 9
            End

            It 'stops at the first success and sleeps only between attempts'
                RETRY_FAILURES=$2

                When run run_cache_update "$1"
                The output should be present
                The status should be success
                The contents of file "$RETRY_ATTEMPTS_FILE" should equal "$3"
                The contents of file "$RETRY_SLEEPS_FILE" should equal "$4"
                The contents of file "$RETRY_EVENTS_FILE" should be blank
            End
        End

        Describe 'exhausted retries'
            Parameters
                aptget_update aptgetUpdateFailed
                dnf_makecache dnfMakecacheFailed
            End

            It 'returns failure and emits an event after ten attempts and nine sleeps'
                RETRY_FAILURES=10

                When run run_cache_update "$1"
                The output should be present
                The error should include 'ERROR:'
                The status should eq 1
                The contents of file "$RETRY_ATTEMPTS_FILE" should equal 10
                The contents of file "$RETRY_SLEEPS_FILE" should equal 9
                The contents of file "$RETRY_EVENTS_FILE" should equal "AKS.CSE.customCloudRepoInit.$2"
            End
        End

        It 'accepts a successful APT exit status regardless of diagnostic text'
            RETRY_FAILURES=10
            RETRY_FAILURE_STATUS=0

            When run run_cache_update aptget_update
            The output should be present
            The status should be success
            The contents of file "$RETRY_ATTEMPTS_FILE" should equal 1
            The contents of file "$RETRY_SLEEPS_FILE" should equal 0
            The contents of file "$RETRY_EVENTS_FILE" should be blank
        End

        It 'retries non-404 APT failures until attempts are exhausted'
            RETRY_FAILURES=10
            RETRY_FAILURE_STATUS=100
            RETRY_FAILURE_TEXT='Temporary failure resolving repository'

            When run run_cache_update aptget_update
            The output should be present
            The error should include 'ERROR: apt-get update failed after 10 attempts'
            The status should eq 1
            The contents of file "$RETRY_ATTEMPTS_FILE" should equal 10
            The contents of file "$RETRY_SLEEPS_FILE" should equal 9
            The contents of file "$RETRY_EVENTS_FILE" should equal 'AKS.CSE.customCloudRepoInit.aptgetUpdateFailed'
        End

        It 'retains DNF text-based detection rather than failing on exit status alone'
            RETRY_FAILURES=10
            RETRY_FAILURE_TEXT='unmatched diagnostic'

            When run run_cache_update dnf_makecache
            The output should include 'unmatched diagnostic'
            The status should be success
            The contents of file "$RETRY_ATTEMPTS_FILE" should equal 1
            The contents of file "$RETRY_SLEEPS_FILE" should equal 0
        End
    End

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
