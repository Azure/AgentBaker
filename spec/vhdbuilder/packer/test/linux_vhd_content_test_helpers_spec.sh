#!/bin/bash
# shellcheck disable=SC2329

# ShellSpec tests for content-test helpers and call sites

Describe 'eMudhra root removal'
  BeforeAll "eval \"\$(sed -n '/^testEmudhraRootRemoval()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)\""
  err() { echo "$1:Error: $2" >&2; }
  openssl() {
    if [ "$1" = crl2pkcs7 ]; then
      printf 'subject=CN = %s\n' "$CONTROL" "$ROOT"
      return "$READ_STATUS"
    fi
    cat
  }
  Parameters
    'DigiCert Global Root G2' '' 0 /dev/null 0
    'DigiCert Global Root G2' 'emSign ECC Root CA - G3' 0 /dev/null 1
    'DigiCert Global Root G2' 'AAA Certificate Services' 0 /dev/null 1
    '' '' 0 /dev/null 1
    'DigiCert Global Root G2' '' 1 /dev/null 1
    'DigiCert Global Root G2' '' 0 "${SHELLSPEC_WORKDIR}/absent-ca-bundle" 1
  End
  It "checks root '$2', read status $3 and bundle $4"
    CONTROL="$1" ROOT="$2" READ_STATUS="$3"
    When call testEmudhraRootRemoval "$4"
    The status should equal "$5"
    if [ "$5" -ne 0 ]; then
      The stderr should include 'testEmudhraRootRemoval:Error:'
    fi
  End
End

Describe 'Linux content-test legacy parameters'
  read_legacy_parameters() {
    eval "$(sed -n '/^OS_VERSION=/,/^AGENTBAKER_REPOSITORY_URL=/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)"
    printf '<%s>\n' "$OS_VERSION" "$ENABLE_FIPS" "$OS_SKU" "$GIT_BRANCH" \
      "$IMG_SKU" "$FEATURE_FLAGS" "$GIT_COMMIT_HASH" "$AGENTBAKER_REPOSITORY_URL"
  }

  Parameters
    ''
    'https://github.com/example/AgentBaker.git'
  End

  It 'accepts unprefixed values and retains the repository URL default'
    arguments=(22.04 false Ubuntu refs/heads/main 22_04-lts-gen2 cvm commit)
    if [ -n "$1" ]; then arguments+=("$1"); fi
    expected_output=$(printf '<%s>\n' "${arguments[@]:0:7}" "${1:-https://github.com/Azure/AgentBaker.git}")

    When call read_legacy_parameters "${arguments[@]}"
    The status should be success
    The output should equal "$expected_output"
  End
End

Describe 'parseAutologinSessions helper function'
  # Extract only the function definition using sed - clean approach!
  BeforeAll "eval \"\$(sed -n '/^parseAutologinSessions()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  Describe 'parseAutologinSessions'
    It 'returns empty when no sessions exist'
      loginctl() {
        if [ "$1" = "list-sessions" ]; then
          echo ""
        fi
      }
      When call parseAutologinSessions
      The output should equal ""
    End

    It 'returns empty when only SSH sessions exist (Remote=yes)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "1 1000 user seat0"
            ;;
          "show-session")
            cat <<EOF
Remote=yes
Service=sshd
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal ""
    End

    It 'returns session ID when console autologin session found (Remote=no + Service=login)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "42 1000 core seat0"
            ;;
          "show-session")
            cat <<EOF
Remote=no
Service=login
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal "42"
    End

    It 'returns multiple session IDs when multiple autologin sessions exist'
      loginctl() {
        case "$1" in
          "list-sessions")
            cat <<EOF
1 1000 user seat0
2 1001 core seat1
3 1002 admin seat2
EOF
            ;;
          "show-session")
            cat <<EOF
Remote=no
Service=login
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The line 1 of output should equal "1"
      The line 2 of output should equal "2"
      The line 3 of output should equal "3"
    End

    It 'returns only autologin sessions when mixed with SSH sessions'
      loginctl() {
        case "$1" in
          "list-sessions")
            cat <<EOF
1 1000 user seat0
2 1001 core pts/0
3 1002 admin seat1
EOF
            ;;
          "show-session")
            case "$2" in
              "1")
                cat <<EOF
Remote=no
Service=login
Type=tty
EOF
                ;;
              "2")
                cat <<EOF
