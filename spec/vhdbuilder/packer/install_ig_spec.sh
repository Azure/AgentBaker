#!/bin/bash

Describe 'ig_extract_upstream_version function'
  Include './vhdbuilder/packer/install-ig.sh'

  It 'returns the upstream version on success'
    When call ig_extract_upstream_version "0.51.0-4.azl3"
    The status should be success
    The output should eq "0.51.0"
    The stderr should eq ""
  End

  It 'writes parse failures to stderr'
    When run ig_extract_upstream_version "not-a-version"
    The status should equal 1
    The output should eq ""
    The stderr should include "[ig] Could not parse upstream version from 'not-a-version'"
  End
End

Describe 'ig-gadgets pinned versions'
  Include './vhdbuilder/packer/install-ig.sh'

  ig_component_upstream_versions() {
    local current_version ig_versions

    ig_versions="$(jq -r '
      .Packages[]
      | select(.name == "inspektor-gadget")
      | .downloadURIs
      | ..
      | objects
      | select((.renovateTag? // "") | test("(^|, )name=ig,"))
      | .latestVersion
    ' parts/common/components.json)" || return 1

    while IFS= read -r current_version; do
      ig_extract_upstream_version "${current_version}" || return 1
    done <<EOF | sort -u
${ig_versions}
EOF
  }

  ig_assert_gadget_versions_match_components() {
    local ig_upstream_versions gadget_upstream_versions all_upstream_versions version_count deb_upstream rpm_upstream

    ig_upstream_versions="$(ig_component_upstream_versions)" || return 1
    if [ -z "${ig_upstream_versions}" ]; then
      echo "No inspektor-gadget component versions found" >&2
      return 1
    fi

    deb_upstream="$(ig_extract_upstream_version "${IG_GADGETS_DEB_VERSION}")" || return 1
    rpm_upstream="$(ig_extract_upstream_version "${IG_GADGETS_RPM_VERSION}")" || return 1
    gadget_upstream_versions="$(printf '%s\n%s\n' "${deb_upstream}" "${rpm_upstream}" | sort -u)"

    all_upstream_versions="$(printf '%s\n%s\n' "${ig_upstream_versions}" "${gadget_upstream_versions}" | sort -u)"
    version_count="$(printf '%s\n' "${all_upstream_versions}" | wc -l | tr -d ' ')"
    if [ "${version_count}" != "1" ]; then
      echo "Expected ig and ig-gadgets to share one upstream version, found:" >&2
      printf '%s\n' "${all_upstream_versions}" >&2
      return 1
    fi
  }

  It 'matches every ig upstream version in components.json'
    When call ig_assert_gadget_versions_match_components
    The status should be success
    The output should eq ""
    The stderr should eq ""
  End
End
