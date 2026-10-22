#!/bin/bash
# shellcheck disable=SC2329

Describe 'ACL certificate refresh'
  setup() {
    TEST_ROOT=$(mktemp -d)
    PAYLOAD="$TEST_ROOT/home/packer/acl-ca-certificates"
    mkdir -p "$PAYLOAD" "$TEST_ROOT/usr/share/p11-kit/modules"
    printf 'new roots\n' > "$PAYLOAD/ca-bundle.trust.microsoft.p11-kit"
    printf 'new blocklist\n' > "$PAYLOAD/ca-bundle.trust.distrusted.p11-kit"
    printf '3.0.0-19.azl3\n' > "$PAYLOAD/version"
    printf 'module: p11-kit-trust.so\nx-init-reserved:\n' > "$TEST_ROOT/usr/share/p11-kit/modules/p11-kit-trust.module"
    mkdir -p "$TEST_ROOT/opt/aks-sysext/usr/lib/extension-release.d" "$TEST_ROOT/etc/extensions"
    printf 'ID=azurelinux\nSYSEXT_LEVEL=1.0\n' > "$TEST_ROOT/opt/aks-sysext/usr/lib/extension-release.d/extension-release.aks-sysext"
    ln -s /opt/aks-sysext "$TEST_ROOT/etc/extensions/aks-sysext"
    printf 'existing content\n' > "$TEST_ROOT/opt/aks-sysext/existing"
    eval "$(sed -n '/^refreshACLCAStore()/,/^}/p' vhdbuilder/scripts/linux/acl/tool_installs_acl.sh |
      sed "s|local payload=/home/packer|local payload=$TEST_ROOT/home/packer|; s|local extension_dir=/opt/|local extension_dir=$TEST_ROOT/opt/|; s|local active_source=/usr/|local active_source=$TEST_ROOT/usr/|; s|readlink /etc/extensions|readlink $TEST_ROOT/etc/extensions|; s|/etc/pki/ca-trust/extracted|$TEST_ROOT/etc/pki/ca-trust/extracted|g")"
  }
  cleanup() {
    rm -rf "$TEST_ROOT"
  }
  BeforeEach 'setup'
  AfterEach 'cleanup'

  sh() {
    [ "$1" = -e ] || return 1
    local bundle
    for bundle in pem/tls-ca-bundle.pem pem/email-ca-bundle.pem pem/objsign-ca-bundle.pem \
        openssl/ca-bundle.trust.crt java/cacerts edk2/cacerts.bin; do
      mkdir -p "$(dirname "$TEST_ROOT/etc/pki/ca-trust/extracted/$bundle")"
      : > "$TEST_ROOT/etc/pki/ca-trust/extracted/$bundle"
      if [ "$bundle" != pem/objsign-ca-bundle.pem ]; then
        printf 'regenerated\n' > "$TEST_ROOT/etc/pki/ca-trust/extracted/$bundle"
      fi
    done
  }

  systemd-sysext() {
    [ "$*" = '--no-reload refresh' ] || return 1
    mkdir -p "$TEST_ROOT/usr/share/pki/ca-trust-source"
    cp "$TEST_ROOT/opt/aks-sysext/usr/share/pki/ca-trust-source/"*.p11-kit "$TEST_ROOT/usr/share/pki/ca-trust-source/"
  }
  systemctl() {
    [ "$*" = 'enable systemd-sysext.service' ]
  }

  It 'reuses /opt/aks-sysext and preserves its metadata, other content and local anchors'
    mkdir -p "$TEST_ROOT/etc/pki/ca-trust/source/anchors"
    printf 'custom CA\n' > "$TEST_ROOT/etc/pki/ca-trust/source/anchors/custom.crt"
    When call refreshACLCAStore
    The status should be success
    The contents of file "$TEST_ROOT/opt/aks-sysext/ca-certificates.version" should equal '3.0.0-19.azl3'
    The contents of file "$TEST_ROOT/opt/aks-sysext/usr/share/pki/ca-trust-source/ca-bundle.trust.microsoft.p11-kit" should equal 'new roots'
    The contents of file "$TEST_ROOT/opt/aks-sysext/usr/share/pki/ca-trust-source/ca-bundle.trust.distrusted.p11-kit" should equal 'new blocklist'
    The contents of file "$TEST_ROOT/opt/aks-sysext/existing" should equal 'existing content'
    The contents of file "$TEST_ROOT/etc/pki/ca-trust/source/anchors/custom.crt" should equal 'custom CA'
    The contents of file "$TEST_ROOT/usr/share/p11-kit/modules/p11-kit-trust.module" should equal "module: p11-kit-trust.so
