#!/bin/bash

# localdns corefile used by localdns systemd unit.
LOCALDNS_CORE_FILE="/opt/azure/containers/localdns/localdns.corefile"
# localdns slice file used by localdns systemd unit.
LOCALDNS_SLICE_FILE="/etc/systemd/system/localdns.slice"
# This function is called from cse_main.sh.
# It creates the localdns corefile and slicefile, then enables and starts localdns.
# Both corefile variants are read from globals set in cse_cmd.sh:
#   LOCALDNS_COREFILE_BASE         — standard corefile without hosts plugin
#   LOCALDNS_COREFILE_WITH_HOSTS — corefile with hosts plugin
# The base variant is written as the initial active corefile.
# Both variants are saved to /etc/localdns/environment so localdns.sh
# can dynamically switch between them on restart.
generateLocalDNSFiles() {
    mkdir -p "$(dirname "${LOCALDNS_CORE_FILE}")"
    touch "${LOCALDNS_CORE_FILE}"
    chmod 0644 "${LOCALDNS_CORE_FILE}"

    # Determine the base corefile to use as the initial active corefile.
    # LOCALDNS_COREFILE_BASE is set by new CSE; fall back to LOCALDNS_GENERATED_COREFILE
    # for backward compatibility when this VHD runs with an older CSE that only sets
    # LOCALDNS_GENERATED_COREFILE.
    local corefile_base="${LOCALDNS_COREFILE_BASE:-${LOCALDNS_GENERATED_COREFILE:-}}"
    if [ -z "${corefile_base}" ]; then
        echo "Error: neither LOCALDNS_COREFILE_BASE nor LOCALDNS_GENERATED_COREFILE is set"
        exit $ERR_LOCALDNS_FAIL
    fi

    # Start with the base corefile as the initial active corefile.
    # localdns.sh will select the appropriate variant (BASE or WITH_HOSTS)
    # based on the SHOULD_ENABLE_HOSTS_PLUGIN feature flag on service start.
    base64 -d <<< "${corefile_base}" > "${LOCALDNS_CORE_FILE}" || exit $ERR_LOCALDNS_FAIL

    # Log whether the initial corefile includes hosts plugin.
    # This is the BASE corefile; localdns.sh may select the WITH_HOSTS variant at service start.
    if grep -q "hosts /etc/localdns/hosts" "${LOCALDNS_CORE_FILE}"; then
        echo "Initial corefile at ${LOCALDNS_CORE_FILE} INCLUDES hosts plugin"
    else
        echo "Initial corefile at ${LOCALDNS_CORE_FILE} DOES NOT include hosts plugin (localdns.sh selects variant at runtime)"
    fi

    # Create environment file for corefile regeneration.
    # This file will be referenced by localdns.service using EnvironmentFile directive.
    # Save BOTH corefile variants so localdns can dynamically choose on each restart.
    # All corefile values are base64-encoded; localdns.sh decodes them at runtime.
    # LOCALDNS_BASE64_ENCODED_COREFILE is the legacy key for old VHDs.
    # LOCALDNS_COREFILE_BASE is the new name ("BASE" = base variant without hosts plugin, not base64).
    # LOCALDNS_COREFILE_WITH_HOSTS is the variant WITH hosts plugin.
    LOCALDNS_ENV_FILE="/etc/localdns/environment"
    mkdir -p "$(dirname "${LOCALDNS_ENV_FILE}")"
    if [ "${SHOULD_ENABLE_HOSTS_PLUGIN:-false}" = "true" ] && [ -z "${LOCALDNS_COREFILE_WITH_HOSTS:-}" ]; then
        echo "WARNING: SHOULD_ENABLE_HOSTS_PLUGIN=true but LOCALDNS_COREFILE_WITH_HOSTS is empty. Hosts plugin will fall back to BASE corefile at runtime."
    fi
    cat > "${LOCALDNS_ENV_FILE}" <<EOF
LOCALDNS_BASE64_ENCODED_COREFILE=${corefile_base}
LOCALDNS_COREFILE_BASE=${corefile_base}
LOCALDNS_COREFILE_WITH_HOSTS=${LOCALDNS_COREFILE_WITH_HOSTS:-}
SHOULD_ENABLE_HOSTS_PLUGIN=${SHOULD_ENABLE_HOSTS_PLUGIN:-false}
LOCALDNS_CRITICAL_FQDNS=${LOCALDNS_CRITICAL_FQDNS:-}
EOF
    chmod 0644 "${LOCALDNS_ENV_FILE}"

	mkdir -p "$(dirname "${LOCALDNS_SLICE_FILE}")"
    touch "${LOCALDNS_SLICE_FILE}"
    chmod 0644 "${LOCALDNS_SLICE_FILE}"
    cat > "${LOCALDNS_SLICE_FILE}" <<EOF
[Unit]
Description=localdns Slice
DefaultDependencies=no
Before=slices.target
Requires=system.slice
After=system.slice
[Slice]
MemoryMax=${LOCALDNS_MEMORY_LIMIT}
CPUQuota=${LOCALDNS_CPU_LIMIT}
EOF
}

