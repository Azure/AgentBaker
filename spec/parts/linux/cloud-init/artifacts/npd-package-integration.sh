#!/usr/bin/env bash
# Run only in a disposable Ubuntu container with jq and dpkg-deb installed:
# bash spec/parts/linux/cloud-init/artifacts/npd-package-integration.sh /tmp/npd.deb
# Uses real apt/dpkg and the published package. Only NPD restart is stubbed.
set -euo pipefail

source ./parts/linux/cloud-init/artifacts/ubuntu/npd-update.sh

package="$(realpath "$1")"
old_version="$(dpkg-deb -f "${package}" Version)"
release="$(npd_ubuntu_release)"
new_version="1.0.1-ubuntu${release}u1"
mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}"
cp "${package}" "${NPD_PACKAGE_CACHE}/old.deb"
npd_validate_package "${package}"
npd_install_package "${package}"
touch "${NPD_SKIP_FILE}"

# Build a test-only revision with a changed monitor set and package data.
stage="$(mktemp -d)"
trap 'rm -rf "${stage}"' EXIT
dpkg-deb -R "${package}" "${stage}"
sed -i "s/^Version: .*/Version: ${new_version}/" "${stage}/DEBIAN/control"
# Keep required kubelet config; remove a different monitor to test dpkg obsolete handling.
removed=/etc/node-problem-detector.d/custom-plugin-monitor/custom-zombie-process-monitor.json
rm "${stage}${removed}"
sed -i '\|custom-zombie-process-monitor.json|d' "${stage}/DEBIAN/conffiles"
added=/etc/node-problem-detector.d/custom-plugin-monitor/test-only-monitor.json
printf '{"test":true}\n' > "${stage}${added}"
printf '%s\n' "${added}" >> "${stage}/DEBIAN/conffiles"
dpkg-deb --build "${stage}" "${NPD_PACKAGE_CACHE}/new.deb"
desired="$(jq -nc --arg release "${release}" --arg version "${new_version}" '{ubuntuPackageVersions:{($release):$version}}')"
old_goal="$(jq -nc --arg release "${release}" --arg version "${old_version}" '{ubuntuPackageVersions:{($release):$version}}')"

npd_restart_and_check() { return 0; }
# Equal baked package must not contact apt repositories.
npd_refresh_metadata() { echo 'unexpected metadata refresh' >&2; return 1; }
npd_download_package() { echo 'unexpected download' >&2; return 1; }
updateNPDConfigs "${old_goal}" '{}'

# New held package installs, obsolete monitor disappears, marker survives.
updateNPDConfigs "${desired}" '{}'
test "$(npd_installed_version)" = "${new_version}"
test ! -f "${removed}"
test -f "${added}"
test -f "${NPD_SKIP_FILE}"
test ! -f "${NPD_UPDATE_DIR}/pending"

# Simulate interrupted application: recovery must restore dpkg and the monitor set.
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
echo 'PASS: real package install, held upgrade, obsolete cleanup, recovery, and failed activation'
