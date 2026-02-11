#!/bin/bash

echo "Sourcing tool_installs_flatcar.sh"

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
    # Configure chronyd for ACL only (Flatcar uses its base image config which already works)
    # ACL has ID=acl in /etc/os-release, so the OS variable will be "ACL"
    if [ "${OS:-}" != "ACL" ]; then
        echo "Skipping chrony configuration for non-ACL Flatcar-based OS"
        return 0
    fi

    echo "Configuring chronyd for ACL"

    # Disable systemd-timesyncd if it exists
    systemctl stop systemd-timesyncd 2>/dev/null || true
    systemctl disable systemd-timesyncd 2>/dev/null || true
    systemctl mask systemd-timesyncd 2>/dev/null || true

    # CRITICAL: ACL has a read-only /usr filesystem (immutable, verity-protected).
    # The default chronyd unit loads config from /usr/lib/chrony/chrony.conf which
    # cannot be modified. Instead we:
    #   1. Write our config to /etc/chrony/chrony.conf (writable)
    #   2. Add a systemd drop-in to override ExecStart to use -f /etc/chrony/chrony.conf
    # The drop-in directory /etc/systemd/system/chronyd.service.d/ is already created
    # by install-dependencies.sh (for 10-chrony-restarts.conf).

    local chrony_config_dir="/etc/chrony"
    local chrony_config="${chrony_config_dir}/chrony.conf"
    local chronyd_dropin_dir="/etc/systemd/system/chronyd.service.d"

    mkdir -p "${chrony_config_dir}"

    # Write Azure-optimized chrony configuration
    cat > "${chrony_config}" <<'EOF'
# Azure-optimized chrony configuration for ACL
# Based on: https://learn.microsoft.com/en-us/azure/virtual-machines/linux/time-sync

# Use Azure's Precision Time Protocol (PTP) device via Hyper-V
# /dev/ptp_hyperv is a symlink to /dev/ptp0 on Azure VMs
refclock PHC /dev/ptp_hyperv poll 3 dpoll -2 offset 0 stratum 2

# Use time.windows.com as fallback NTP server
server time.windows.com

# CRITICAL: Always step the clock when offset > 1 second.
# The default 'makestep 1.0 3' only steps during the first 3 updates,
# then slews gradually — which cannot correct large offsets (e.g. the VHD
# test sets time 5 years in the past). Using -1 means "always step".
makestep 1.0 -1

# Record the rate at which the system clock gains/losses time
driftfile /var/lib/chrony/drift

# Enable kernel synchronization of the real-time clock (RTC)
rtcsync

# Get TAI-UTC offset and leap seconds from the system tz database
leapsectz right/UTC

# Specify file containing keys for NTP authentication
keyfile /etc/chrony.keys

# Save NTS keys and cookies
ntsdumpdir /var/lib/chrony

# Specify directory for log files
logdir /var/log/chrony

# Don't panic on large time offset changes
maxupdateskew 100.0

# Tolerate higher delay from time.windows.com
maxdistance 16.0

# Disable listening on UDP port (leaving only Unix socket interface)
cmdport 0
EOF

    # Add systemd drop-in to override ExecStart to use our writable config path.
    # The empty 'ExecStart=' line clears the original ExecStart before setting a new one
    # (required by systemd convention for Type=forking services).
    mkdir -p "${chronyd_dropin_dir}"
    cat > "${chronyd_dropin_dir}/20-chrony-config-override.conf" <<'EOF'
[Service]
ExecStart=
ExecStart=/usr/sbin/chronyd -f /etc/chrony/chrony.conf $OPTIONS
EOF

    # Ensure required directories exist (may already exist from base image)
    mkdir -p /var/lib/chrony
    mkdir -p /var/log/chrony

    # Reload systemd to pick up the new drop-in, then restart chronyd
    systemctl daemon-reload
    systemctl restart chronyd || {
        echo "Error: Failed to restart chronyd"
        systemctl status chronyd || true
        return 1
    }

    # Verify chronyd is active
    if ! systemctl is-active chronyd >/dev/null 2>&1; then
        echo "Error: chronyd is not active after restart"
        systemctl status chronyd || true
        return 1
    fi

    echo "chronyd configured successfully for ACL"
    echo "  - Config: ${chrony_config}"
    echo "  - Drop-in: ${chronyd_dropin_dir}/20-chrony-config-override.conf"
    echo "  - makestep: 1.0 -1 (always step for offset > 1s)"
    echo "  - PTP device: /dev/ptp_hyperv"
    echo "  - Status: $(systemctl is-active chronyd)"
}
