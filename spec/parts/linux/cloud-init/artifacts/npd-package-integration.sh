#!/usr/bin/env bash
# Run only in a disposable Ubuntu or Azure Linux 3 container with jq installed
# (Azure Linux also needs tar and rpm-build):
# bash spec/parts/linux/cloud-init/artifacts/npd-package-integration.sh /tmp/npd.{deb,rpm}
# Uses real apt/dpkg or tdnf/rpm and the published package. Only NPD restart is stubbed.
set -euo pipefail

source ./parts/linux/cloud-init/artifacts/npd-update.sh

package="$(realpath "$1")"
format="$(npd_package_format)"
read -r distro release <<< "$(npd_platform)"
IFS=$'\t' read -r _name old_version _arch <<< "$(npd_package_fields "${package}")"
case "${format}" in
    deb) new_version="1.0.1-ubuntu${release}u1"; map=ubuntuPackageVersions ;;
    rpm) new_version="1.0.1-1.azl${release%%.*}"; map=azureLinuxPackageVersions ;;
esac
mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}"
cp "${package}" "${NPD_PACKAGE_CACHE}/old.${format}"
npd_validate_package "${package}"
npd_install_package "${package}"
touch "${NPD_SKIP_FILE}"

# Build a test-only revision with a changed monitor set and package data.
removed=/etc/node-problem-detector.d/custom-plugin-monitor/custom-zombie-process-monitor.json
added=/etc/node-problem-detector.d/custom-plugin-monitor/test-only-monitor.json
# The new package changes a monitor that was also edited on the node; the package copy must win.
changed=/etc/node-problem-detector.d/custom-plugin-monitor/custom-kubelet-monitor.json
revise_monitor() { jq -c --arg revision "$2" '. + {testRevision: $revision}' "$1" > "$1.tmp" && mv "$1.tmp" "$1"; }
stage="$(mktemp -d)"
trap 'rm -rf "${stage}"' EXIT
case "${format}" in
    deb)
        dpkg-deb -R "${package}" "${stage}"
        sed -i "s/^Version: .*/Version: ${new_version}/" "${stage}/DEBIAN/control"
        # Keep required kubelet config; remove a different monitor to test dpkg obsolete handling.
        rm "${stage}${removed}"
        sed -i '\|custom-zombie-process-monitor.json|d' "${stage}/DEBIAN/conffiles"
        printf '{"test":true}\n' > "${stage}${added}"
        printf '%s\n' "${added}" >> "${stage}/DEBIAN/conffiles"
        revise_monitor "${stage}${changed}" package
        dpkg-deb --build "${stage}" "${NPD_PACKAGE_CACHE}/new.deb"
        ;;
    rpm)
        mkdir -p "${stage}/root"
        npd_extract_package "${package}" "${stage}/root"
        rm "${stage}/root${removed}"
        printf '{"test":true}\n' > "${stage}/root${added}"
        revise_monitor "${stage}/root${changed}" package
        {
            printf 'Name: node-problem-detector-aks-config\nVersion: %s\nRelease: %s\n' "${new_version%%-*}" "${new_version#*-}"
            printf 'Summary: test\nLicense: MIT\nBuildArch: %s\n%%description\ntest\n' "$(uname -m)"
            printf '%%install\ncp -a %s/root/. %%{buildroot}/\n%%files\n' "${stage}"
            (cd "${stage}/root" && find . -type f | sed 's|^\.||' | while read -r file; do
                case "${file}" in
                    /etc/*) printf '%%config(noreplace) %s\n' "${file}" ;;
                    *) printf '%s\n' "${file}" ;;
                esac
            done)
        } > "${stage}/npd.spec"
        rpmbuild -bb --quiet --define "_topdir ${stage}/build" --define '_build_id_links none' "${stage}/npd.spec"
        cp "${stage}"/build/RPMS/*/*.rpm "${NPD_PACKAGE_CACHE}/new.rpm"
        ;;
esac
desired="$(jq -nc --arg map "${map}" --arg release "${release}" --arg version "${new_version}" '{($map):{($release):$version}}')"
old_goal="$(jq -nc --arg map "${map}" --arg release "${release}" --arg version "${old_version}" '{($map):{($release):$version}}')"

npd_restart_and_check() { return 0; }
# Equal baked package must not contact repositories.
npd_refresh_metadata() { echo 'unexpected metadata refresh' >&2; return 1; }
npd_download_package() { echo 'unexpected download' >&2; return 1; }
updateNPDConfigs "${old_goal}" '{}'

# New held package installs, obsolete monitor disappears, marker survives.
revise_monitor "${changed}" local
updateNPDConfigs "${desired}" '{}'
test "$(npd_installed_version)" = "${new_version}"
test "$(jq -r .testRevision "${changed}")" = package
test -z "$(find /etc/node-problem-detector.d /etc/systemd/system -name '*.rpmnew' -o -name '*.rpmsave' -o -name '*.dpkg-*')"
test ! -f "${removed}"
test -f "${added}"
test -f "${NPD_SKIP_FILE}"
test ! -f "${NPD_UPDATE_DIR}/pending"

# Simulate interrupted application: recovery must restore the package and monitor set.
printf '%s\n' "${old_version}" > "${NPD_UPDATE_DIR}/pending"
updateNPDConfigs "${old_goal}" '{}'
test "$(npd_installed_version)" = "${old_version}"
test -f "${removed}"
test ! -f "${added}"
test -f "${NPD_SKIP_FILE}"

# Fail the new service once; the real handler must reinstall the old package.
npd_restart_and_check() {
    if [ "$(npd_installed_version)" = "${new_version}" ]; then return 1; fi
}
if updateNPDConfigs "${desired}" '{}'; then
    echo 'expected activation failure' >&2
    exit 1
fi
test "$(npd_installed_version)" = "${old_version}"
test -f "${removed}"
test ! -f "${added}"
test -f "${NPD_SKIP_FILE}"
test ! -f "${NPD_UPDATE_DIR}/pending"
echo "PASS: ${distro} real package install, held upgrade, local-edit replacement, obsolete cleanup, recovery, and failed activation"
