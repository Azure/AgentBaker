#!/usr/bin/env shellspec

Describe 'ACL extension capture'
  Include vhdbuilder/packer/acl-extensions.sh

  setup() {
    TEST_ROOT="${SHELLSPEC_TMPBASE}/acl-extensions"
    mkdir -p "$TEST_ROOT"
  }
  BeforeEach 'setup'

  validate_release() {
    local hierarchy=$1 name=$2 relative=$3
    writeExtensionRelease "$TEST_ROOT/$name" "$hierarchy" "$name"
    (
      # shellcheck disable=SC1091
      source /usr/lib/os-release
      grep -Fx "ID=$ID" "$TEST_ROOT/$name/$relative" &&
        grep -Fx "VERSION_ID=$VERSION_ID" "$TEST_ROOT/$name/$relative" &&
        grep -Fx 'EXTENSION_RELOAD_MANAGER=1' "$TEST_ROOT/$name/$relative" &&
        test -d "$TEST_ROOT/$name/$hierarchy"
    ) >/dev/null
  }

  It 'creates sysext metadata with the image name and base OS identity'
    When call validate_release opt aks-opt usr/lib/extension-release.d/extension-release.aks-opt
    The status should be success
    The output should be blank
  End

  It 'creates confext metadata under etc'
    When call validate_release etc aks-etc etc/extension-release.d/extension-release.aks-etc
    The status should be success
    The output should be blank
  End

  validate_image() {
    mkdir -p "$TEST_ROOT/image/opt"
    printf 'payload\n' > "$TEST_ROOT/image/opt/content"
    chmod 751 "$TEST_ROOT/image/opt/content"
    ln -s content "$TEST_ROOT/image/opt/link"
    writeExtensionRelease "$TEST_ROOT/image" opt aks-opt
    buildExtensionImage "$TEST_ROOT/image" "$TEST_ROOT/aks-opt.raw" &&
      debugfs -R 'cat /opt/content' "$TEST_ROOT/aks-opt.raw" 2>/dev/null &&
      debugfs -R 'stat /opt/link' "$TEST_ROOT/aks-opt.raw" 2>/dev/null &&
      debugfs -R 'stat /opt/content' "$TEST_ROOT/aks-opt.raw" 2>/dev/null
  }

  It 'creates a raw filesystem preserving content, symlinks, and modes'
    When call validate_image
    The status should be success
    The output should include 'payload'
    The output should include 'Type: symlink'
    The output should include '0751'
    The output should include 'Fast link dest: "content"'
  End

  validate_templates() {
    local template
    for template in vhdbuilder/packer/vhd-image-builder-acl{,-arm64}.json; do
      jq -e '
        .provisioners[0].source == "vhdbuilder/packer/acl-extensions.sh" and
        (.provisioners[1].inline[0] | endswith(" start")) and
        .provisioners[2].inline[0] == "sudo mkdir -p /opt/azure/containers" and
        (.provisioners[-5].inline[-1] | contains("acl-extensions.sh finish")) and
        (.provisioners[-5].inline[-2] | contains("/boot/flatcar/first_boot")) and
        (.provisioners[-4:] | map({type, direction, source, destination})) == [
          {type: "file", direction: "download", source: "/var/lib/extensions/aks-opt.raw", destination: "aks-opt.raw"},
          {type: "file", direction: "download", source: "/var/lib/confexts/aks-etc.raw", destination: "aks-etc.raw"},
          {type: "file", direction: "download", source: "/var/log/aks-opt.files.txt", destination: "aks-opt.files.txt"},
          {type: "file", direction: "download", source: "/var/log/aks-etc.files.txt", destination: "aks-etc.files.txt"}
        ]
      ' "$template" >/dev/null || return 1
    done
  }

  It 'captures both architectures before provisioning through final cleanup'
    When call validate_templates
    The status should be success
    The output should be blank
  End

  validate_listings() {
    unshare --mount --propagation private bash -s -- "$TEST_ROOT" "$PWD" <<'SCRIPT'
set -euo pipefail
source "$2/vhdbuilder/packer/acl-extensions.sh"
root="$1/listings"
for hierarchy in opt etc; do
    name=aks-opt
    [[ "$hierarchy" != etc ]] || name=aks-etc
    mkdir -p "$root/$name/$hierarchy/directory"
    printf 'payload\n' > "$root/$name/$hierarchy/file with spaces"
    chmod 751 "$root/$name/$hierarchy/file with spaces"
    chown 12345:12346 "$root/$name/$hierarchy/file with spaces"
    ln -s 'file with spaces' "$root/$name/$hierarchy/link"
    mknod "$root/$name/$hierarchy/deleted" c 0 0
    writeExtensionRelease "$root/$name" "$hierarchy" "$name"
    buildExtensionImage "$root/$name" "$root/$name.raw"
    before=$(sha256sum "$root/$name.raw")
    writeExtensionListing "$root/$name.raw" "$root/$name.files.txt"
    test "$before" = "$(sha256sum "$root/$name.raw")"
    grep -F "# Image: $name.raw" "$root/$name.files.txt" >/dev/null
    grep -F "./$hierarchy/directory" "$root/$name.files.txt" >/dev/null
    grep -F "./$hierarchy/file\\ with\\ spaces" "$root/$name.files.txt" | grep -E '\-rwxr-x--x.*12345.*12346' >/dev/null
    grep -F "./$hierarchy/link -> file\\ with\\ spaces" "$root/$name.files.txt" >/dev/null
    grep -F "./$hierarchy/deleted" "$root/$name.files.txt" | grep -E 'c.*0,[[:space:]]+0' >/dev/null
    grep -F "extension-release.$name" "$root/$name.files.txt" >/dev/null
    grep -F './lost+found' "$root/$name.files.txt" >/dev/null
    test "$(stat -c %a "$root/$name.files.txt")" = 644
done
SCRIPT
  }

  It 'lists actual sysext and confext image entries without modifying the images'
    Skip if 'root is required for isolated mount namespace testing' [ "$(id -u)" != 0 ]
    When call validate_listings
    The status should be success
    The output should be blank
  End

  It 'fails if an extension image cannot be mounted for listing'
    mount() { return 1; }
    When call writeExtensionListing "$TEST_ROOT/missing.raw" "$TEST_ROOT/missing.files.txt"
    The status should be failure
    The output should be blank
  End

  overlay_round_trip() {
    unshare --mount --propagation private bash -s -- "$TEST_ROOT" "$PWD" <<'SCRIPT'
set -euo pipefail
source "$2/vhdbuilder/packer/acl-extensions.sh"
root="$1/round-trip"
mkdir -p "$root"/{base,upper,work,merged,image,image-mount}
trap 'umount "$root/merged" 2>/dev/null || :; umount "$root/image-mount" 2>/dev/null || :' EXIT
printf 'unchanged\n' > "$root/base/unchanged"
printf 'old\n' > "$root/base/changed"
printf 'removed\n' > "$root/base/deleted"
mkdir "$root/base/opaque"
touch "$root/base/opaque/old"
mount -t overlay overlay -o "lowerdir=$root/base,upperdir=$root/upper,workdir=$root/work" "$root/merged"
printf 'new\n' > "$root/merged/changed"
printf 'added\n' > "$root/merged/added"
chmod 751 "$root/merged/added"
chown 12345:12346 "$root/merged/added"
ln -s added "$root/merged/link"
rm "$root/merged/deleted"
rm -r "$root/merged/opaque"
mkdir "$root/merged/opaque"
touch "$root/merged/opaque/new"
umount "$root/merged"
test "$(cat "$root/base/changed")" = old
test -f "$root/base/deleted"
test ! -e "$root/base/added"
mv "$root/upper" "$root/image/opt"
writeExtensionRelease "$root/image" opt aks-opt
buildExtensionImage "$root/image" "$root/aks-opt.raw"
mount -o loop,ro "$root/aks-opt.raw" "$root/image-mount"
mount -t overlay overlay -o "lowerdir=$root/image-mount/opt:$root/base" "$root/merged"
test "$(cat "$root/merged/unchanged")" = unchanged
test "$(cat "$root/merged/changed")" = new
test "$(cat "$root/merged/link")" = added
test "$(stat -c %a:%u:%g "$root/merged/added")" = 751:12345:12346
test ! -e "$root/merged/deleted"
test ! -e "$root/merged/opaque/old"
test -f "$root/merged/opaque/new"
SCRIPT
  }

  It 'round-trips overlay deletions and opaque directories without modifying the base'
    Skip if 'root is required for isolated mount namespace testing' [ "$(id -u)" != 0 ]
    When call overlay_round_trip
    The status should be success
    The output should be blank
  End

  Describe 'sysext discovery'
    Include parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh

    It 'downloads images outside the hierarchy they help activate'
      retrycmd_if_failure() { printf '%s\n' "$*"; }
      export ORAS_REGISTRY_CONFIG_FILE=/tmp/registry-config
      When call downloadSysextFromVersion kubelet registry/kubelet:version
      The status should be success
      The output should include '--output /var/lib/aks-sysext-cache/kubelet/downloads'
    End

    It 'finds images in the independent cache'
      find() { printf '%s\n' "$*"; }
      When call matchLocalSysext kubelet 1.33 x86-64
      The status should be success
      The output should include '/var/lib/aks-sysext-cache/kubelet/downloads'
    End
  End

  It 'rejects systemd versions without mutable extension support'
    command() { return 0; }
    systemd-sysext() { echo 'systemd 255'; }
    When call checkExtensionSupport
    The status should be failure
    The stderr should include 'requires systemd >= 256'
  End
End