x-init-reserved:"
    The contents of file "$TEST_ROOT/opt/aks-sysext/usr/lib/extension-release.d/extension-release.aks-sysext" should equal "ID=azurelinux
SYSEXT_LEVEL=1.0"
  End

  It 'fails rather than accepting missing trust data'
    rm "$PAYLOAD/ca-bundle.trust.microsoft.p11-kit"
    When call refreshACLCAStore
    The status should be failure
    The stderr should include 'Missing refreshed ACL trust source'
    The path "$TEST_ROOT/opt/aks-sysext/ca-certificates.version" should not be exist
  End

  It 'fails if a trust extraction command fails'
    sh() { return 1; }
    When call refreshACLCAStore
    The status should be failure
    The path "$TEST_ROOT/opt/aks-sysext/ca-certificates.version" should not be exist
  End

  It 'fails when the sysext cannot be merged'
    systemd-sysext() { return 1; }
    When call refreshACLCAStore
    The status should be failure
    The stderr should include 'Failed to merge refreshed ACL trust sources'
  End

  It 'fails when the sysext boot service cannot be enabled'
    systemctl() { return 1; }
    When call refreshACLCAStore
    The status should be failure
    The path "$TEST_ROOT/opt/aks-sysext/ca-certificates.version" should not be exist
  End

  It 'fails rather than replacing missing shared sysext metadata'
    rm "$TEST_ROOT/opt/aks-sysext/usr/lib/extension-release.d/extension-release.aks-sysext"
    When call refreshACLCAStore
    The status should be failure
    The stderr should include 'Missing existing ACL aks-sysext configuration'
  End

  It 'fails if sysext reports success without activating the refreshed data'
    systemd-sysext() { return 0; }
    When call refreshACLCAStore
    The status should be failure
    The stderr should include 'Refreshed ACL trust source is not active'
  End

  It 'rejects missing generated formats'
    sh() { return 0; }
    When call refreshACLCAStore
    The status should be failure
    The stderr should include 'Missing refreshed ACL trust bundle'
  End

  Describe 'host preparation'
    load_preparer() {
      eval "$(sed -n '/^prepareACLTrustSources()/,/^}/p' vhdbuilder/packer/prepare-acl-ca-certificates.sh)"
    }
    BeforeEach 'load_preparer'

    It 'rejects an unsupported architecture before downloading'
      When call prepareACLTrustSources RISCV64 "$TEST_ROOT/prepared"
      The status should be failure
      The stderr should include 'Unsupported ACL certificate architecture'
    End

    retrycmd_curl_file() {
      printf 'download\n' >> "$TEST_ROOT/downloads"
      [ -f "$4" ] || printf 'corrupt RPM\n' > "$4"
    }
    sleep() { :; }

    It 'rejects a corrupt RPM before extracting or writing a version marker'
      When call prepareACLTrustSources X86_64 "$TEST_ROOT/prepared"
      The status should be failure
      The output should include 'FAILED'
      The stderr should include 'checksum did NOT match'
      The stderr should include 'Failed to download and verify ACL package'
      The contents of file "$TEST_ROOT/downloads" should equal "download
