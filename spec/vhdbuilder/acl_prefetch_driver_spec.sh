#!/bin/bash

Describe 'ACL workload privilege boundary'
  Skip if 'already running as root' [ "${EUID}" -eq 0 ]

  It 'reexecutes the workload with noninteractive sudo'
    Mock sudo
      printf '%s\n' "$*"
    End
    When run bash ./vhdbuilder/scripts/acl-prefetch-driver.sh
    The status should be success
    The output should equal '-n /bin/bash ./vhdbuilder/scripts/acl-prefetch-driver.sh'
  End

  It 'preserves deprovision arguments across the privilege boundary'
    Mock sudo
      printf '%s\n' "$*"
    End
    When run bash ./vhdbuilder/scripts/acl-prefetch-driver.sh --deprovision
    The status should be success
    The output should include '--deprovision'
  End

  It 'propagates denied sudo'
    Mock sudo
      echo 'sudo unavailable' >&2
      exit 23
    End
    When run bash ./vhdbuilder/scripts/acl-prefetch-driver.sh
    The status should equal 23
    The stderr should equal 'sudo unavailable'
  End
End

Describe 'ACL phase selection'
  Include ./vhdbuilder/scripts/acl-prefetch-driver.sh

  It 'defaults to workload execution'
    When call aclPrefetchPhase
    The status should be success
    The output should equal workload
  End

  It 'selects deprovisioning explicitly'
    When call aclPrefetchPhase --deprovision
    The status should be success
    The output should equal deprovision
  End

  It 'rejects obsolete experimental arguments'
    When call aclPrefetchPhase --prepare-only
    The status should equal 2
    The stderr should include Usage
  End

  It 'rejects extra arguments'
    When call aclPrefetchPhase --deprovision extra
    The status should equal 2
    The stderr should include Usage
  End
End

Describe 'ACL deprovision and workload execution'
  Include ./vhdbuilder/scripts/acl-prefetch-driver.sh

  setup_phase() {
    WORK="$(mktemp -d)"
    PREFETCH_SCRIPT="${WORK}/prefetch.sh"
    CLEANUP_SCRIPT="${WORK}/cleanup.sh"
    TRACE="${WORK}/trace"
    export TRACE
    printf 'printf "warm\\n" >> "$TRACE"\n' > "${PREFETCH_SCRIPT}"
    printf 'printf "cleanup\\n" >> "$TRACE"\n' > "${CLEANUP_SCRIPT}"
    systemctl() { printf 'systemctl %s\n' "$*" >> "${TRACE}"; }
    acl-image-prepare() { printf 'prepare\n' >> "${TRACE}"; }
    waagent() { printf 'waagent %s\n' "$*" >> "${TRACE}"; }
    sync() { printf 'sync\n' >> "${TRACE}"; }
  }
  cleanup_phase() { rm -rf "${WORK}"; }
  BeforeEach 'setup_phase'
  AfterEach 'cleanup_phase'

  It 'cleans and prepares the image before normal Azure deprovisioning'
    expected="$(printf 'cleanup\nprepare\nwaagent -force -deprovision+user\nsync')"
    When call runACLPhase deprovision "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should be success
    The contents of file "${TRACE}" should equal "${expected}"
    The path "${PREFETCH_SCRIPT}" should be exist
    The path "${CLEANUP_SCRIPT}" should not be exist
  End

  It 'profiles without relying on workload-phase cleanup or first-boot edits'
    rm "${CLEANUP_SCRIPT}"
    expected="$(printf 'systemctl start containerd\nwarm')"
    When call runACLPhase workload "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should be success
    The contents of file "${TRACE}" should equal "${expected}"
    The path "${PREFETCH_SCRIPT}" should be exist
  End

  It 'fails before cleanup when the ACL package prerequisite is missing'
    unset -f acl-image-prepare
    When call runACLPhase deprovision "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should be failure
    The stderr should include 'base image containing acl-image-prepare'
    The path "${TRACE}" should not be exist
    The path "${CLEANUP_SCRIPT}" should be exist
  End

  It 'propagates workload failure'
    printf 'exit 19\n' > "${PREFETCH_SCRIPT}"
    When call runACLPhase workload "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should equal 19
  End

  It 'does not prepare after cleanup fails'
    printf 'exit 17\n' > "${CLEANUP_SCRIPT}"
    When call runACLPhase deprovision "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should equal 17
    The path "${TRACE}" should not be exist
  End

  It 'does not deprovision after ACL preparation fails'
    acl-image-prepare() { return 18; }
    When call runACLPhase deprovision "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should equal 18
    The contents of file "${TRACE}" should equal cleanup
    The path "${CLEANUP_SCRIPT}" should be exist
  End

  It 'does not remove cleanup after waagent fails'
    waagent() { return 20; }
    expected="$(printf 'cleanup\nprepare')"
    When call runACLPhase deprovision "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should equal 20
    The contents of file "${TRACE}" should equal "${expected}"
    The path "${CLEANUP_SCRIPT}" should be exist
  End

  It 'propagates final flush failure'
    sync() { return 21; }
    When call runACLPhase deprovision "${PREFETCH_SCRIPT}" "${CLEANUP_SCRIPT}"
    The status should equal 21
  End
End

Describe 'ACL release AIB template'
  run_configuration() {
    local work
    work="$(mktemp -d)"
    (
      export ACL_HOOK_TEST_ROOT="${work}"
      acl-image-prepare() { return 0; }
      sudo() {
        if [[ "$*" != 'tee /tmp/DeprovisioningScript.sh' ]]; then
          echo "unexpected command: $*" >&2
          return 1
        fi
        cat > "${ACL_HOOK_TEST_ROOT}/DeprovisioningScript.sh"
      }
      export -f acl-image-prepare sudo
      jq -r '.properties.customize[1].inline[]' vhdbuilder/prefetch/templates/acl.json |
        bash -euo pipefail
      cat "${work}/DeprovisioningScript.sh"
    )
    local status=$?
    rm -rf "${work}"
    return "${status}"
  }
  validate_template() {
    jq -e '
      .properties.buildTimeoutInMinutes == 60 and
      (.properties.customize | length) == 2 and
      .properties.customize[0].type == "File" and
      .properties.customize[0].sourceUri == .properties.optimize.workload.scriptUri and
      .properties.customize[0].destination == "/tmp/acl-prefetch-driver.sh" and
      .properties.customize[1].type == "Shell" and
      (.properties.customize[1].inline[0] | contains("command -v acl-image-prepare")) and
      (.properties.customize[1].inline[1] | contains("/tmp/DeprovisioningScript.sh")) and
      (.properties.customize[1].inline[1] | contains("--deprovision")) and
      (.properties | has("vmProfile") | not) and
      .properties.optimize.vmBoot.state == "Enabled" and
      .properties.optimize.workload.state == "Enabled"
    ' vhdbuilder/prefetch/templates/acl.json
  }
  It 'uses the supported generalization override without pilot-specific hardware'
    When call validate_template
    The status should be success
    The output should equal true
  End

  It 'writes an executable shell program for the documented override'
    expected="$(printf '%s\n' '#!/bin/bash' 'set -euo pipefail' 'sudo /bin/bash /tmp/acl-prefetch-driver.sh --deprovision')"
    When call run_configuration
    The status should be success
    The output should equal "${expected}"
  End
End
