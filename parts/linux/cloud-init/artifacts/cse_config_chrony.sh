#!/bin/bash

is_ubuntu_2604_or_later() {
    local version_major
    local version_minor

    [ "$OS" = "$UBUNTU_OS_NAME" ] || return 1

    IFS='.' read -r version_major version_minor _ <<< "${OS_VERSION:-}"
    case "$version_major" in
        ''|*[!0-9]*) return 1 ;;
    esac
    case "$version_minor" in
        ''|*[!0-9]*) return 1 ;;
    esac

    [ "$version_major" -gt 26 ] ||
        { [ "$version_major" -eq 26 ] && [ "$version_minor" -ge 4 ]; }
}

should_configure_ubuntu_cvm_time_sync() {
    [ "${PRE_PROVISION_ONLY:-false}" != "true" ] || return 1
    is_ubuntu_2604_or_later || return 1

    case "$(uname -r)" in
        *-azure-fde*) return 0 ;;
        *) return 1 ;;
    esac
}

detect_confidential_vm_platform() {
    local platform

    if ! platform="$(systemd-detect-virt --cvm 2>/dev/null)"; then
        return 1
    fi

    case "$platform" in
        sev-snp|tdx)
            echo "$platform"
            ;;
        *)
            return 1
            ;;
    esac
}

ubuntu_ntp_pools() {
    cat <<'EOF'
pool ntp.ubuntu.com        iburst maxsources 4
pool 0.ubuntu.pool.ntp.org iburst maxsources 1
pool 1.ubuntu.pool.ntp.org iburst maxsources 1
pool 2.ubuntu.pool.ntp.org iburst maxsources 2
EOF
}

apply_ubuntu_ntp_chrony_configuration() {
    apply_chrony_configuration "$(ubuntu_ntp_pools)"
}

apply_chrony_configuration() {
    local time_sources="${1:-refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0}"
    local chrony_conf="${CHRONY_CONF:-/etc/chrony/chrony.conf}"
    local timesyncd_load_state
    local chrony_failed=0

    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        timesyncd_load_state="$(systemctl show -p LoadState --value systemd-timesyncd 2>/dev/null || true)"
        if [ "$timesyncd_load_state" = "not-found" ]; then
            echo "systemd-timesyncd is removed, no need to disable"
        else
            if ! systemctl stop systemd-timesyncd; then
                echo "ERROR: failed to stop systemd-timesyncd" >&2
                chrony_failed=1
            fi
            if ! systemctl disable systemd-timesyncd; then
                echo "ERROR: failed to disable systemd-timesyncd" >&2
                chrony_failed=1
            fi
        fi

        if [ ! -e "$chrony_conf" ]; then
            if ! apt_get_update; then
                echo "ERROR: failed to update package metadata before installing Chrony" >&2
                chrony_failed=1
            fi
            if ! apt_get_install 30 1 600 chrony; then
                echo "ERROR: failed to install Chrony" >&2
                chrony_failed=1
            fi
        fi
    elif [ "$OS" = "$FLATCAR_OS_NAME" ]; then
        if ! rm -f "$chrony_conf"; then
            echo "ERROR: failed to remove the existing Flatcar Chrony configuration" >&2
            chrony_failed=1
        fi
    fi

    if ! cat > "$chrony_conf" <<EOF
# Welcome to the chrony configuration file. See chrony.conf(5) for more
# information about usuable directives.

# This will use (up to):
# - 4 sources from ntp.ubuntu.com which some are ipv6 enabled
# - 2 sources from 2.ubuntu.pool.ntp.org which is ipv6 enabled as well
# - 1 source from [01].ubuntu.pool.ntp.org each (ipv4 only atm)
# This means by default, up to 6 dual-stack and up to 2 additional IPv4-only
# sources will be used.
# At the same time it retains some protection against one of the entries being
# down (compare to just using one of the lines). See (LP: #1754358) for the
# discussion.
#
# About using servers from the NTP Pool Project in general see (LP: #104525).
# Approved by Ubuntu Technical Board on 2011-02-08.
# See http://www.pool.ntp.org/join.html for more information.
#pool ntp.ubuntu.com        iburst maxsources 4
#pool 0.ubuntu.pool.ntp.org iburst maxsources 1
#pool 1.ubuntu.pool.ntp.org iburst maxsources 1
#pool 2.ubuntu.pool.ntp.org iburst maxsources 2

# This directive specifies the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony/chrony.keys

# This directive specifies the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/chrony.drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
${time_sources}
makestep 1.0 -1
EOF
    then
        echo "ERROR: failed to write Chrony configuration to ${chrony_conf}" >&2
        chrony_failed=1
    fi

    if [ "$OS" = "$UBUNTU_OS_NAME" ]; then
        if ! systemctl restart chrony; then
            echo "ERROR: failed to restart Chrony" >&2
            chrony_failed=1
        fi
    elif [ "$OS" = "$FLATCAR_OS_NAME" ]; then
        if ! systemctl restart chronyd; then
            echo "ERROR: failed to restart chronyd" >&2
            chrony_failed=1
        fi
    fi

    return "$chrony_failed"
}