Remote=yes
Service=sshd
Type=tty
EOF
                ;;
              "3")
                cat <<EOF
Remote=no
Service=login
Type=tty
EOF
                ;;
            esac
            ;;
        esac
      }
      When call parseAutologinSessions
      The line 1 of output should equal "1"
      The line 2 of output should equal "3"
    End

    It 'detects format changes when both Remote and Service fields are missing'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "99 1000 user seat0"
            ;;
          "show-session")
            # Missing both Remote= and Service= fields - format changed!
            cat <<EOF
Type=tty
State=active
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal "PARSE_ERROR:99"
      The error should include "Cannot parse loginctl output"
      The error should include "format may have changed"
    End

    It 'handles sessions with only Remote field present (partial data)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "98 1000 user seat0"
            ;;
          "show-session")
            cat <<EOF
Remote=yes
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal ""
    End

    It 'handles sessions with only Service field present (partial data)'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "97 1000 user seat0"
            ;;
          "show-session")
            cat <<EOF
Service=login
Type=tty
EOF
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal ""
    End


    It 'detects when loginctl show-session fails or returns empty'
      loginctl() {
        case "$1" in
          "list-sessions")
            echo "99 1000 user seat0"
            ;;
          "show-session")
            # Simulate failure - returns empty output
            return 1
            ;;
        esac
      }
      When call parseAutologinSessions
      The output should equal "PARSE_ERROR:99"
      The error should include "Command may have failed"
    End
  End
End

Describe 'Inspektor Gadget version helper functions'
  BeforeAll "eval \"\$(sed -n '/^extractIgUpstreamVersion()/,/^}/p;/^igPackageVersionsShareUpstreamVersion()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  Describe 'extractIgUpstreamVersion'
    It 'extracts the upstream version from an Ubuntu package version'
      When call extractIgUpstreamVersion "0.51.0-ubuntu24.04u4"
      The output should equal "0.51.0"
    End

    It 'extracts the upstream version from an Azure Linux package version'
      When call extractIgUpstreamVersion "0.51.0-4.azl3"
      The output should equal "0.51.0"
    End

    It 'fails for an unparsable package version'
      When call extractIgUpstreamVersion "not-a-version"
      The status should equal 1
      The output should equal ""
    End
  End

  Describe 'igPackageVersionsShareUpstreamVersion'
    It 'accepts matching upstream versions with different distro revisions'
      When call igPackageVersionsShareUpstreamVersion "0.51.0-4.azl3" "0.51.0-1.azl3"
      The status should equal 0
    End

    It 'rejects different upstream versions'
      When call igPackageVersionsShareUpstreamVersion "0.51.1-1.azl3" "0.51.0-1.azl3"
      The status should equal 1
    End
  End
End

Describe 'testCronPermissions invocation'
  BeforeAll "eval \"\$(sed -n '/^testCronPermissions()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  checkPathPermissions() {
    echo "checkPathPermissions:$2:$3:$4"
  }

  checkPathDoesNotExist() {
    echo "checkPathDoesNotExist:$2"
  }

  runCronPermissionsTest() {
    # Exercise argument expansion at the real call site without running the full VHD suite.
    eval "$(sed -n '/^testCronPermissions /p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')"
  }

  Describe 'images exempt from cron checks'
    Parameters
      "" "AzureContainerLinux"
      "azure-linux-3-acl" "AzureContainerLinux"
      "" "Flatcar"
      "minimal" "Ubuntu"
    End

    It 'preserves the cron exemption at the call site'
      IMG_SKU="$1"
      OS_SKU="$2"

      When call runCronPermissionsTest
      The status should be success
      The output should include "testCronPermissions: Skipping cron file check"
      The output should not include "checkPathPermissions:"
      The output should not include "checkPathDoesNotExist:"
    End
  End

  It 'still checks cron paths for regular Ubuntu images'
    IMG_SKU="22_04-lts-gen2"
    OS_SKU="Ubuntu"

    When call runCronPermissionsTest
    The status should be success
    The output should include "checkPathPermissions:/etc/cron.allow:640:1"
    The output should include "checkPathPermissions:/etc/cron.hourly:600:1"
    The output should include "checkPathPermissions:/etc/crontab:600:0"
    The output should include "checkPathDoesNotExist:/etc/cron.deny"
    The output should not include "testCronPermissions: Skipping cron file check"
  End
