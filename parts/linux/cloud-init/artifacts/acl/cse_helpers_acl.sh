#!/bin/bash

echo "Sourcing cse_helpers_distro.sh for ACL"

stub() {
    echo "${FUNCNAME[1]} stub"
}

dnfversionlockWALinuxAgent() {
    stub
}

holdWALinuxAgent() {
    stub
}

apt_get_update() {
    stub
}

apt_get_dist_upgrade() {
    stub
}

# Temporary provisioning-only guard until EROFS supports artifact streaming.
# Fail only for detected IPE conflicts; unavailable checks warn and continue.
# Later IPE enablement is not covered.
validateACLArtifactStreamingSecurityProfile() {
  local profile_cache="${ARTIFACT_STREAMING_SECURITY_PROFILE_CACHE:-/run/acl/node-security-profile}"
  if [ ! -e "$profile_cache" ]; then
    echo "WARNING: No cached ACL security profile; skipping the IPE tag check." >&2
    return 0
  fi

  local profile pair key mode
  local -a pairs
  if ! profile="$(cat "$profile_cache")"; then
    echo "WARNING: Cannot read ${profile_cache}; skipping the IPE tag check." >&2
    return 0
  fi
  # Match ACL's initrd parser: comma-separated pairs, first matching key wins.
  IFS=',' read -ra pairs <<< "$profile"
  for pair in "${pairs[@]}"; do
    IFS='=' read -r key mode <<< "$pair"
    [ "$key" = "ipe" ] || continue
    case "$mode" in
      audit|enforcing)
        echo "Artifact streaming requires OverlayBD, but the boot-time tag requests acl-node-security-profile=ipe=${mode}. Disable artifact streaming or set ipe=disabled in acl-node-security-profile (preserving other entries), then reboot before provisioning." >&2
        return 1
        ;;
      disabled|off|"") ;;
      *)
        echo "WARNING: Unrecognized cached IPE mode '${mode}'; skipping the IPE tag check." >&2
        ;;
    esac
    break
  done
  return 0
}

validateACLArtifactStreamingIPECompatibility() {
  validateACLArtifactStreamingSecurityProfile || return 1

  local securityfs_root="${ARTIFACT_STREAMING_SECURITYFS_ROOT:-/sys/kernel/security}"
  if ! findmnt -n -t securityfs --mountpoint "$securityfs_root" >/dev/null ||
      [ ! -r "$securityfs_root" ] || [ ! -x "$securityfs_root" ]; then
    echo "WARNING: Cannot inspect securityfs at ${securityfs_root}; skipping the kernel IPE check." >&2
    return 0
  fi
  [ -d "${securityfs_root}/ipe" ] || return 0

  local policies_dir="${securityfs_root}/ipe/policies"
  if [ ! -d "$policies_dir" ] || [ ! -r "$policies_dir" ] || [ ! -x "$policies_dir" ]; then
    echo "WARNING: Cannot enumerate ${policies_dir}; skipping the kernel IPE check." >&2
    return 0
  fi
  local policy active
  # Audit mode still selects EROFS. Include policies with dot-prefixed names.
  for policy in "$policies_dir"/* "$policies_dir"/.[!.]* "$policies_dir"/..?*; do
    [ -e "$policy" ] || continue
    if ! active="$(cat "${policy}/active")"; then
      echo "WARNING: Cannot read ${policy}/active; skipping this IPE policy." >&2
      continue
    fi
    case "$active" in
      0) ;;
      1)
        echo "Artifact streaming requires OverlayBD, but ACL IPE is active and requires EROFS (for example, enabled with acl-node-security-profile=ipe=audit). Disable artifact streaming or set ipe=disabled in acl-node-security-profile (preserving other entries), then reboot before provisioning." >&2
        return 1
        ;;
      *)
        echo "WARNING: Invalid IPE policy state '${active}' in ${policy}/active; skipping this IPE policy." >&2
        ;;
    esac
  done
  return 0
}