# enableLocalDNS creates localdns files and starts the service.
# Both corefile variants (with/without hosts plugin) are read from globals
# set in cse_cmd.sh. No parameters needed.
enableLocalDNS() {
    # Guard: Check if this VHD has localdns assets installed.
    # Older VHDs may not have localdns.service or the execution script.
    # This ensures backward compatibility when new CSE runs on old VHDs.
    if [ ! -f /etc/systemd/system/localdns.service ]; then
        echo "Warning: localdns.service not found on this VHD, skipping localdns setup"
        return 0
    fi
    if [ ! -f /opt/azure/containers/localdns/localdns.sh ]; then
        echo "Warning: localdns.sh not found on this VHD, skipping localdns setup"
        return 0
    fi

    echo "enableLocalDNS called, generating corefile..."
    generateLocalDNSFiles
    # Log corefile variant after it's been successfully written
    echo "Generated corefile: $(grep -q 'hosts /etc/localdns/hosts' "${LOCALDNS_CORE_FILE}" 2>/dev/null && echo 'WITH hosts plugin' || echo 'WITHOUT hosts plugin')"

    # Disable hosts plugin cleanup path: if the hosts plugin was previously enabled but is now
    # disabled (e.g. rollback), clean up the timer and hosts file. The enable path is handled
    # separately — enableAKSLocalDNSHostsSetup() is called earlier in basePrep() to give the
    # timer a head start on DNS resolution before enableLocalDNS() starts CoreDNS.
    if [ "${SHOULD_ENABLE_HOSTS_PLUGIN}" != "true" ]; then
        logs_to_events "AKS.CSE.enableLocalDNS.disableAKSLocalDNSHostsSetup" disableAKSLocalDNSHostsSetup
    fi

    echo "localdns should be enabled."
    # localdns.service budgets StartLimitBurst=5 / StartLimitIntervalSec=720 so steady-state
    # failures terminate in 'failed' for NPD to observe. Provisioning restarts draw on that same
    # budget -- a manual restart costs a slot just like an automatic one -- so clear it before each
    # attempt. Otherwise the first burst wedges the unit for 12 minutes and every retry below is
    # refused with "Start request repeated too quickly". daemon-reload is not a substitute: it
    # clears start_ratelimit on systemd 249 but not on 255 (Ubuntu 24.04).
    local localdns_started=false
    local i
    # 100 matches what systemctlEnableAndStart did before (systemctl_restart 100 5 30,
    # cse_helpers.sh), but it is a backstop rather than a budget: check_cse_timeout below
    # reaches its limit first in any realistic run, so the loop ends on the CSE deadline,
    # not on the count. Do not reason about the loop's duration from 100. check_cse_timeout bounds the slow case: if
    # every restart hangs for its full 30s timeout, this loop would outlive CSE's 15m kill in
    # cse_start.sh and be SIGKILLed mid-iteration, losing the status log and the exit code
    # below. Breaking out early lets the give-up path run and report properly, matching the
    # other retry loops in cse_helpers.sh.
    #
    # Every systemd call here is wrapped in timeout, as _systemctl_retry_svc_operation did.
    # These all talk to PID 1 over D-Bus; an unbounded one that wedges would never return to
    # the top of the loop, so check_cse_timeout above would never be re-evaluated and CSE
    # would be SIGKILLed before it could report.
    local localdns_giveup_reason="exhausted the restart attempts"
    # Hoisted out of the loop: nothing here rewrites a unit between iterations, and
    # daemon-reload re-parses every unit on the box. _systemctl_retry_svc_operation did it
    # per attempt, but the point of inlining this loop was to stop paying for what that
    # helper did blindly.
    timeout 30 systemctl daemon-reload
    for i in $(seq 1 100); do
        if ! check_cse_timeout; then
            localdns_giveup_reason="CSE provisioning budget exhausted at attempt ${i}"
            break
        fi
        timeout 30 systemctl reset-failed localdns 2>/dev/null || true
        if timeout 30 systemctl restart localdns; then
            localdns_started=true
            break
        fi
        # Periodic, bounded diagnostics. systemctlEnableAndStart used to dump 'systemctl status'
        # plus an unbounded 'journalctl -u' on every failed attempt (shouldLogRetryInfo=true in
        # _systemctl_retry_svc_operation), which is ~99 dumps and a measured 6-8s per iteration --
        # enough to eat most of the provisioning window on its own. Dropping it entirely lost the
        # only record of what was going wrong across the retries, so sample it instead: every
        # tenth attempt, with the journal bounded by -n.
        if [ $((i % 10)) -eq 0 ]; then
            echo "localdns restart attempt ${i} failed; unit state and recent journal follow."
            timeout 30 systemctl status localdns --no-pager -l || true
            timeout 30 journalctl -u localdns --no-pager -n 50 || true
        fi
        sleep 5
    done
    if [ "${localdns_started}" != "true" ]; then
        echo "localdns could not be started: ${localdns_giveup_reason}."
        # No reset here -- the last failure's auto-restarts land the unit in 'failed', which is the
        # terminal state NPD needs.
        #
        # This snapshot is taken ~5s after the last failed restart, so the unit is normally
        # 'activating (auto-restart)' rather than 'failed': the loop cleared the start-limit
        # counter on every iteration, so it cannot have accumulated toward the terminal state yet.
        # That is why the journal is captured alongside it -- the status line alone describes a
        # unit mid-cycle and does not explain why any of the attempts failed.
        timeout 30 systemctl status localdns --no-pager -l > /var/log/azure/localdns-status.log || true
        timeout 30 journalctl -u localdns --no-pager -n 200 >> /var/log/azure/localdns-status.log || true
        exit $ERR_LOCALDNS_FAIL
    fi
    # Log on this path too. systemctlEnableAndStart wrote a status log when 'systemctl enable'
    # failed as well as when the start failed; inlining the loop kept the start path and dropped
    # this one, so an enable failure exited with nothing but the code.
    # Capture the code rather than using 'if ! ...': '!' inverts before $? is read, so the
    # distinction is gone inside the then-block. retrycmd_if_failure returns 2 when
    # check_cse_timeout trips (cse_helpers.sh:270, :298) and 1 when it genuinely exhausts
    # its attempts. Reporting the first as a systemd failure sends the on-call after the
    # wrong thing -- same reason localdns_giveup_reason exists for the start loop above.
    retrycmd_if_failure 120 5 25 systemctl enable localdns
    local enable_rc=$?
    if [ "$enable_rc" -ne 0 ]; then
        if [ "$enable_rc" -eq 2 ]; then
            echo "localdns could not be enabled: CSE provisioning budget exhausted."
        else
            echo "localdns could not be enabled by systemctl."
        fi
        timeout 30 systemctl status localdns --no-pager -l > /var/log/azure/localdns-status.log || true
        timeout 30 journalctl -u localdns --no-pager -n 200 >> /var/log/azure/localdns-status.log || true
        exit $ERR_LOCALDNS_FAIL
    fi
    echo "Enable localdns succeeded."
    # Exporter socket setup is deferred to configureLocalDNSExporterSocket() (after ensureKubelet)
    # to avoid delaying kubelet start. The kubelet node label is added separately in cse_main.sh.
}

