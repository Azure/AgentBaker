#!/usr/bin/env bash
# shellcheck disable=SC1091,SC2034,SC2317
# Run only in a disposable Ubuntu or Azure Linux 3 container with jq installed
# (Azure Linux also needs tar and the cloud-native repo), with Microsoft PMC configured:
# bash spec/parts/linux/cloud-init/artifacts/npd-binary-integration.sh <old-binary-pkg> <new-version>
# Installs a real older NPD binary package, then drives updateNPDBinary to a newer
# published revision with the real package manager and PMC. Only the NPD service
# restart and the apiserver reporting check are stubbed.
set -euo pipefail

source ./parts/linux/cloud-init/artifacts/npd-update.sh

old_package="$(realpath "$1")"
new_version="$2"
NPD_COMPONENT=npdBinary
format="$(npd_package_format)"
read -r distro release <<< "$(npd_platform)"
IFS=$'\t' read -r _name old_version _arch <<< "$(npd_package_fields "${old_package}")"
case "${format}" in
    deb) map=ubuntuPackageVersions ;;
    rpm) map=azureLinuxPackageVersions ;;
esac
mkdir -p "${NPD_UPDATE_DIR}" "${NPD_PACKAGE_CACHE}"
cp "${old_package}" "${NPD_PACKAGE_CACHE}/"
npd_validate_package "${old_package}"
npd_install_package "${old_package}"
mkdir -p "$(dirname "${NPD_SKIP_FILE}")"
touch "${NPD_SKIP_FILE}"
goal() { jq -nc --arg map "${map}" --arg release "${release}" --arg version "$1" '{($map):{($release):$version}}'; }

npd_restart_and_check() { return 0; }
npd_check_reporting() { return 0; }

# Same version: no repository access.
npd_refresh_metadata_real="$(declare -f npd_refresh_metadata)"
npd_refresh_metadata() { echo 'unexpected metadata refresh' >&2; return 1; }
updateNPDBinary "$(goal "${old_version}")" '{}'
eval "${npd_refresh_metadata_real}"

# Real download and held upgrade of the binary; the config package is untouched.
config_before="$(NPD_COMPONENT=npdConfig npd_installed_version 2>/dev/null || echo none)"
updateNPDBinary "$(goal "${new_version}")" '{}'
test "$(npd_installed_version)" = "${new_version}"
test "$(NPD_COMPONENT=npdConfig npd_installed_version 2>/dev/null || echo none)" = "${config_before}"
test ! -f "${NPD_UPDATE_DIR}/pending-binary"
/usr/bin/node-problem-detector --version
case "${format}" in
    deb) test "$(dpkg-query -W -f='${db:Status-Want}' node-problem-detector-kubernetes)" = hold ;;
    rpm) grep -qsx node-problem-detector-kubernetes "${NPD_TDNF_LOCK_FILE}" ;;
esac
new_package="$(npd_find_cached_package "${new_version}")"

# Simulate an interrupted binary update: recovery restores the cached old binary.
printf '%s\n' "${old_version}" > "${NPD_UPDATE_DIR}/pending-binary"
updateNPDBinary '{}' '{}'
test "$(npd_installed_version)" = "${old_version}"
test ! -f "${NPD_UPDATE_DIR}/pending-binary"

# NPD stops reporting after the update: the real handler must restore the old binary.
npd_check_reporting() { echo 'stubbed: no heartbeat' >&2; return 1; }
if updateNPDBinary "$(goal "${new_version}")" '{}'; then
    echo 'expected reporting failure' >&2
    exit 1
fi
test "$(npd_installed_version)" = "${old_version}"
test ! -f "${NPD_UPDATE_DIR}/pending-binary"
test -f "${new_package}"
echo "PASS: ${distro} binary no-op, real held upgrade, interrupted recovery, and rollback when NPD stops reporting"
