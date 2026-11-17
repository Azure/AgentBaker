#!/bin/bash
# shellcheck disable=SC2329

Describe 'embedded script installation'
  anc() {
    echo unpack
    return "$UNPACK_STATUS"
  }

  source() {
    echo sourced
  }

  run_unpack() {
    local commands
    case "$1" in
      *.json)
        # shellcheck disable=SC2016 # Match Packer expressions literally.
        commands=$(jq -r '.provisioners[] | select(.type == "shell") | .inline |
          if type == "array" then .[] elif type == "string" then . else empty end |
          select(contains("unpack-scripts") or contains("/opt/azure/image-build/pre-install-dependencies.sh"))' "$1" |
          sed -nE 's|^sudo FEATURE_FLAGS=\{\{user `feature_flags`\}\} ENABLE_FIPS=\{\{user `enable_fips`\}\} /home/packer/aks-node-controller |anc |p; s|^sudo FEATURE_FLAGS=.*pre-install-dependencies.sh$|source|p')
        ;;
      *)
        commands=$(awk '
          /aks-node-controller unpack-scripts/ { print }
          /^source / { print; exit }
        ' "$1" | sed -E 's|/[^ ]*/aks-node-controller |anc |')
        ;;
    esac
    set -e
    eval "$commands"
  }

  Parameters
    vhdbuilder/packer/vhd-image-builder-base.json
    vhdbuilder/packer/vhd-image-builder-cvm.json
    vhdbuilder/packer/vhd-image-builder-arm64-gen2.json
    vhdbuilder/packer/vhd-image-builder-arm64-gb.json
    vhdbuilder/packer/vhd-image-builder-mariner.json
    vhdbuilder/packer/vhd-image-builder-mariner-cvm.json
    vhdbuilder/packer/vhd-image-builder-mariner-arm64.json
    vhdbuilder/packer/vhd-image-builder-acl.json
    vhdbuilder/packer/vhd-image-builder-acl-arm64.json
    vhdbuilder/packer/imagecustomizer/azlosguard/scripts/azlosguard-postinstall.sh
  End

  It "unpacks before sourcing helpers in $1"
    UNPACK_STATUS=0
    When run run_unpack "$1"
    The status should be success
    The line 1 of output should equal unpack
    The output should end with sourced
  End

  It "stops before sourcing helpers if unpacking fails in $1"
    UNPACK_STATUS=1
    When run run_unpack "$1"
    The status should be failure
    The output should equal unpack
  End
End

  Describe 'image-build permission restoration'
    setup() {
      TEST_ROOT=$(mktemp -d)
      mkdir -p "$TEST_ROOT/etc/ssh" "$TEST_ROOT/etc/security"
      touch "$TEST_ROOT/etc/ssh/sshd_config" "$TEST_ROOT/etc/security/pwquality.conf" "$TEST_ROOT/NOTICE.txt"
      printf 'deny = 5\n' > "$TEST_ROOT/etc/security/faillock.conf"
      chmod 0644 "$TEST_ROOT/etc/security/faillock.conf"
      eval "$(sed \
        -e "s|/etc/|$TEST_ROOT/etc/|g" \
        -e "s|/NOTICE.txt|$TEST_ROOT/NOTICE.txt|g" \
        ./parts/root/shared/opt/azure/image-build/packer_source.sh)"
      export OS=UBUNTU FEATURE_FLAGS=None
    }
    cleanup() {
      rm -rf "$TEST_ROOT"
    }
    isUbuntu() { [ "$1" = UBUNTU ]; }
    install() { :; }
    run_configuration() {
      installKataPayload() { :; }
      configureImageFiles
    }
    faillock_mode() {
      stat -c '%a' "$TEST_ROOT/etc/security/faillock.conf"
    }

    BeforeEach 'setup'
    AfterEach 'cleanup'

    It 'restores 0600 when the base image already contains faillock.conf'
      When run run_configuration
      The status should be success
      The result of function faillock_mode should equal 600
    End

    It 'does not require faillock.conf on other image trees'
      rm "$TEST_ROOT/etc/security/faillock.conf"
      When run run_configuration
      The status should be success
    End

    It 'preserves a non-Ubuntu distro-owned faillock.conf mode'
      export OS=AZURELINUX
      When run run_configuration
      The status should be success
      The result of function faillock_mode should equal 644
    End
  End

Describe 'hotfix kernel normalization'
  setup() {
    TEST_ROOT=$(mktemp -d)
    cp ./parts/root/common/etc/modprobe.d/CIS.conf "$TEST_ROOT/CIS.conf"
    printf 'blacklist algif_aead\n' > "$TEST_ROOT/customer.conf"
    chmod 0600 "$TEST_ROOT/CIS.conf"
    eval "$(sed "s|/etc/modprobe.d|$TEST_ROOT|g" ./parts/root/common/opt/azure/image-build/image_config.sh)"
    export OS=UBUNTU OS_VERSION=22.04
  }
  cleanup() {
    rm -f "$TEST_ROOT/CIS.conf" "$TEST_ROOT/customer.conf"
    rmdir "$TEST_ROOT"
  }
  uname() { echo 5.15.0-1126-azure; }
  isUbuntu() { [ "$1" = UBUNTU ]; }
  mode() { stat -c '%a' "$TEST_ROOT/CIS.conf"; }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  It 'restores baked CIS bytes without changing customer files or permissions'
    When call configureVulnerableKernelModuleDenyRules "$TEST_ROOT/CIS.conf"
    The status should be success
    The output should include 'Removing algif_aead'
    The contents of file "$TEST_ROOT/CIS.conf" should not include 'algif_aead'
    The contents of file "$TEST_ROOT/CIS.conf" should not include '# CVE-2026-31431'
    The contents of file "$TEST_ROOT/CIS.conf" should include 'install dccp /bin/true'
    The contents of file "$TEST_ROOT/customer.conf" should equal 'blacklist algif_aead'
    The result of function mode should equal 600
  End

  It 'retains the build-wide cleanup when no file list is supplied'
    When call configureVulnerableKernelModuleDenyRules
    The status should be success
    The output should include 'Removing algif_aead'
    The contents of file "$TEST_ROOT/customer.conf" should equal ''
    The result of function mode should equal 600
  End
End
