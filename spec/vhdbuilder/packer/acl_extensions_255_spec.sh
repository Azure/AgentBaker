#!/usr/bin/env shellspec

Describe 'ACL systemd 255 extension lifecycle'
  Include vhdbuilder/packer/acl-extensions.sh

  It 'refuses a root export identity'
    SUDO_USER=root
    When call prepareExtensionExport
    The status should be failure
    The stderr should include 'requires sudo from the non-root Packer account'
  End

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
    local mode="${1:-normal}" root="${SHELLSPEC_TMPBASE}/lifecycle-${1:-normal}"
    mkdir -p "$root"/{etc/systemd/system,etc/alternatives,opt,varlib,varlog}
    ln -s "$(readlink -f "$(command -v awk)")" "$root/etc/alternatives/awk"
    if [ -f /etc/mke2fs.conf ]; then cp /etc/mke2fs.conf "$root/etc/mke2fs.conf"; fi
    printf 'base\n' > "$root/opt/base"
    printf 'old\n' > "$root/opt/removed"
    printf 'old configuration\n' > "$root/etc/removed"
    printf 'base configuration\n' > "$root/etc/overridden"
    printf 'base-only\n' > "$root/etc/base file"
    chmod 751 "$root/etc/base file"
    chown 23456:23457 "$root/etc/base file"
    ln -s 'base file' "$root/etc/base-link"
    ln -s absent "$root/etc/dangling-link"
    touch "$root/etc/.base-hidden"
    printf 'old-machine-id\n' > "$root/etc/machine-id"
    printf 'root:x:0:0:root:/root:/bin/bash\nacl-export-test:x:12345:12345:export:/home/acl-export-test:/bin/bash\n' > "$root/etc/passwd"
    printf 'root:x:0:\nacl-export-test:x:12345:\n' > "$root/etc/group"
    printf 'root:!:::::::\n' > "$root/etc/shadow"
    printf 'passwd: files\ngroup: files\nshadow: files\nhosts: files dns\n' > "$root/etc/nsswitch.conf"
    printf '127.0.0.1 localhost %s\n' "$(hostname)" > "$root/etc/hosts"
    printf 'root ALL=(ALL) ALL\n@includedir /etc/sudoers.d\n' > "$root/etc/sudoers"
    chmod 440 "$root/etc/sudoers"
    mkdir -p "$root/etc/pam.d"
    printf 'auth required pam_deny.so\naccount required pam_deny.so\nsession required pam_deny.so\n' > "$root/etc/pam.d/sudo"
    ln -s /usr/lib/os-release "$root/etc/os-release"
    unshare --mount --propagation private bash -euo pipefail -s -- "$PWD" "$root" "$mode" <<'SCRIPT'
repo=$1
root=$2
mode=$3
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
deprovisionExtensionBuild() {
    printf 'deprovisioned\n' > /etc/deprovisioned
    sed -i '/^acl-export-test:/d' /etc/passwd /etc/group
}

startExtensionCapture
test -f "$root/etc/systemd/system/aks-extensions.service"
test ! -e /var/lib/aks-extensions.mutable/etc/systemd/system/aks-extensions.service
printf 'captured\n' > /opt/captured
chmod 751 /opt/captured
ln -s captured /opt/link
printf 'configuration\n' > /etc/captured
printf 'updated configuration\n' > /etc/overridden
rm /opt/removed /etc/removed
# Keep the capture overlay referenced while finalization rotates its workdirs.
exec 8</opt
if [[ "$mode" == export ]]; then
    SUDO_USER=acl-export-test prepareExtensionExport
    if enableExtensionExport 2>"$root/unremoved-account.err"; then exit 1; fi
    grep -q 'Refusing to export extensions containing the Packer account' "$root/unremoved-account.err"