# Configures the localdns metrics exporter socket to listen on the node IP.
# Runs after ensureKubelet (the kubelet node label is added separately before ensureKubelet).
# The VHD default binds to 0.0.0.0 which already works for vmagent scraping.
# The drop-in narrows binding to the node IP for tighter scoping when available.
configureLocalDNSExporterSocket() {
    # Guard: skip everything if the socket unit doesn't exist (old VHD without exporter files).
    # This is a backward compatibility check for VHDs built before the exporter was added.
    # Without this guard, we'd create an orphaned drop-in directory and
    # systemctlEnableAndStartNoBlock would hit its retry loop (~100 retries × 5s) for a missing unit.
    if ! systemctl cat localdns-exporter.socket &>/dev/null; then
        echo "localdns-exporter: socket unit not found on this VHD, skipping"
        return 0
    fi

    # Create drop-in to narrow socket binding from 0.0.0.0 to the node IP.
    local node_ip
    node_ip=$(get_primary_nic_ip)
    if [ -n "${node_ip}" ]; then
        echo "localdns-exporter: creating socket drop-in to bind to ${node_ip}:9353"
        mkdir -p /etc/systemd/system/localdns-exporter.socket.d
        tee /etc/systemd/system/localdns-exporter.socket.d/10-listen-address.conf > /dev/null <<EOF
[Socket]
ListenStream=
ListenStream=${node_ip}:9353
EOF
        systemctl daemon-reload
    else
        echo "localdns-exporter: get_primary_nic_ip returned empty, using VHD default (0.0.0.0:9353)"
    fi

    # Enable localdns metrics exporter socket for Prometheus scraping.
    # This is optional observability — don't block provisioning if it fails.
    # Note: the kubelet node label is added separately in cse_main.sh before ensureKubelet.
    echo "Enabling localdns-exporter.socket for metrics collection."
    if systemctlEnableAndStartNoBlock localdns-exporter.socket 30; then
        echo "Enable localdns-exporter.socket succeeded."
    else
        echo "WARNING: Failed to enable localdns-exporter.socket. Metrics will not be available but continuing provisioning."
    fi
}