End

Describe 'isCloudInitLogMsgIgnored'
  BeforeAll "eval \"\$(sed -n '/^CLOUD_INIT_LOG_MSG_IGNORE_LIST=(/,/^)/p;/^AZL4_CLOUD_INIT_LOG_MSG_IGNORE_LIST=(/,/^)/p;/^isCloudInitLogMsgIgnored()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  Context 'AzureLinux 4.0 DHCP lease errors'
    Parameters
      "4.0" "2026-10-27 20:23:26,123 - azure.py[ERROR]: Failed to obtain DHCP lease (iface=None)" success
      "4.0" "2026-10-27 20:23:26,124 - azure.py[ERROR]: Azure datasource failure occurred: result=error|reason=failure to obtain DHCP lease|agent=Cloud-Init/25.2|duration=300.27|interface=None" success
      "4.0" "2026-10-27 20:18:26,125 - subp.py[WARNING]: Command ['hostname', '-f'] failed" success
      "4.0" "2026-10-27 20:23:26,126 - util.py[ERROR]: Failed to mount /dev/sdb1" failure
      "3.0" "2026-10-27 20:23:26,123 - azure.py[ERROR]: Failed to obtain DHCP lease (iface=None)" failure
    End

    It "returns $3 on OS_VERSION $1 for '$2'"
      OS_VERSION="$1"
      When call isCloudInitLogMsgIgnored "$2"
      The status should be "$3"
    End
  End

  It 'ignores a message matching the ignore list'
    When call isCloudInitLogMsgIgnored "2026-10-27 20:18:26,125 - subp.py[WARNING]: Command ['hostname', '-f'] failed"
    The status should be success
  End

  It 'does not ignore a message matching no ignore-list pattern'
    When call isCloudInitLogMsgIgnored "2026-10-27 20:23:26,126 - util.py[ERROR]: Failed to mount /dev/sdb1"
    The status should be failure
  End

  It 'ignores a message matching a later pattern without flagging earlier ones'
    CLOUD_INIT_LOG_MSG_IGNORE_LIST+=("util.py[ERROR]: Failed to mount")
    When call isCloudInitLogMsgIgnored "2026-10-27 20:23:26,126 - util.py[ERROR]: Failed to mount /dev/sdb1"
    The status should be success
  End
End

Describe 'testPam AzureLinux 4.0 failure handling'
  BeforeAll "eval \"\$(sed -n '/^testPam()/,/^}/p' './vhdbuilder/packer/test/linux-vhd-content-test.sh')\""

  err() { echo "$1:Error: $2" >&2; }
  python3() { :; }
  pip3() { :; }
  deactivate() { :; }
  pytest() {
    echo "test_pam.py::test_user_can_login RERUN"
    echo "FAILED test_pam.py::test_user_can_login - assert False"
    echo "==================== 1 failed, 12 passed, 5 rerun in 9.00s ===================="
    return 1
  }

  setup_pam_dir() {
    PAM_TEST_ROOT=$(mktemp -d)
    mkdir -p "$PAM_TEST_ROOT/AgentBaker/vhdbuilder/packer/test/pam/bin"
    touch "$PAM_TEST_ROOT/AgentBaker/vhdbuilder/packer/test/pam/bin/activate"
    cd "$PAM_TEST_ROOT" || return 1
  }

  cleanup_pam_dir() {
    cd "$SHELLSPEC_PROJECT_ROOT" && rm -rf "$PAM_TEST_ROOT"
  }

  BeforeEach 'setup_pam_dir'
  AfterEach 'cleanup_pam_dir'

  It 'reports PAM failures as warnings on AzureLinux 4.0'
    When call testPam AzureLinux 4.0
    The status should be success
    The output should include 'testPam: Warning: ignoring PAM test failures on AzureLinux 4.0'
    The output should include 'FAILED test_pam.py::test_user_can_login - assert False'
    The output should include '1 failed, 12 passed, 5 rerun'
    The output should not include 'test_pam.py::test_user_can_login RERUN'
    The stderr should not include ':Error:'
  End

  It 'keeps PAM failures fatal on AzureLinux 3.0'
    When call testPam AzureLinux 3.0
    The status should be failure
    The output should not include 'Warning: ignoring PAM test failures'
    The stderr should include 'testPam:Error: PAM configuration is not functional'
  End
End
