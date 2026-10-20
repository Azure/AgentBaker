#!/bin/bash

configPrivateClusterHosts() {
    mkdir -p /etc/systemd/system/reconcile-private-hosts.service.d/
    touch /etc/systemd/system/reconcile-private-hosts.service.d/10-fqdn.conf
    tee /etc/systemd/system/reconcile-private-hosts.service.d/10-fqdn.conf > /dev/null <<EOF
[Service]
Environment="KUBE_API_SERVER_NAME=${API_SERVER_NAME}"
EOF
  systemctlEnableAndStart reconcile-private-hosts 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

configureSystemdUseDomains() {
    NETWORK_CONFIG_FILE="/etc/systemd/networkd.conf"

    if awk '/^\[DHCPv4\]/{flag=1; next} /^\[/{flag=0} flag && /#UseDomains=no/' "$NETWORK_CONFIG_FILE"; then
        sed -i '/^\[DHCPv4\]/,/^\[/ s/#UseDomains=no/UseDomains=yes/' $NETWORK_CONFIG_FILE
    fi

    if [ "${IPV6_DUAL_STACK_ENABLED}" = "true" ]; then
        if awk '/^\[DHCPv6\]/{flag=1; next} /^\[/{flag=0} flag && /#UseDomains=no/' "$NETWORK_CONFIG_FILE"; then
            sed -i '/^\[DHCPv6\]/,/^\[/ s/#UseDomains=no/UseDomains=yes/' $NETWORK_CONFIG_FILE
        fi
    fi

    # Restart systemd networkd service
    systemctl restart systemd-networkd

    # Restart rsyslog service to display the correct hostname in log
    systemctl restart rsyslog
}

configureCNI() {
    # needed for bridge iptables rules and customer-configured conntrack sysctls
    retrycmd_if_failure 120 5 25 modprobe -a br_netfilter nf_conntrack || exit $ERR_MODPROBE_FAIL
    echo -n "br_netfilter" > /etc/modules-load.d/br_netfilter.conf
    echo -n "nf_conntrack" > /etc/modules-load.d/nf_conntrack.conf
    configureCNIIPTables
}

configureCNIIPTables() {
    if [ "${NETWORK_PLUGIN}" = "azure" ]; then
        mv $CNI_BIN_DIR/10-azure.conflist $CNI_CONFIG_DIR/
        chmod 600 $CNI_CONFIG_DIR/10-azure.conflist
        if [ "${NETWORK_POLICY}" = "calico" ]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        elif [ -n "${NETWORK_POLICY}" ] || [ "${NETWORK_POLICY}" = "none" ] && [ "${NETWORK_MODE}" = "transparent" ]; then
          sed -i 's#"mode":"bridge"#"mode":"transparent"#g' $CNI_CONFIG_DIR/10-azure.conflist
        fi
        /sbin/ebtables -t nat --list
    fi
}

disableSystemdResolved() {
    ls -ltr /etc/resolv.conf
    cat /etc/resolv.conf
    UBUNTU_RELEASE=$(lsb_release -r -s 2>/dev/null || echo "")
    if [ "${UBUNTU_RELEASE}" = "20.04" ] || [ "${UBUNTU_RELEASE}" = "22.04" ] || [ "${UBUNTU_RELEASE}" = "24.04" ] || [ "${UBUNTU_RELEASE}" = "26.04" ]; then
        echo "Ignoring systemd-resolved query service but using its resolv.conf file"
        echo "This is the simplest approach to workaround resolved issues without completely uninstall it"
        [ -f /run/systemd/resolve/resolv.conf ] && ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
        ls -ltr /etc/resolv.conf
        cat /etc/resolv.conf
    fi
}

ensureNoDupOnPromiscuBridge() {
    systemctlEnableAndStart ensure-no-dup 30 || exit $ERR_SYSTEMCTL_START_FAIL
}

ensureDHCPv6() {
    systemctlEnableAndStart dhcpv6 30 || exit $ERR_SYSTEMCTL_START_FAIL
    retrycmd_if_failure 120 5 25 modprobe ip6_tables || exit $ERR_MODPROBE_FAIL
}

ensureAzureNetworkConfig() {
    # Reload udev rules to pick up the new azure-network rules
    udevadm control --reload-rules

    # Trigger udev to detect and populate network interfaces
    echo "Triggering udev for network devices..."
    udevadm trigger --subsystem-match=net --action=add

    # Give udev time to process and trigger the systemd service
    udevadm settle --timeout=10
}

configureSecondaryNICs() {
    # Read NIC list from cached IMDS metadata.
    # IMDS reports all NICs attached to the VM, including secondary ones.
    # On a vanilla Azure VM cloud-init would configure these automatically,
    # but AKS VHDs disable cloud-init network management (apply_network_config: false
    # on Ubuntu, hardcoded Name=eth0 in networkd on AzureLinux). This function
    # fills the gap for Standard-type secondary NICs that need OS-level DHCP.
    local nic_count
    nic_count=$(jq -r '.network.interface | length' "$IMDS_INSTANCE_METADATA_CACHE_FILE") || {
        echo "Failed to parse NIC count from IMDS cache file: $IMDS_INSTANCE_METADATA_CACHE_FILE" >&2
        return $ERR_SECONDARY_NIC_CONFIG_FAIL
    }
    if ! [ "$nic_count" -eq "$nic_count" ] 2>/dev/null; then
        echo "Invalid NIC count '$nic_count' from IMDS cache file: $IMDS_INSTANCE_METADATA_CACHE_FILE" >&2
        return $ERR_SECONDARY_NIC_CONFIG_FAIL
    fi

    if [ "$nic_count" -le 1 ]; then
        echo "No secondary NICs detected, skipping"
        return 0
    fi

    echo "Detected $nic_count NICs, configuring secondary interfaces..."

    local is_netplan=false
    # Ubuntu netplan primary NIC default metric is ~100,
    # so secondary NICs use 200, 300, etc. (base 100).
    # AzureLinux/Mariner networkd primary NIC DHCP default metric is 1024,
    # so secondary NICs must use >1024 to avoid asymmetric routing.
    local metric_base=2000
    if isUbuntu; then
        is_netplan=true
        metric_base=100
    fi

    # Collect resolved interface names for networkctl up calls after reload.
    local secondary_ifaces=""

    for i in $(seq 1 $((nic_count - 1))); do
        local mac
        mac=$(jq -r ".network.interface[$i].macAddress" "$IMDS_INSTANCE_METADATA_CACHE_FILE")
        # IMDS returns MAC without colons (e.g. "7CED8D8A4DCE"), convert to colon-separated lowercase
        mac=$(echo "$mac" | sed 's/\(..\)/\1:/g; s/:$//' | tr '[:upper:]' '[:lower:]')
        local metric=$(( metric_base + i * 100 ))

        # Resolve the actual kernel interface name by matching the MAC address against
        # /sys/class/net/*/address. This is necessary because SR-IOV virtual functions
        # can claim eth1 before the secondary NIC is attached, so we cannot assume
        # secondary NIC $i is eth${i}.
        local iface_name=""
        for sys_path in /sys/class/net/*/address; do
            local sys_dir
            sys_dir=$(dirname "$sys_path")
            # Skip SR-IOV VFs (enslaved interfaces) — they share the MAC of their master
            # but don't hold an IP address themselves.
            [ -e "$sys_dir/master" ] && continue
            if [ "$(cat "$sys_path" 2>/dev/null | tr '[:upper:]' '[:lower:]')" = "$mac" ]; then
                iface_name=$(basename "$sys_dir")
                break
            fi
        done
        if [ -z "$iface_name" ]; then
            echo "Warning: could not find interface for MAC ${mac}, using eth${i} as fallback for logging"
            iface_name="eth${i}"
        fi

        # Check if this NIC has IPv6 addresses configured in IMDS
        local ipv6_count
        ipv6_count=$(jq -r ".network.interface[$i].ipv6.ipAddress | length" "$IMDS_INSTANCE_METADATA_CACHE_FILE")
        local has_ipv6=false
        if [ "$ipv6_count" -gt 0 ]; then
            has_ipv6=true
        fi

        if [ "$is_netplan" = true ]; then
            # Ubuntu: generate netplan config for the secondary NIC.
            # Match by MAC address so we configure the right device regardless of
            # kernel naming (SR-IOV VFs can shift ethN indices).
            local netplan_file="/etc/netplan/60-secondary-nic-${i}.yaml"
            {
                cat <<NETPLAN_EOF
network:
  ethernets:
    secondary-nic-${i}:
      match:
        macaddress: "${mac}"
      dhcp4: true
      dhcp4-overrides:
        route-metric: ${metric}
        use-dns: false
NETPLAN_EOF
                if [ "$has_ipv6" = true ]; then
                    cat <<NETPLAN_V6_EOF
      dhcp6: true
      dhcp6-overrides:
        route-metric: ${metric}
        use-dns: false
NETPLAN_V6_EOF
                fi
            } > "$netplan_file"
            chmod 600 "$netplan_file"
        else
            # AzureLinux/Mariner: generate networkd .network unit.
            # Prefix 10- so it takes precedence over the VHD's 99-dhcp-en.network.
            # Match by MAC address so we configure the right device regardless of
            # kernel naming (SR-IOV VFs can shift ethN indices).
            local networkd_file="/etc/systemd/network/10-secondary-nic-${i}.network"
            if [ "$has_ipv6" = true ]; then
                cat > "$networkd_file" <<NETWORKD_EOF
[Match]
MACAddress=${mac}

[Network]
DHCP=yes
IPv6AcceptRA=yes

[DHCPv4]
RouteMetric=${metric}
UseDNS=false
UseDomains=false
SendRelease=false

[DHCPv6]
RouteMetric=${metric}
UseDNS=false
UseDomains=false
NETWORKD_EOF
            else
                cat > "$networkd_file" <<NETWORKD_EOF
[Match]
MACAddress=${mac}

[Network]
DHCP=ipv4

[DHCPv4]
RouteMetric=${metric}
UseDNS=false
UseDomains=false
SendRelease=false
NETWORKD_EOF
            fi
        fi

        echo "Configured secondary NIC ${iface_name} (mac=${mac}, metric=${metric})"
        # Only track interfaces that actually exist and are not SR-IOV VFs for
        # the networkctl up loop. The .network files match by MAC and will
        # auto-activate once the real interface appears, so a missing or VF
        # interface should not block or fail the reload path.
        if [ -d "/sys/class/net/${iface_name}" ] && [ ! -e "/sys/class/net/${iface_name}/master" ]; then
            secondary_ifaces="${secondary_ifaces} ${iface_name}"
        fi
    done

    # Apply all configs in a single operation to avoid repeated network restarts.
    if [ "$is_netplan" = true ]; then
        if ! retrycmd_if_failure 5 3 10 netplan apply; then
            echo "Failed to apply netplan config for secondary NICs" >&2
            return $ERR_SECONDARY_NIC_CONFIG_FAIL
        fi
    else
        if isACL; then
            # On ACL (Flatcar-based), networkctl's control socket is often broken
            # ("Transport endpoint is not connected") because systemd-networkd's
            # varlink socket is torn down during the initrd→real-root pivot and may
            # never recover. Bypass the broken socket entirely by restarting the
            # systemd-networkd service, which talks to PID-1's (always-working)
            # D-Bus socket instead. The restart re-reads all .network files and
            # brings up matching interfaces automatically — no separate
            # networkctl up/reload calls needed.
            if ! retrycmd_if_failure 5 5 30 systemctl restart systemd-networkd; then
                echo "Failed to restart systemd-networkd for secondary NICs" >&2
                return $ERR_SECONDARY_NIC_CONFIG_FAIL
            fi
        else
            local reload_retries=5 reload_sleep=3
            if ! retrycmd_if_failure $reload_retries $reload_sleep 10 networkctl reload; then
                echo "Failed to reload networkd for secondary NICs" >&2
                return $ERR_SECONDARY_NIC_CONFIG_FAIL
            fi
            for iface in $secondary_ifaces; do
                if ! retrycmd_if_failure $reload_retries $reload_sleep 10 networkctl up "$iface"; then
                    echo "Failed to bring up ${iface}" >&2
                    return $ERR_SECONDARY_NIC_CONFIG_FAIL
                fi
            done
        fi
    fi
}

#EOF