# This function enables and starts the aks-localdns-hosts-setup timer.
# The timer periodically resolves critical AKS FQDN DNS records and populates /etc/localdns/hosts.
# Called from basePrep() early in the boot sequence, before enableLocalDNS().
# This allows DNS resolution to begin while the rest of basePrep installs packages and configures the node.
# The timer's systemd service reads LOCALDNS_CRITICAL_FQDNS from /etc/localdns/environment,
# so this function writes a minimal environment file before starting the timer.
# generateLocalDNSFiles() (called later by enableLocalDNS) overwrites it with the full content.
# removeAKSLocalDNSHostsSetupTimerOverride removes the optional systemd drop-in that
# overrides the default hosts-setup timer cadence.
removeAKSLocalDNSHostsSetupTimerOverride() {
    local hosts_setup_timer_override="$1"

    if [ ! -f "${hosts_setup_timer_override}" ]; then
        return 0
    fi

    rm -f "${hosts_setup_timer_override}"
    if systemctl daemon-reload; then
        echo "Restored default aks-localdns-hosts-setup timer refresh interval."
    else
        echo "Warning: Failed to reload systemd after removing ${hosts_setup_timer_override}"
    fi
}

enableAKSLocalDNSHostsSetup() {
    # Best-effort setup: log errors but never fail.
    # The corefile will fall back to the no-hosts variant if hosts file is empty.
    # Allow overriding paths for testing (via environment variables)
    local hosts_file="${AKS_LOCALDNS_HOSTS_FILE:-/etc/localdns/hosts}"
    local hosts_setup_script="${AKS_LOCALDNS_HOSTS_SETUP_SCRIPT:-/opt/azure/containers/aks-localdns-hosts-setup.sh}"
    local hosts_setup_service="${AKS_LOCALDNS_HOSTS_SETUP_SERVICE:-/etc/systemd/system/aks-localdns-hosts-setup.service}"
    local hosts_setup_timer="${AKS_LOCALDNS_HOSTS_SETUP_TIMER:-/etc/systemd/system/aks-localdns-hosts-setup.timer}"

    # Guard: verify required artifacts exist on this VHD.
    # Older VHDs (or certain build modes) may not include them.
    if [ ! -f "${hosts_setup_script}" ]; then
        echo "Warning: ${hosts_setup_script} not found on this VHD, skipping aks-localdns-hosts-setup"
        return 0
    fi
    if [ ! -x "${hosts_setup_script}" ]; then
        echo "Warning: ${hosts_setup_script} is not executable, skipping aks-localdns-hosts-setup"
        return 0
    fi
    if [ ! -f "${hosts_setup_service}" ]; then
        echo "Warning: ${hosts_setup_service} not found on this VHD, skipping aks-localdns-hosts-setup"
        return 0
    fi
    if [ ! -f "${hosts_setup_timer}" ]; then
        echo "Warning: ${hosts_setup_timer} not found on this VHD, skipping aks-localdns-hosts-setup"
        return 0
    fi

    # Verify LOCALDNS_CRITICAL_FQDNS is set before proceeding; if not, skip hosts setup.
    if [ -z "${LOCALDNS_CRITICAL_FQDNS:-}" ]; then
        echo "WARNING: LOCALDNS_CRITICAL_FQDNS is not set. RP did not pass critical FQDNs."
        echo "Skipping aks-localdns-hosts-setup. Corefile will fall back to version without hosts plugin."
        return 0
    fi

    # Write a minimal environment file so the systemd service (which reads from
    # /etc/localdns/environment via EnvironmentFile=) has LOCALDNS_CRITICAL_FQDNS available.
    # generateLocalDNSFiles() overwrites this later with the full content including corefiles.
    local env_file="/etc/localdns/environment"
    mkdir -p "$(dirname "${env_file}")"
    cat > "${env_file}" <<EOF
LOCALDNS_CRITICAL_FQDNS=${LOCALDNS_CRITICAL_FQDNS}
EOF
    chmod 0644 "${env_file}"

    # Create an empty hosts file so the localdns hosts plugin can start watching it
    # immediately. The file will be populated by aks-localdns-hosts-setup timer asynchronously.
    mkdir -p "$(dirname "${hosts_file}")"
    touch "${hosts_file}"
    chmod 0644 "${hosts_file}"

    local hosts_plugin_refresh_interval="${LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS:-}"
    local hosts_setup_timer_override="${hosts_setup_timer}.d/10-refresh-interval.conf"
    local min_hosts_plugin_refresh_interval_in_seconds=5

    if [ -z "${hosts_plugin_refresh_interval}" ]; then
        removeAKSLocalDNSHostsSetupTimerOverride "${hosts_setup_timer_override}"
    else
        local should_override_refresh_interval="false"
        case "${hosts_plugin_refresh_interval}" in
            *[!0-9]*)
                ;;
            *)
                should_override_refresh_interval="true"
                if [ "${hosts_plugin_refresh_interval}" -lt "${min_hosts_plugin_refresh_interval_in_seconds}" ]; then
                    echo "Warning: LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS must be >= ${min_hosts_plugin_refresh_interval_in_seconds}, got '${hosts_plugin_refresh_interval}'. Clamping to ${min_hosts_plugin_refresh_interval_in_seconds}s."
                    hosts_plugin_refresh_interval="${min_hosts_plugin_refresh_interval_in_seconds}"
                fi
                ;;
        esac

        if [ "${should_override_refresh_interval}" = "true" ]; then
            mkdir -p "$(dirname "${hosts_setup_timer_override}")"
            cat > "${hosts_setup_timer_override}" <<EOF
