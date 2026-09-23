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
End

Describe 'cse_config_chrony.sh distro routing'
    script_path='./parts/linux/cloud-init/artifacts/cse_config_chrony.sh'

    chrony_routing_block() {
        sed -n '/^configureChrony() {$/,/^}$/p' "$script_path"
    }

    It 'keeps Azure Linux and Mariner on their native chronyd configuration path'
        When call chrony_routing_block
        The output should include 'elif isMarinerOrAzureLinux "$OS"; then'
        The output should include 'configure_mariner_azurelinux_chrony || true'
    End

    It 'keeps Ubuntu and Flatcar on configure_chrony unless the CVM path is selected'
        When call chrony_routing_block
        The output should include 'elif should_configure_ubuntu_2604_cvm_time_sync; then'
        The output should include 'configure_ubuntu_2604_cvm_time_sync'
        The output should include 'configure_chrony || true'
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
        UBUNTU_OS_NAME="UBUNTU"
        FLATCAR_OS_NAME="FLATCAR"
        ERR_CVM_PLATFORM_DETECTION_FAIL=244
        ERR_NTP_UNREACHABLE=245
        ERR_CHRONY_CONFIG_FAIL=246
        # shellcheck disable=SC1091
        . "./parts/linux/cloud-init/artifacts/cse_config_chrony.sh"
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

    Describe 'Ubuntu 26.04 CVM Chrony configuration'
        setup_chrony_test() {
            export CHRONY_CONF="${TEST_DIR}/chrony.conf"
            OS="$UBUNTU_OS_NAME"
            OS_VERSION="26.04"
            touch "$CHRONY_CONF"
        }

        Mock systemctl
            echo "systemctl $*"
        End

        It 'scopes the platform-specific behavior to Ubuntu 26.04 FDE images'
            OS="$UBUNTU_OS_NAME"
            OS_VERSION="26.04"
            Mock uname
                echo "7.0.0-1011-azure-fde"
            End

            When call is_ubuntu_2604_cvm
            The status should be success
        End

        It 'does not select an Ubuntu 26.04 non-FDE image'
            OS="$UBUNTU_OS_NAME"
            OS_VERSION="26.04"
            Mock uname
                echo "7.0.0-1011-azure"
            End

            When call is_ubuntu_2604_cvm
            The status should be failure
        End

        It 'does not select another Ubuntu release even when it has an FDE kernel'
            OS="$UBUNTU_OS_NAME"
            OS_VERSION="24.04"
            Mock uname
                echo "6.8.0-1065-azure-fde"
            End

            When call is_ubuntu_2604_cvm
            The status should be failure
        End

        It 'skips platform-specific time sync during pre-provision image preparation'
            OS="$UBUNTU_OS_NAME"
            OS_VERSION="26.04"
            PRE_PROVISION_ONLY="true"
            Mock uname
                echo "7.0.0-1011-azure-fde"
            End

            When call should_configure_ubuntu_2604_cvm_time_sync
            The status should be failure
        End

        It 'selects platform-specific time sync when provisioning the real node'
            OS="$UBUNTU_OS_NAME"
            OS_VERSION="26.04"
            PRE_PROVISION_ONLY="false"
            Mock uname
                echo "7.0.0-1011-azure-fde"
            End

            When call should_configure_ubuntu_2604_cvm_time_sync
            The status should be success
        End

        It 'preserves the PHC default for another Ubuntu release'
            setup_chrony_test
            OS_VERSION="24.04"

            When call configure_chrony
            The output should include "systemctl restart chrony"
            The contents of file "$CHRONY_CONF" should include "refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0"
            The status should be success
        End

        It 'detects AMD SEV-SNP with the systemd confidential VM signal'
            Mock systemd-detect-virt
                [ "$1" = "--cvm" ] || return 1
                echo "sev-snp"
            End

            When call detect_confidential_vm_platform
            The output should eq "sev-snp"
            The status should be success
        End

        It 'detects Intel TDX with the systemd confidential VM signal'
            Mock systemd-detect-virt
                [ "$1" = "--cvm" ] || return 1
                echo "tdx"
            End

            When call detect_confidential_vm_platform
            The output should eq "tdx"
            The status should be success
        End

        It 'rejects an unknown confidential VM platform'
            Mock systemd-detect-virt
                echo "none"
            End

            When call detect_confidential_vm_platform
            The status should be failure
        End

        It 'fails provisioning when the confidential VM platform cannot be determined'
            Mock detect_confidential_vm_platform
                return 1
            End
            Mock chrony_emit_event
                echo "event: $*" >&2
            End

            When call configure_ubuntu_2604_cvm_time_sync
            The error should include "unable to determine Ubuntu 26.04 CVM platform"
            The error should include "AKS.CSE.chrony.platformDetectionFailed"
            The status should equal 244
        End

        It 'preserves the existing PHC configuration for SEV-SNP'
            setup_chrony_test
            Mock detect_confidential_vm_platform
                echo "sev-snp"
            End
            Mock chrony_emit_event
                echo "event: $*"
            End

            When call configure_ubuntu_2604_cvm_time_sync
            The output should include "preserving the existing Hyper-V PHC Chrony configuration"
            The output should include "AKS.CSE.chrony.usingPHC"
            The contents of file "$CHRONY_CONF" should include "refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0"
            The status should be success
        End

        It 'returns the Chrony configuration failure code when SEV-SNP PHC setup fails'
            Mock detect_confidential_vm_platform
                echo "sev-snp"
            End
            Mock configure_chrony
                return 1
            End
            Mock chrony_emit_event
                echo "event: $*" >&2
            End

            When call configure_ubuntu_2604_cvm_time_sync
            The output should include "AMD SEV-SNP detected"
            The error should include "failed to configure Chrony with the Hyper-V PHC source"
            The error should include "AKS.CSE.chrony.configurationFailed"
            The status should equal 246
        End

        It 'defines exactly the four approved Ubuntu NTP pools'
            When call ubuntu_ntp_pools
            The lines of output should eq 4
            The line 1 of output should eq "pool ntp.ubuntu.com        iburst maxsources 4"
            The line 2 of output should eq "pool 0.ubuntu.pool.ntp.org iburst maxsources 1"
            The line 3 of output should eq "pool 1.ubuntu.pool.ntp.org iburst maxsources 1"
            The line 4 of output should eq "pool 2.ubuntu.pool.ntp.org iburst maxsources 2"
            The status should be success
        End

        It 'configures TDX with only the fixed Ubuntu NTP pools'
            setup_chrony_test
            Mock detect_confidential_vm_platform
                echo "tdx"
            End
            Mock verify_chrony_ntp_sync
                echo "verified NTP synchronization"
            End
            Mock chrony_emit_event
                echo "event: $*"
            End

            When call configure_ubuntu_2604_cvm_time_sync
            The output should include "Intel TDX detected"
            The output should include "verified NTP synchronization"
            The contents of file "$CHRONY_CONF" should include "pool ntp.ubuntu.com        iburst maxsources 4"
            The contents of file "$CHRONY_CONF" should include "pool 0.ubuntu.pool.ntp.org iburst maxsources 1"
            The contents of file "$CHRONY_CONF" should include "pool 1.ubuntu.pool.ntp.org iburst maxsources 1"
            The contents of file "$CHRONY_CONF" should include "pool 2.ubuntu.pool.ntp.org iburst maxsources 2"
            The contents of file "$CHRONY_CONF" should not include "refclock PHC"
            The status should be success
        End

        It 'returns failure when the Chrony service cannot restart'
            setup_chrony_test
            Mock systemctl
                if [ "$1" = "restart" ]; then
                    return 1
                fi
            End

            When call configure_chrony
            The error should include "failed to restart Chrony"
            The status should equal 1
        End

        It 'skips stopping and disabling systemd-timesyncd when the unit is removed'
            setup_chrony_test
            Mock systemctl
                if [ "$1" = "show" ]; then
                    echo "not-found"
                    return 1
                fi
                if [ "$1" = "stop" ] || [ "$1" = "disable" ]; then
                    echo "unexpected systemd-timesyncd operation"
                    return 1
                fi
            End

            When call configure_chrony
            The output should include "systemd-timesyncd is removed, no need to disable"
            The output should not include "unexpected systemd-timesyncd operation"
            The contents of file "$CHRONY_CONF" should include "refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0"
            The status should be success
        End

        It 'stops and disables systemd-timesyncd when the unit is loaded but inactive'
            setup_chrony_test
            Mock systemctl
                if [ "$1" = "show" ]; then
                    echo "loaded"
                elif [ "$1" = "stop" ] || [ "$1" = "disable" ]; then
                    echo "systemctl $*"
                fi
            End

            When call configure_chrony
            The output should include "systemctl stop systemd-timesyncd"
            The output should include "systemctl disable systemd-timesyncd"
            The contents of file "$CHRONY_CONF" should include "refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0"
            The status should be success
        End

        It 'continues Chrony setup and returns failure when systemd-timesyncd cannot be stopped'
            setup_chrony_test
            Mock systemctl
                if [ "$1" = "show" ]; then
                    echo "loaded"
                elif [ "$1" = "stop" ]; then
                    return 1
                elif [ "$1" = "disable" ] || [ "$1" = "restart" ]; then
                    echo "systemctl $*"
                fi
            End

            When call configure_chrony
            The error should include "failed to stop systemd-timesyncd"
            The output should include "systemctl disable systemd-timesyncd"
            The output should include "systemctl restart chrony"
            The contents of file "$CHRONY_CONF" should include "refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0"
            The status should equal 1
        End

        It 'returns the Chrony configuration failure code without checking NTP when TDX setup fails'
            Mock detect_confidential_vm_platform
                echo "tdx"
            End
            Mock ubuntu_ntp_pools
                echo "fixed pools"
            End
            Mock configure_chrony
                return 1
            End
            Mock verify_chrony_ntp_sync
                echo "unexpected NTP verification"
            End
            Mock chrony_emit_event
                echo "event: $*" >&2
            End

            When call configure_ubuntu_2604_cvm_time_sync
            The output should not include "unexpected NTP verification"
            The error should include "failed to configure Chrony with the Ubuntu NTP pools"
            The error should include "AKS.CSE.chrony.configurationFailed"
            The status should equal 246
        End

        It 'waits for Chrony to synchronize successfully'
            Mock chronyc
                echo "$*"
            End
            Mock chrony_emit_event
                echo "event: $*"
            End

            When call verify_chrony_ntp_sync
            The output should include "waitsync 12 0 0 5"
            The output should include "NTP synchronization confirmed through the Ubuntu NTP pools"
            The output should include "AKS.CSE.chrony.ntpSynchronized"
            The status should be success
        End

        It 'returns the NTP-unreachable code with Chrony diagnostics when NTP is not reachable'
            Mock chronyc
                case "$1" in
                    waitsync)
                        return 1
                        ;;
                    sources)
                        echo "mock Chrony sources"
                        ;;
                    tracking)
                        echo "mock Chrony tracking"
                        ;;
                esac
            End
            Mock chrony_emit_event
                echo "event: $*" >&2
            End

            When call verify_chrony_ntp_sync
            The error should include "NTP not reachable"
            The error should include "AKS.CSE.chrony.ntpUnavailable"
            The error should include "mock Chrony sources"
            The error should include "mock Chrony tracking"
            The status should equal 245
        End

        It 'propagates failed TDX NTP synchronization'
            Mock detect_confidential_vm_platform
                echo "tdx"
            End
            Mock ubuntu_ntp_pools
                echo "fixed pools"
            End
            Mock configure_chrony
                :
            End
            Mock verify_chrony_ntp_sync
                exit 245
            End
            Mock chrony_emit_event
                :
            End

            When call configure_ubuntu_2604_cvm_time_sync
            The output should include "Intel TDX detected"
            The status should equal 245
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
