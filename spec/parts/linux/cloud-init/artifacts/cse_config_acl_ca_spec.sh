#!/bin/bash
# shellcheck disable=SC2329

Describe 'ACL proxy CA configuration'
  setup() {
    TEST_ROOT=$(mktemp -d)
    mkdir -p "$TEST_ROOT/etc/pki/ca-trust/source/anchors"
    ACL_OS_NAME=AZURECONTAINERLINUX
    AZURELINUX_OS_NAME=AZURELINUX
    ACL_OS_VARIANT=AZURECONTAINERLINUX
    ERR_UPDATE_CA_CERTS=42
    UPDATE_STATUS=0
    eval "$(sed -n '/^isACL()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
    eval "$(sed -n '/^configureHTTPProxyCA()/,/^}/p' parts/linux/cloud-init/artifacts/cse_config.sh |
      sed "s|/etc/pki/ca-trust/source/anchors|$TEST_ROOT/etc/pki/ca-trust/source/anchors|")"
    HTTP_PROXY_TRUSTED_CA=$(printf '%s' 'proxy CA fixture' | base64 | tr -d '\n')
  }
  cleanup() { rm -rf "$TEST_ROOT"; }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  isAzureLinuxOSGuard() { return 1; }
  isMarinerOrAzureLinux() { return 1; }
  update-ca-trust() {
    printf 'update-ca-trust\n' > "$TEST_ROOT/update-command"
    return "$UPDATE_STATUS"
  }

  Describe 'os-release formats'
    Parameters
      AZURECONTAINERLINUX ''
      AZURELINUX AZURECONTAINERLINUX
    End
    It "uses writable anchors and native trust regeneration for $1 variant=$2"
      OS=$1 OS_VARIANT=$2
      When call configureHTTPProxyCA
      The status should be success
      The output should be blank
      The contents of file "$TEST_ROOT/etc/pki/ca-trust/source/anchors/proxyCA.crt" should equal 'proxy CA fixture'
      The contents of file "$TEST_ROOT/update-command" should equal 'update-ca-trust'
    End
  End

  It 'propagates a trust regeneration failure'
    OS=AZURELINUX OS_VARIANT=AZURECONTAINERLINUX UPDATE_STATUS=1
    When run configureHTTPProxyCA
    The status should equal 42
    The output should be blank
  End

  It 'keeps Custom CA Trust on the same writable anchor directory'
    When call grep -Fx 'ExecStart=/opt/scripts/update_certs.sh /etc/pki/ca-trust/source/anchors update-ca-trust' \
      parts/linux/cloud-init/artifacts/acl/update_certs.service
    The status should be success
    The output should include '/etc/pki/ca-trust/source/anchors update-ca-trust'
  End
End