[Timer]
OnUnitActiveSec=${hosts_plugin_refresh_interval}s
AccuracySec=1s
EOF
            chmod 0644 "${hosts_setup_timer_override}"
            if grep -q "^OnUnitActiveSec=${hosts_plugin_refresh_interval}s$" "${hosts_setup_timer_override}" &&
                grep -q "^AccuracySec=1s$" "${hosts_setup_timer_override}"; then
                if systemctl daemon-reload; then
                    echo "Configured aks-localdns-hosts-setup timer refresh interval to ${hosts_plugin_refresh_interval}s."
                else
                    echo "Warning: Failed to reload systemd after updating ${hosts_setup_timer_override}"
                fi
            else
                echo "Warning: Failed to update ${hosts_setup_timer_override} with refresh interval ${hosts_plugin_refresh_interval}s"
            fi
        else
            echo "Warning: LOCALDNS_HOSTS_PLUGIN_REFRESH_INTERVAL_IN_SECONDS must be an integer, got '${hosts_plugin_refresh_interval}'. Using default timer interval."
            removeAKSLocalDNSHostsSetupTimerOverride "${hosts_setup_timer_override}"
        fi
    fi

    # Enable the timer for periodic refresh.
    # This will update the hosts file with fresh IPs from live DNS.
    echo "Enabling aks-localdns-hosts-setup timer..."
    if systemctlEnableAndStartNoBlock aks-localdns-hosts-setup.timer 30; then
        echo "aks-localdns-hosts-setup timer enabled successfully."
    else
        echo "Warning: Failed to enable aks-localdns-hosts-setup timer"
    fi
}