download
download
download
download"
      The path "$TEST_ROOT/prepared/ca-certificates.rpm" should not be exist
      The path "$TEST_ROOT/prepared/version" should not be exist
    End

    rpm2cpio() { cat "$1"; }
    cpio() {
      cat >/dev/null
      mkdir -p usr/share/pki/ca-trust-source
      printf 'new roots\n' > usr/share/pki/ca-trust-source/ca-bundle.trust.microsoft.p11-kit
      printf 'new blocklist\n' > usr/share/pki/ca-trust-source/ca-bundle.trust.distrusted.p11-kit
    }
    sha256sum() {
      local checksum file
      read -r checksum file
      if grep -qx 'complete RPM' "$file"; then
        return 0
      fi
      printf 'checksum did NOT match\n' >&2
      return 1
    }

    It 'discards a poisoned cache and retries download plus verification'
      mkdir -p "$TEST_ROOT/prepared"
      printf 'truncated RPM\n' > "$TEST_ROOT/prepared/ca-certificates.rpm"
      retrycmd_curl_file() {
        [ -f "$4" ] || printf 'complete RPM\n' > "$4"
      }
      When call prepareACLTrustSources X86_64 "$TEST_ROOT/prepared"
      The status should be success
      The stderr should include 'checksum did NOT match'
      The contents of file "$TEST_ROOT/prepared/version" should equal '3.0.0-19.azl3'
      The contents of file "$TEST_ROOT/prepared/ca-bundle.trust.microsoft.p11-kit" should equal 'new roots'
      The path "$TEST_ROOT/prepared/ca-certificates.rpm" should not be exist
    End

    It 'removes a partial file after transport failure and downloads it again'
      retrycmd_curl_file() {
        if [ ! -f "$TEST_ROOT/transport-failed" ]; then
          printf 'truncated RPM\n' > "$4"
          touch "$TEST_ROOT/transport-failed"
          return 1
        fi
        [ ! -f "$4" ] || return 1
        printf 'complete RPM\n' > "$4"
      }
      When call prepareACLTrustSources X86_64 "$TEST_ROOT/prepared"
      The status should be success
      The contents of file "$TEST_ROOT/prepared/version" should equal '3.0.0-19.azl3'
    End
  End

  Describe 'Packer wiring'
    Parameters
      acl
      acl-arm64
    End
    It "uploads trust sources for $1"
      When call jq -e '(.provisioners[0].inline | index("mkdir -p /home/packer/acl-ca-certificates") != null) and ([.provisioners[] | select(.source == "vhdbuilder/packer/acl-ca-certificates/" and .destination == "/home/packer/acl-ca-certificates/")] | length == 1)' "vhdbuilder/packer/vhd-image-builder-$1.json"
      The status should be success
      The output should equal true
    End
  End

  Describe 'incident certificate rejection'
    load_rejection_test() {
      eval "$(sed -n '/^testACLRejectedCertificates()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)"
      CURL_STATUS=60
      CURL_ERROR='curl: (60) SSL certificate problem: certificate rejected'
    }
    BeforeEach 'load_rejection_test'
    err() { printf '%s\n' "$*" >&2; }
    curl() {
      printf '%s\n' "${@: -1}" >> "$TEST_ROOT/probes"
      printf '%s\n' "$CURL_ERROR" >&2
      return "$CURL_STATUS"
    }

    It 'requires explicit rejection at both incident endpoints'
      When call testACLRejectedCertificates
      The status should be success
      The output should be blank
      The contents of file "$TEST_ROOT/probes" should equal "https://dv-g3b-emudhra.we-have-a-problem.com/
https://dv-g3b-aaa-emudhra.we-have-a-problem.com/"
    End

    It 'rejects a successful TLS connection'
      CURL_STATUS=0 CURL_ERROR=
      When call testACLRejectedCertificates
      The status should be failure
      The stderr should include 'Expected certificate rejection'
    End

    It 'does not count a network timeout as certificate rejection'
      CURL_STATUS=28 CURL_ERROR='curl: (28) Connection timed out'
      When call testACLRejectedCertificates
      The status should be failure
      The stderr should include 'curl exited 28'
    End

    It 'does not count an unrelated certificate validation error as rejection'
      CURL_ERROR='curl: (60) SSL certificate problem: unable to get local issuer certificate'
      When call testACLRejectedCertificates
      The status should be failure
      The stderr should include 'Expected certificate rejection'
    End

    It 'fails if only the first incident endpoint is rejected'
      curl() {
        local url="${*: -1}"
        if [ "$url" = 'https://dv-g3b-emudhra.we-have-a-problem.com/' ]; then
          printf '%s\n' "$CURL_ERROR" >&2
          return 60
        fi
        return 0
      }
      When call testACLRejectedCertificates
      The status should be failure
      The stderr should include 'dv-g3b-aaa-emudhra.we-have-a-problem.com'
    End
  End

  Describe 'non-ACL VHD validation'
    Parameters
      'UBUNTU|'
      'AZURELINUX|'
      'FLATCAR|'
      'AZURELINUX|OSGUARD'
    End
    It "leaves $1 unchanged"
      test_os=$1
      ACL_OS_NAME=AZURECONTAINERLINUX
      AZURELINUX_OS_NAME=AZURELINUX
      ACL_OS_VARIANT=AZURECONTAINERLINUX
      getCurrentPackageTestOS() { printf '%s\n' "$test_os"; }
      eval "$(sed -n '/^isACL()/,/^}/p' parts/linux/cloud-init/artifacts/cse_helpers.sh)"
      eval "$(sed -n '/^testACLCAStore()/,/^}/p' vhdbuilder/packer/test/linux-vhd-content-test.sh)"
      When call testACLCAStore
      The status should be success
      The output should be blank
    End
  End
End