fi
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
grep -F '# Base directory: /etc' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/base\ file' /var/log/aks-etc-base.files.txt | grep -E '\-rwxr-x--x.*23456.*23457' >/dev/null
grep -F './etc/base-link -> base\ file' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/dangling-link -> absent' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/.base-hidden' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/removed' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/overridden' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/systemd/system/aks-extensions.service' /var/log/aks-etc-base.files.txt >/dev/null
grep -F './etc/machine-id' /var/log/aks-etc-base.files.txt >/dev/null
! grep -F './etc/captured' /var/log/aks-etc-base.files.txt
! grep -F './etc/deprovisioned' /var/log/aks-etc-base.files.txt
! grep -F './etc/extension-release.d/extension-release.aks-etc' /var/log/aks-etc-base.files.txt
! grep -F './etc/sudoers.d/99-aks-extension-export' /var/log/aks-etc-base.files.txt
test "$(stat -c %a /var/log/aks-etc-base.files.txt)" = 644
test "$(cat "$root/etc/overridden")" = 'base configuration'
test "$(cat /etc/overridden)" = 'updated configuration'
sha256sum /var/lib/aks-extensions/*.raw > "$root/images.sha256"

if [[ "$mode" == export ]]; then
    status=0
    setpriv --reuid=12345 --regid=12345 --clear-groups /usr/bin/scp -f /var/log/aks-opt.files.txt </dev/null >"$root/scp-before.out" 2>"$root/scp-before.err" || status=$?
    test "$status" = 255
    grep -q 'unknown user 12345' "$root/scp-before.err"
    enableExtensionExport
    test "$(getent passwd acl-export-test | cut -d: -f3)" = 12345
    ! grep -q '^acl-export-test:' /etc/shadow
    ! grep -q '^acl-export-test:' /run/aks-extensions/image-etc/etc/passwd
    test ! -e /run/aks-extensions/image-etc/etc/sudoers.d/99-aks-extension-export
    test ! -e /home/acl-export-test
    printf '\0\0\0' | setpriv --reuid=12345 --regid=12345 --clear-groups /usr/bin/scp -f /var/log/aks-opt.files.txt > "$root/scp-transfer.out" 2>"$root/scp-transfer.err"
    grep -aq 'aks-opt.files.txt' "$root/scp-transfer.out"
    test ! -s "$root/scp-transfer.err"
    mkdir /run/aks-extensions/upload
    chown 12345:12345 /run/aks-extensions/upload
    printf 'C0600 5 cleanup.sh\nexit\n\0' | setpriv --reuid=12345 --regid=12345 --clear-groups /usr/bin/scp -t /run/aks-extensions/upload >"$root/scp-upload.out" 2>"$root/scp-upload.err"
    test "$(cat /run/aks-extensions/upload/cleanup.sh)" = exit
    test "$(stat -c %a /run/aks-extensions/upload/cleanup.sh)" = 600
    if setpriv --reuid=12345 --regid=12345 --clear-groups sudo -n /usr/bin/true >"$root/denied.out" 2>"$root/denied.err"; then exit 1; fi
    # Exercise the actual restricted sudo command without changing the host OS.
    printf 'ID=azurelinux\nVARIANT_ID=azurecontainerlinux\n' > "$root/acl-release"
    mount --bind "$root/acl-release" /usr/lib/os-release
    setpriv --reuid=12345 --regid=12345 --clear-groups sudo -n /var/lib/aks-extensions/acl-extensions.sh complete >"$root/complete.out" 2>"$root/complete.err"
    umount /usr/lib/os-release
    ! getent passwd acl-export-test
    test ! -e /etc/sudoers.d/99-aks-extension-export
    test ! -e /run/aks-extensions/export
    sha256sum --status -c "$root/images.sha256"
    status=0
    setpriv --reuid=12345 --regid=12345 --clear-groups /usr/bin/scp -f /var/log/aks-opt.files.txt </dev/null >"$root/scp-after.out" 2>"$root/scp-after.err" || status=$?
    test "$status" = 255
    grep -q 'unknown user 12345' "$root/scp-after.err"
fi

printf 'persistent opt\n' > /opt/runtime
printf 'persistent etc\n' > /etc/runtime
rm /opt/captured /etc/captured
opt_mount_id=$(findmnt -rn -M /opt -o ID)
source "$repo/parts/linux/cloud-init/artifacts/acl/cse_install_acl.sh" >/dev/null
getSystemdArch() { echo x86-64; }
retrycmd_if_failure() {
    shift 3
    "$@"
}
oras() {
    local destination
    while [[ $# -gt 0 ]]; do
        if [[ "$1" == --output ]]; then destination=$2; break; fi
        shift
    done
    mkdir -p "$destination"
    touch "$destination/aks-secure-tls-bootstrap-client-v1.1.6-1-azlinux3-x86-64.raw"
}
ORAS_REGISTRY_CONFIG_FILE=/etc/oras/config.yaml
downloadSysextFromVersion aks-secure-tls-bootstrap-client registry/client:version /opt/aks-secure-tls-bootstrap-client/downloads >/dev/null
downloaded=$(matchLocalSysext aks-secure-tls-bootstrap-client 1.1.6-1-azlinux3 x86-64)
test "$downloaded" = /var/lib/aks-sysext-cache/aks-secure-tls-bootstrap-client/downloads/aks-secure-tls-bootstrap-client-v1.1.6-1-azlinux3-x86-64.raw
test -f "$downloaded"
test ! -e /opt/aks-secure-tls-bootstrap-client/downloads
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

  It 'exports through real SCP after deprovisioning and removes temporary access through restricted sudo'
    Skip if 'requires root and a private mount namespace' [ "$(id -u)" -ne 0 ]
    When call validate_lifecycle export
    The status should be success
    The output should be blank
  End
End