# disableAKSLocalDNSHostsSetup disables the hosts plugin on a node where it was previously enabled.
# Called from enableLocalDNS() when SHOULD_ENABLE_HOSTS_PLUGIN is not true.
# This handles the production rollback case where a customer disables the hosts plugin
# on an existing agentpool and AKS-RP re-runs CSE with SHOULD_ENABLE_HOSTS_PLUGIN=false.
# All operations are idempotent — safe to call when hosts plugin was never enabled.
disableAKSLocalDNSHostsSetup() {
    local hosts_file="${AKS_LOCALDNS_HOSTS_FILE:-/etc/localdns/hosts}"
    local hosts_setup_timer="${AKS_LOCALDNS_HOSTS_SETUP_TIMER:-/etc/systemd/system/aks-localdns-hosts-setup.timer}"
    local hosts_setup_timer_override="${hosts_setup_timer}.d/10-refresh-interval.conf"

    echo "disableAKSLocalDNSHostsSetup called, cleaning up hosts plugin state..."

    # Stop and disable the hosts-setup timer if it exists and is active.
    # This prevents further updates to the hosts file.
    if [ -f "${hosts_setup_timer}" ]; then
        systemctl disable --now aks-localdns-hosts-setup.timer 2>/dev/null || true
        echo "Disabled and stopped aks-localdns-hosts-setup.timer"
    else
        echo "aks-localdns-hosts-setup.timer not found on this VHD, skipping"
    fi

    removeAKSLocalDNSHostsSetupTimerOverride "${hosts_setup_timer_override}"

    # Remove the hosts file to clean up stale data.
    # select_localdns_corefile() selects based on SHOULD_ENABLE_HOSTS_PLUGIN,
    # so removing the file isn't strictly needed for corefile selection, but
    # it prevents CoreDNS from serving stale host entries if the feature is re-enabled later.
    if [ -f "${hosts_file}" ]; then
        rm -f "${hosts_file}"
        echo "Removed ${hosts_file}"
    else
        echo "${hosts_file} does not exist, skipping"
    fi

    echo "disableAKSLocalDNSHostsSetup complete"
}

#EOF
