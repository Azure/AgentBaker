#!/bin/bash
# shellcheck disable=SC2329

Describe 'Prefetch optimization handoff'
  resolve_optimization() {
    export BUILDER="$1" OS_SKU="$2" ARCHITECTURE="$3" FEATURE_FLAGS="$4"
    export GENERATE_PUBLISHING_INFO="$5" RUN_PREFETCH_OPTIMIZATION="$6"
    if [ "$5" = unset ]; then unset GENERATE_PUBLISHING_INFO; fi
    local script output compatible effective
    script=$(awk '
      /^  - bash: \|$/ { script = ""; collecting = 1; next }
      collecting && /^      / { script = script substr($0, 7) "\n"; next }
      /^    displayName: Determine Prefetch Optimization compatibility$/ { print script; found = 1; exit }
      /^    displayName: Build VHD$/ { exit }
      END { if (!found) exit 1 }
    ' .pipelines/templates/.builder-release-template.yaml) || return $?
    output=$(bash -eu -c "$script") || return $?
    compatible=$(printf '%s\n' "$output" | sed -n 's/^##vso\[task.setvariable variable=PREFETCH_COMPATIBLE\]//p')
    effective=$(printf '%s\n' "$output" | sed -n 's/^##vso\[task.setvariable variable=RUN_PREFETCH_OPTIMIZATION\]//p')
    printf 'compatible=%s optimization=%s\n' "$compatible" "${effective:-$RUN_PREFETCH_OPTIMIZATION}"
  }

  Parameters
    packer Ubuntu ARM64 cvm True True False False
    packer Ubuntu X86_64 cvm True True True True
    packer Ubuntu ARM64 None True True True True
    packer AzureLinux ARM64 cvm True True False False
    imagecustomizer AzureLinuxOSGuard X86_64 None True True False False
    packer AzureContainerLinux X86_64 None True True False False
    packer Ubuntu X86_64 None False True True False
    packer Ubuntu X86_64 None unset True True False
    packer Ubuntu X86_64 None True False True False
    packer Ubuntu arm64 cvm true true False False
    packer Ubuntu X86_64 None true true True true
  End

  It "resolves $1/$2/$3/$4 publishing=$5 requested=$6 before Packer"
    When call resolve_optimization "$1" "$2" "$3" "$4" "$5" "$6"
    The status should be success
    The output should equal "compatible=$7 optimization=$8"
  End
End

Describe 'AIB deprovision hook installation'
  setup() {
    TEST_ROOT=$(mktemp -d "${SHELLSPEC_WORKDIR}/deprovision-hook.XXXXXX")
    export HOOK_PATH="${TEST_ROOT}/DeprovisioningScript.sh" INSTALL_STATUS=0
  }
  BeforeEach 'setup'

  Mock sudo
    if [ "$1" = install ] && [ "$5" = /tmp/DeprovisioningScript.sh ]; then
      [ "$INSTALL_STATUS" -eq 0 ] || exit "$INSTALL_STATUS"
      command install "$2" "$3" "$4" "$HOOK_PATH"
    elif [ "$1" = /bin/bash ]; then
      printf '%s\n' "$*"
    else
      echo "Unexpected sudo invocation: $*" >&2
      exit 1
    fi
  End

  install_hook() {
    local script
    script=$(jq -r --arg cvm "$1" '
      .properties.customize[] | select(.name == "InstallDeprovisionHook") |
      .inline | join("\n") | gsub("<CVM_BUILD>"; $cvm)
    ' vhdbuilder/prefetch/templates/optimize.json) || return $?
    bash -eu -c "$script"
  }

  run_hook() {
    install_hook "$1" || return $?
    "$HOOK_PATH"
  }

  Describe 'CVM argument forwarding'
    Parameters
      true
      false
    End

    It "installs an executable hook forwarding CVM=$1"
      When call run_hook "$1"
      The status should be success
      The output should equal "/bin/bash -eu /opt/azure/containers/deprovision.sh $1"
      The value "$(stat -c %a "$HOOK_PATH")" should equal 755
    End
  End

  It 'replaces a destination symlink without modifying its target'
    printf 'untouched\n' > "${TEST_ROOT}/target"
    ln -s "${TEST_ROOT}/target" "$HOOK_PATH"

    When call run_hook true
    The status should be success
    The output should equal '/bin/bash -eu /opt/azure/containers/deprovision.sh true'
    The path "$HOOK_PATH" should not be symlink
    The contents of file "${TEST_ROOT}/target" should equal untouched
  End

  It 'propagates installation failure'
    INSTALL_STATUS=7
    When call install_hook false
    The status should equal 7
    The path "$HOOK_PATH" should not be exist
  End
End
