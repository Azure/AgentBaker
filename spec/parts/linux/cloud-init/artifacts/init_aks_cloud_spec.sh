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

    It 'maps ussec/usnat locations to legacy cert endpoint mode'
        When run grep -Eq 'ussec\*\|usnat\*\) mode="legacy"' "$script_path"
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
        When run grep -Eq '^[[:space:]]*if \[ "\$install_ca_refresh_schedule" -eq 1 \]; then$' "$script_path"
        The status should eq 0
    End

    It 'checks for ca-refresh mode after certificate refresh logic'
        When run grep -Eq '^if \[ "\$action" = "ca-refresh" \]; then$' "$script_path"
        The status should eq 0
    End

    It 'exits early in ca-refresh mode after certificate refresh logic'
        When run grep -Eq '^[[:space:]]*exit$' "$script_path"
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

Describe 'init-aks-cloud.sh script-level wiring'
    script_path='./parts/linux/cloud-init/artifacts/init-aks-cloud.sh'

    setup_script_level() {
        TEST_DIR="$(mktemp -d)"
        MOCK_BIN_DIR="${TEST_DIR}/mock-bin"
        EVENTS_DIR="${TEST_DIR}/events"
        SCRIPT_COPY="${TEST_DIR}/init-aks-cloud.sh"
        mkdir -p "${MOCK_BIN_DIR}" "${EVENTS_DIR}"
        cp "${script_path}" "${SCRIPT_COPY}"
        sed -i "s|^EVENTS_LOGGING_DIR=.*|EVENTS_LOGGING_DIR=\"${EVENTS_DIR}/\"|" "${SCRIPT_COPY}"

        cat > "${MOCK_BIN_DIR}/curl" <<'EOF'
#!/bin/bash
url="${*: -1}"
case "$url" in
    *"type=cacertificates&ext=json")
        cat <<'RESPONSE'
[{"Name": "test.cer", "CertBody": "-----BEGIN CERTIFICATE-----\r\nMIIB\r\n-----END CERTIFICATE-----"}]
200
RESPONSE
        ;;
    *)
        printf '\n500\n'
        ;;
esac
EOF
        cat > "${MOCK_BIN_DIR}/jq" <<'EOF'
#!/bin/bash
task=""
message=""
while [ $# -gt 0 ]; do
    if [ "$1" = "--arg" ]; then
        key="$2"
        value="$3"
        case "$key" in
            TaskName) task="$value" ;;
            Message) message="$value" ;;
        esac
        shift 3
        continue
    fi
    shift
done
printf '{"TaskName":"%s","Message":"%s"}\n' "$task" "$message"
EOF
        cat > "${MOCK_BIN_DIR}/grep" <<'EOF'
#!/bin/bash
if [ "$1" = "-oP" ]; then
    pattern="$2"
    case "$pattern" in
        '(?<=Name\": \")[^\"]*')
            echo "test.cer"
            exit 0
            ;;
        '(?<=CertBody\": \")[^\"]*')
            echo "-----BEGIN CERTIFICATE-----\\r\\nMIIB\\r\\n-----END CERTIFICATE-----"
            exit 0
            ;;
    esac
fi
exec /usr/bin/grep "$@"
EOF
        cat > "${MOCK_BIN_DIR}/sleep" <<'EOF'
#!/bin/bash
exit 0
EOF
        cat > "${MOCK_BIN_DIR}/cp" <<'EOF'
#!/bin/bash
exit 0
EOF
        cat > "${MOCK_BIN_DIR}/update-ca-certificates" <<'EOF'
#!/bin/bash
exit 0
EOF
        cat > "${MOCK_BIN_DIR}/update-ca-trust" <<'EOF'
#!/bin/bash
exit 0
EOF
        chmod +x "${MOCK_BIN_DIR}/curl" "${MOCK_BIN_DIR}/jq" "${MOCK_BIN_DIR}/grep" "${MOCK_BIN_DIR}/sleep" \
            "${MOCK_BIN_DIR}/cp" "${MOCK_BIN_DIR}/update-ca-certificates" \
            "${MOCK_BIN_DIR}/update-ca-trust"
    }

    cleanup_script_level() {
        rm -rf "${TEST_DIR}"
    }

    BeforeEach 'setup_script_level'
    AfterEach 'cleanup_script_level'

    It 'passes refresh_location into determine_cert_endpoint_mode and emits legacy mode event'
        When run env -u __SOURCED__ PATH="${MOCK_BIN_DIR}:$PATH" LOCATION="zzzz" bash -c 'script="$1"; events="$2"; bash "$script" ca-refresh usseceast >/dev/null 2>&1; rc=$?; cat "$events"/*.json; exit "$rc"' _ "${SCRIPT_COPY}" "${EVENTS_DIR}"
        The status should be success
        The stdout should include '"TaskName":"AKS.CSE.rcv1p.certEndpointMode"'
        The stdout should include 'mode=legacy, location=usseceast'
    End
End

Describe 'init-aks-cloud.sh functional tests'
    # Set __SOURCED__ before Include so only the function definitions are loaded. The
    # script's ${__SOURCED__:+return 0} guard is a no-op when __SOURCED__ is unset, so
    # without this Include would fall through into the top-level provisioning path
    # (wireserver calls, cron install, exit) and cause side effects. Matches the
    # sourcing convention documented in the script header and used in cse_main_spec.sh.
    __SOURCED__=1
    Include "./parts/linux/cloud-init/artifacts/init-aks-cloud.sh"

    setup() {
        TEST_DIR="$(mktemp -d)"
        export OS_RELEASE_FILE="${TEST_DIR}/os-release"
        export APT_SOURCES_LIST="${TEST_DIR}/apt/sources.list"
        export APT_SOURCES_LIST_D_DIR="${TEST_DIR}/apt/sources.list.d"
        export APT_KEYRINGS_DIR="${TEST_DIR}/apt/keyrings"
        export APT_BACKUP_DIR="${TEST_DIR}/apt/backup"
        export SSL_CERTS_DIR="${TEST_DIR}/ssl-certs"
        export SSL_CERT_TARGET="${TEST_DIR}/ssl-cert-target.pem"
        export WIRESERVER_ENDPOINT="http://wireserver.local"
        mkdir -p "${APT_SOURCES_LIST_D_DIR}" "${APT_KEYRINGS_DIR}" \
                 "${APT_BACKUP_DIR}" "${SSL_CERTS_DIR}" \
                 "$(dirname "${APT_SOURCES_LIST}")"
        # ca-certificates.crt is referenced when copying the bundle
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
