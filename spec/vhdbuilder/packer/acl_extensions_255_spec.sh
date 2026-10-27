#!/usr/bin/env shellspec

Describe 'ACL systemd 255 extension lifecycle'
  Include vhdbuilder/packer/acl-extensions.sh

  validate_release_rejection() {
    local root="${SHELLSPEC_TMPBASE}/release"
    writeExtensionRelease "$root" opt aks-opt
    printf 'ID=not-the-base-os\nVERSION_ID=0\n' > "$root/usr/lib/extension-release.d/extension-release.aks-opt"
    checkExtensionRelease "$root" opt
  }

  It 'rejects an extension for a different OS'
    When call validate_release_rejection
    The status should be failure
    The stderr should include 'does not match the base OS identity'
  End

  validate_lifecycle() {
    local root="${SHELLSPEC_TMPBASE}/lifecycle"
    mkdir -p "$root"/{etc/systemd/system,etc/alternatives,opt,varlib,varlog}
    ln -s "$(readlink -f "$(command -v awk)")" "$root/etc/alternatives/awk"
    if [ -f /etc/mke2fs.conf ]; then cp /etc/mke2fs.conf "$root/etc/mke2fs.conf"; fi
    printf 'base\n' > "$root/opt/base"
    printf 'old\n' > "$root/opt/removed"
    printf 'old configuration\n' > "$root/etc/removed"
    printf 'old-machine-id\n' > "$root/etc/machine-id"
    ln -s /usr/lib/os-release "$root/etc/os-release"
    unshare --mount --propagation private bash -euo pipefail -s -- "$PWD" "$root" <<'SCRIPT'
repo=$1
root=$2
source "$repo/vhdbuilder/packer/acl-extensions.sh"
native_sysext=$(command -v systemd-sysext)
native_version=$("$native_sysext" --version | head -n1 | awk '{print $2}')
mount --bind "$root/opt" /opt
mount --bind "$root/etc" /etc
mount --bind "$root/varlib" /var/lib
mount --bind "$root/varlog" /var/log
mount -t tmpfs tmpfs /run
systemctl() { printf '%s\n' "$*" >> "$root/systemctl.log"; }
systemd-sysext() {
    if [[ "$1" == --version ]]; then
        echo 'systemd 255'
    else
        [[ "$*" == '--no-reload refresh' && "$SYSTEMD_SYSEXT_HIERARCHIES" == /usr ]]
        if ((native_version >= 255)); then
            SYSTEMD_LOG_LEVEL=warning "$native_sysext" "$@" >/dev/null
        fi
    fi
}
deprovisionExtensionBuild() { printf 'deprovisioned\n' > /etc/deprovisioned; }

startExtensionCapture
test -f "$root/etc/systemd/system/aks-extensions.service"
test ! -e /var/lib/aks-extensions.mutable/etc/systemd/system/aks-extensions.service
printf 'captured\n' > /opt/captured
chmod 751 /opt/captured
ln -s captured /opt/link
printf 'configuration\n' > /etc/captured
rm /opt/removed /etc/removed
# Keep the capture overlay referenced while finalization rotates its workdirs.
exec 8</opt
finishExtensionCapture
exec 8<&-
test "$(cat /opt/link)" = captured
test "$(stat -c %a /opt/captured)" = 751
test "$(cat /etc/captured)" = configuration
test "$(cat /etc/deprovisioned)" = deprovisioned
test ! -e /opt/removed
test ! -e /etc/removed
test ! -s /etc/machine-id
test ! -e /run/aks-extensions/image-etc/etc/machine-id
test ! -e /run/aks-extensions/image-etc/etc/systemd/system/aks-extensions.service
test -f "$root/opt/removed"
test -f "$root/etc/removed"
test ! -e "$root/opt/captured"
test ! -e "$root/etc/captured"
test ! -e /var/lib/extensions/aks-opt.raw
test ! -e /var/lib/confexts/aks-etc.raw
test ! -e /var/lib/aks-extension-build
test -z "$(ls -A /var/lib/aks-extensions.mutable/opt)"
test -z "$(ls -A /var/lib/aks-extensions.mutable/etc)"
grep -q './opt/captured' /var/log/aks-opt.files.txt
grep -q './etc/captured' /var/log/aks-etc.files.txt
sha256sum /var/lib/aks-extensions/*.raw > "$root/images.sha256"

printf 'persistent opt\n' > /opt/runtime
printf 'persistent etc\n' > /etc/runtime
rm /opt/captured /etc/captured
opt_mount_id=$(findmnt -rn -M /opt -o ID)
source "$repo/parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh" >/dev/null
getSystemdArch() { echo x86-64; }
if ((native_version >= 255)); then
    native_root=/var/lib/native-extension
    writeExtensionRelease "$native_root" opt aks-native
    mkdir -p "$native_root/usr/bin" /etc/extensions
    printf '#!/bin/sh\necho native\n' > "$native_root/usr/bin/acl255-native-probe"
    chmod 755 "$native_root/usr/bin/acl255-native-probe"
    ln -s "$native_root" /etc/extensions/aks-native
fi
(cd /opt && mergeSysexts)
test "$(findmnt -rn -M /opt -o ID)" = "$opt_mount_id"
printf 'still writable\n' > /opt/after-refresh
if ((native_version >= 255)); then
    test "$(/usr/bin/acl255-native-probe)" = native
    SYSTEMD_LOG_LEVEL=warning "$native_sysext" --no-reload unmerge >/dev/null
fi
stopExtensions
test ! -e /opt/runtime
test ! -e /etc/runtime
activateExtensions
test "$(cat /opt/runtime)" = 'persistent opt'
test "$(cat /etc/runtime)" = 'persistent etc'
test "$(cat /opt/after-refresh)" = 'still writable'
test ! -e /opt/captured
test ! -e /etc/captured
sha256sum --status -c "$root/images.sha256"
stopExtensions

# Failure of the second hierarchy must roll back the first.
mv /var/lib/aks-extensions/aks-opt.raw /var/lib/aks-extensions/aks-opt.saved
if activateExtensions 2>"$root/missing.log"; then exit 1; fi
grep -q 'Missing ACL extension image' "$root/missing.log"
! isExtensionMounted etc
! mountpoint -q /run/aks-extensions/image-etc
mv /var/lib/aks-extensions/aks-opt.saved /var/lib/aks-extensions/aks-opt.raw
activateExtensions
stopExtensions

checkExtensionRelease() { echo 'incompatible image' >&2; return 1; }
if activateExtensions 2>"$root/incompatible.log"; then exit 1; fi
grep -q 'incompatible image' "$root/incompatible.log"
! isExtensionMounted etc
! mountpoint -q /run/aks-extensions/image-etc
SCRIPT
  }

  It 'captures, packages and reactivates writable layers without mutable APIs'
    Skip if 'requires root and a private mount namespace' [ "$(id -u)" -ne 0 ]
    When call validate_lifecycle
    The status should be success
    The output should be blank
  End
End
