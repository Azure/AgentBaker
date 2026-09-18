#!/bin/bash

# The coredns version localdns runs is pinned in two scripts that cannot share a constant:
# install-dependencies.sh sources from /home/packer during the VHD build, while
# linux-vhd-content-test.sh runs later on the baked VHD. Both pins must also name a tag that
# components.json actually caches, otherwise extractAndCacheCoreDnsBinary fails the build.
# These specs guard that three-way agreement.
Describe 'coredns pinned version'
  INSTALL_DEPENDENCIES_PATH="vhdbuilder/packer/install-dependencies.sh"
  VHD_CONTENT_TEST_PATH="vhdbuilder/packer/test/linux-vhd-content-test.sh"
  COMPONENTS_PATH="parts/common/components.json"

  # install-dependencies.sh cannot be sourced here - it sources /home/packer/* and invokes the
  # install steps at top level - so read the pin out of the file textually instead.
  coredns_pinned_version_in() {
    local script_path="$1" pinned_version

    pinned_version="$(sed -n 's/^COREDNS_VERSION="\(.*\)"$/\1/p' "${script_path}")" || return 1
    if [ -z "${pinned_version}" ]; then
      echo "No COREDNS_VERSION pin found in ${script_path}" >&2
      return 1
    fi
    if [ "$(printf '%s\n' "${pinned_version}" | wc -l | tr -d ' ')" != "1" ]; then
      echo "Expected exactly one COREDNS_VERSION pin in ${script_path}, found:" >&2
      printf '%s\n' "${pinned_version}" >&2
      return 1
    fi

    printf '%s\n' "${pinned_version}"
  }

  coredns_component_versions() {
    jq -r '
      .ContainerImages[]
      | select(.downloadURL | test("/kubernetes/coredns:"))
      | .multiArchVersionsV2[]
      | .latestVersion
    ' "${COMPONENTS_PATH}"
  }

  coredns_assert_pins_match_components() {
    local install_pin test_pin

    install_pin="$(coredns_pinned_version_in "${INSTALL_DEPENDENCIES_PATH}")" || return 1
    test_pin="$(coredns_pinned_version_in "${VHD_CONTENT_TEST_PATH}")" || return 1

    if [ "${install_pin}" != "${test_pin}" ]; then
      echo "COREDNS_VERSION pins disagree: ${INSTALL_DEPENDENCIES_PATH} has ${install_pin}, ${VHD_CONTENT_TEST_PATH} has ${test_pin}" >&2
      return 1
    fi

    if ! coredns_component_versions | grep -qxF "${install_pin}"; then
      echo "Pinned coredns version ${install_pin} is not cached by ${COMPONENTS_PATH}. Cached versions:" >&2
      coredns_component_versions >&2
      return 1
    fi
  }

  It 'is the same in install-dependencies.sh and linux-vhd-content-test.sh, and is cached by components.json'
    When call coredns_assert_pins_match_components
    The status should be success
    The output should eq ""
    The stderr should eq ""
  End

  It 'reports a script that has lost its pin'
    When run coredns_pinned_version_in "${COMPONENTS_PATH}"
    The status should equal 1
    The output should eq ""
    The stderr should include "No COREDNS_VERSION pin found in ${COMPONENTS_PATH}"
  End
End
