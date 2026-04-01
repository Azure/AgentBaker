#!/bin/bash

echo "Sourcing tool_installs_acl.sh"

stub() {
    echo "${FUNCNAME[1]} stub"
}

installBcc() {
    stub
}

installBpftrace() {
    stub
}

listInstalledPackages() {
    stub
}

disableNtpAndTimesyncdInstallChrony() {
    # On ACL, chronyd is preinstalled and ntp does not exist, so we only need to
    # mask timesyncd (to prevent conflicts) and enable+start chronyd.

    systemctl stop systemd-timesyncd || exit 1
    systemctl disable systemd-timesyncd || exit 1
    systemctl mask systemd-timesyncd || exit 1

    systemctlEnableAndStart chronyd 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

# Move unit files out of systemd's search path to prevent first-boot preset-all
# from auto-enabling them. CSE restores them before selectively enabling.
deferFirstBootPresetServices() {
    local defer_dir="/opt/azure/containers/deferred-units"
    mkdir -p "${defer_dir}"
    for svc in kms.service mig-partition.service localdns.service secure-tls-bootstrap.service; do
        if [ -f "/etc/systemd/system/${svc}" ]; then
            mv "/etc/systemd/system/${svc}" "${defer_dir}/${svc}"
        fi
    done
}