configure_mariner_azurelinux_chrony() {
    local chrony_conf="${CHRONY_CONF:-/etc/chrony.conf}"
    local chrony_failed=0

    if ! cat > "$chrony_conf" <<'EOF'
# This directive specifies the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony.keys

# This directive specifies the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF
    then
        echo "ERROR: failed to write Chrony configuration to ${chrony_conf}" >&2
        chrony_failed=1
    fi

    if ! systemctl restart chronyd; then
        echo "ERROR: failed to restart chronyd" >&2
        chrony_failed=1
    fi

    return "$chrony_failed"
}

verify_chrony_ntp_sync() {
    local max_attempts=12
    local retry_interval_seconds=5

    if chronyc waitsync "$max_attempts" 0 0 "$retry_interval_seconds"; then
        echo "NTP synchronization confirmed through the Ubuntu NTP pools"
        return 0
    fi

    echo "ERROR: NTP not reachable; Chrony did not synchronize" >&2
    echo "Chrony source diagnostics:" >&2
    chronyc sources -v >&2 || echo "ERROR: unable to retrieve Chrony source diagnostics" >&2
    echo "Chrony tracking diagnostics:" >&2
    chronyc tracking >&2 || echo "ERROR: unable to retrieve Chrony tracking diagnostics" >&2
    return "$ERR_NTP_UNREACHABLE"
}

configure_ubuntu_cvm_time_sync() {
    local platform

    if ! platform="$(detect_confidential_vm_platform)"; then
        echo "ERROR: unable to determine Ubuntu 26.04 or later CVM platform with systemd-detect-virt --cvm" >&2
        return "$ERR_CVM_PLATFORM_DETECTION_FAIL"
    fi

    case "$platform" in
        sev-snp)
            echo "AMD SEV-SNP detected; preserving the existing Hyper-V PHC Chrony configuration"
            if ! logs_to_events "AKS.CSE.configureChronySEVSNP" apply_chrony_configuration; then
                echo "WARNING: failed to reapply the Hyper-V PHC Chrony configuration for AMD SEV-SNP; continuing provisioning" >&2
            fi
            ;;
        tdx)
            echo "Intel TDX detected; configuring Chrony to use the Ubuntu NTP pools"
            if ! logs_to_events "AKS.CSE.configureChronyTDX" apply_ubuntu_ntp_chrony_configuration; then
                echo "ERROR: failed to configure Chrony with the Ubuntu NTP pools for Intel TDX" >&2
                return "$ERR_CHRONY_CONFIG_FAIL"
            fi
            logs_to_events "AKS.CSE.verifyChronyNTPSync" verify_chrony_ntp_sync
            ;;
    esac
}

configure_node_time_sync() {
    if isACL "$OS" "$OS_VARIANT"; then
        echo "Skipping chrony configuration for ACL (PTP clock baked into chronyd, no external NTP sources)"
    elif isMarinerOrAzureLinux "$OS"; then
        logs_to_events "AKS.CSE.configureChronyMarinerAzureLinux" configure_mariner_azurelinux_chrony || true
    elif should_configure_ubuntu_cvm_time_sync; then
        configure_ubuntu_cvm_time_sync
    else
        logs_to_events "AKS.CSE.configureChronyDefaultPHC" apply_chrony_configuration || true
    fi
}
