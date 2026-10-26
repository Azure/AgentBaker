#!/bin/bash
# shellcheck disable=SC2329,SC2317

Describe 'AIB deprovision hook installation'
  setup() {
    TEST_ROOT="$(mktemp -d)"
    mkdir -p "${TEST_ROOT}/tmp" "${TEST_ROOT}/opt/azure/containers"
    printf '%s\n' 'original AIB hook' > "${TEST_ROOT}/tmp/DeprovisioningScript.sh"
    chmod 0444 "${TEST_ROOT}/tmp/DeprovisioningScript.sh"
    ln "${TEST_ROOT}/tmp/DeprovisioningScript.sh" "${TEST_ROOT}/original-hook"
    chmod 1777 "${TEST_ROOT}/tmp"
    if [ "$(id -u)" -eq 0 ]; then
      chown 65534:65534 "${TEST_ROOT}/tmp/DeprovisioningScript.sh"
    fi
    touch "${TEST_ROOT}/tmp/post-deprovision-walinuxagent.sh" \
      "${TEST_ROOT}/tmp/install_walinuxagent.py"
    export FAIL_RENAME=false
  }

  cleanup() {
    rm -rf -- "${TEST_ROOT}"
  }

  sudo() {
    if [ "$1" = "mv" ] && [ "${FAIL_RENAME}" = "true" ]; then
      echo "mock hook replacement failed" >&2
      return 42
    fi
    "$@"
  }

  install_hook() {
    local inline status=0
    inline="$(jq -r '.properties.customize[] | select(.name == "InstallDeprovisionHook") | .inline[]' \
      vhdbuilder/prefetch/templates/optimize.json)"
    inline="${inline//\/tmp\//${TEST_ROOT}/tmp/}"
    inline="${inline//\/opt\/azure\/containers\//${TEST_ROOT}/opt/azure/containers/}"
    inline="${inline//<CVM_BUILD>/$1}"
    export -f sudo
    printf '%s\n' "${inline}" | bash -eu || status=$?
    find "${TEST_ROOT}/tmp" -name 'DeprovisioningScript.??????' -print
    return "${status}"
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  Context 'CVM and non-CVM builds'
    Parameters
      true
      false
    End

    It "replaces the existing hook without truncating its inode (CVM=$1)"
      When call install_hook "$1"
      The status should be success
      The stdout should be blank
      The contents of file "${TEST_ROOT}/original-hook" should equal 'original AIB hook'
      The contents of file "${TEST_ROOT}/tmp/DeprovisioningScript.sh" should equal \
        "$(printf '%s\n' '#!/bin/bash -eu' "exec sudo /bin/bash -eu ${TEST_ROOT}/tmp/agentbaker-deprovision.sh $1")"
      The path "${TEST_ROOT}/tmp/DeprovisioningScript.sh" should be executable
    End
  End

  It 'preserves the original hook and cleans up staging files if replacement fails'
    FAIL_RENAME=true
    When call install_hook false
    The status should equal 42
    The stdout should be blank
    The stderr should include 'mock hook replacement failed'
    The contents of file "${TEST_ROOT}/tmp/DeprovisioningScript.sh" should equal 'original AIB hook'
  End
End

Describe 'AIB deprovision hook execution'
  Include ./vhdbuilder/packer/aib-deprovision.sh

  setup() {
    CALLS="$(mktemp)"
    WAAGENT_STATUS=0
  }

  cleanup() {
    command rm -f -- "${CALLS}"
  }

  sudo() {
    printf '%s\n' "$*" >> "${CALLS}"
    if [ "$1" != "waagent" ]; then
      echo "Expected waagent lookup through PATH, not $1" >&2
      return 99
    fi
    return "${WAAGENT_STATUS}"
  }

  installPinnedWALinuxAgent() {
    echo 'pin agent' >> "${CALLS}"
  }

  cleanupNvPCRCredentials() {
    echo "credentials $*" >> "${CALLS}"
  }

  rm() {
    echo 'remove scripts' >> "${CALLS}"
  }

  sync() {
    echo 'sync' >> "${CALLS}"
  }

  BeforeEach 'setup'
  AfterEach 'cleanup'

  Context 'CVM and non-CVM builds'
    Parameters
      true
      false
    End

    It "uses PATH for deprovisioning before agent pinning and CVM cleanup (CVM=$1)"
      When call main "$1"
      The status should be success
      The contents of file "${CALLS}" should equal "$(printf '%s\n' \
        'waagent -force -deprovision+user' 'pin agent' "credentials $1 /" 'remove scripts' 'sync')"
    End
  End

  It 'propagates deprovisioning failure without pinning or deleting scripts'
    WAAGENT_STATUS=42
    When call main false
    The status should equal 42
    The contents of file "${CALLS}" should equal 'waagent -force -deprovision+user'
  End
End
