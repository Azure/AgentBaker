#!/bin/bash
# Create comprehensive RX buffer error mock data for testing
# This script creates various network interface scenarios for testing rx_out_of_buffer detection

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create mock data directories
mkdir -p "$SCRIPT_DIR/mock-data"
mkdir -p "$SCRIPT_DIR/mock-commands/rx-buffer"

# Clean up any existing RX buffer test files
rm -rf "$SCRIPT_DIR"/mock-commands/rx-buffer/*
rm -rf "$SCRIPT_DIR"/mock-commands/ip-*
rm -rf "$SCRIPT_DIR"/mock-commands/ethtool-*
rm -rf "$SCRIPT_DIR"/mock-commands/jq-*

echo "Create RX buffer error mock commands"

# Scenario 1: Multiple PCI interfaces with high error rates
create_high_error_scenario() {
    
    # Mock ip command that returns PCI interfaces
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-high-errors" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 2,
    "ifname": "eth0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:03",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  },
  {
    "ifindex": 3,
    "ifname": "eth1",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:04",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  },
  {
    "ifindex": 1,
    "ifname": "lo",
    "flags": ["LOOPBACK", "UP", "LOWER_UP"],
    "mtu": 65536,
    "qdisc": "noqueue",
    "operstate": "UNKNOWN",
    "linkmode": "DEFAULT",
    "group": "default",
    "link_type": "loopback",
    "address": "00:00:00:00:00:00",
    "broadcast": "00:00:00:00:00:00"
  }
]
IP_OUTPUT
EOF
    
    # Mock ethtool for eth0 with high error rate
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth0-high-errors" <<'EOF'
#!/bin/bash
cat <<'ETHTOOL_OUTPUT'
NIC statistics:
     rx_packets: 2000000
     tx_packets: 1600000
     rx_bytes: 3000000000
     tx_bytes: 2400000000
     rx_out_of_buffer: 1150
     rx_errors: 5
     tx_errors: 2
     rx_dropped: 10
     tx_dropped: 8
     multicast: 50000
     collisions: 0
     rx_length_errors: 1
     rx_over_errors: 2
     rx_crc_errors: 1
     rx_frame_errors: 1
     rx_fifo_errors: 0
     rx_missed_errors: 145
     tx_aborted_errors: 0
     tx_carrier_errors: 0
     tx_fifo_errors: 0
     tx_heartbeat_errors: 0
ETHTOOL_OUTPUT
EOF
    
    # Mock ethtool for eth1 with very high error rate
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth1-high-errors" <<'EOF'
#!/bin/bash
cat <<'ETHTOOL_OUTPUT'
NIC statistics:
     rx_packets: 3000000
     tx_packets: 2400000
     rx_bytes: 4500000000
     tx_bytes: 3600000000
     rx_out_of_buffer: 1500
     rx_errors: 15
     tx_errors: 8
     rx_dropped: 25
     tx_dropped: 18
     multicast: 75000
     collisions: 0
     rx_length_errors: 3
     rx_over_errors: 5
     rx_crc_errors: 4
     rx_frame_errors: 3
     rx_fifo_errors: 0
     rx_missed_errors: 485
     tx_aborted_errors: 0
     tx_carrier_errors: 0
     tx_fifo_errors: 0
     tx_heartbeat_errors: 0
ETHTOOL_OUTPUT
EOF
}

# Scenario 2: Normal operation with low error rates
create_normal_scenario() {
    
    # Mock ip command that returns single PCI interface
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-normal" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 2,
    "ifname": "eth0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:03",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  }
]
IP_OUTPUT
EOF
    
    # Mock ethtool for eth0 with normal low error rate
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth0-normal" <<'EOF'
#!/bin/bash
cat <<'ETHTOOL_OUTPUT'
NIC statistics:
     rx_packets: 5000000
     tx_packets: 4000000
     rx_bytes: 7500000000
     tx_bytes: 6000000000
     rx_out_of_buffer: 5
     rx_errors: 2
     tx_errors: 1
     rx_dropped: 3
     tx_dropped: 2
     multicast: 100000
     collisions: 0
     rx_length_errors: 0
     rx_over_errors: 1
     rx_crc_errors: 1
     rx_frame_errors: 0
     rx_fifo_errors: 0
     rx_missed_errors: 3
     tx_aborted_errors: 0
     tx_carrier_errors: 0
     tx_fifo_errors: 0
     tx_heartbeat_errors: 0
ETHTOOL_OUTPUT
EOF
}

# Scenario 3: No PCI interfaces found
create_no_interfaces_scenario() {
    
    # Mock ip command that returns no PCI interfaces
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-no-pci" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 1,
    "ifname": "lo",
    "flags": ["LOOPBACK", "UP", "LOWER_UP"],
    "mtu": 65536,
    "qdisc": "noqueue",
    "operstate": "UNKNOWN",
    "linkmode": "DEFAULT",
    "group": "default",
    "link_type": "loopback",
    "address": "00:00:00:00:00:00",
    "broadcast": "00:00:00:00:00:00"
  },
  {
    "ifindex": 3,
    "ifname": "docker0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "noqueue",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "link_type": "bridge",
    "address": "02:42:ac:11:00:01",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "virtual"
  }
]
IP_OUTPUT
EOF
}

# Scenario 4: Missing rx_out_of_buffer metric
create_missing_metrics_scenario() {
    
    # Mock ip command with PCI interface
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-missing-metrics" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 2,
    "ifname": "eth0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:03",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  }
]
IP_OUTPUT
EOF
    
    # Mock ethtool with missing rx_out_of_buffer metric
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth0-missing-metrics" <<'EOF'
#!/bin/bash
cat <<'ETHTOOL_OUTPUT'
NIC statistics:
     rx_packets: 1000000
     tx_packets: 800000
     rx_bytes: 1500000000
     tx_bytes: 1200000000
     rx_errors: 5
     tx_errors: 2
     rx_dropped: 10
     tx_dropped: 8
     multicast: 50000
     collisions: 0
ETHTOOL_OUTPUT
EOF
}

# Scenario 5: Invalid/non-numeric metric values
create_invalid_data_scenario() {
    
    # Mock ip command with PCI interface
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-invalid-data" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 2,
    "ifname": "eth0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:03",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  }
]
IP_OUTPUT
EOF
    
    # Mock ethtool with invalid metric values
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth0-invalid-data" <<'EOF'
#!/bin/bash
cat <<'ETHTOOL_OUTPUT'
NIC statistics:
     rx_packets: invalid_value
     tx_packets: 800000
     rx_bytes: 1500000000
     tx_bytes: 1200000000
     rx_out_of_buffer: not_a_number
     rx_errors: 5
     tx_errors: 2
     rx_dropped: 10
     tx_dropped: 8
     multicast: 50000
     collisions: 0
ETHTOOL_OUTPUT
EOF
}

# Scenario 6: ethtool command failure
create_ethtool_failure_scenario() {
    
    # Mock ip command with PCI interface
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-ethtool-failure" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 2,
    "ifname": "eth0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:03",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  }
]
IP_OUTPUT
EOF
    
    # Mock ethtool that fails
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth0-failure" <<'EOF'
#!/bin/bash
echo "Cannot get device statistics: Operation not supported" >&2
exit 1
EOF
}

# Scenario 7: Command dependency failures
create_command_failures() {
    
    # Mock jq that fails
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/jq-failure" <<'EOF'
#!/bin/bash
echo "jq: error: parse error: Invalid JSON" >&2
exit 1
EOF
    
    # Mock ip that fails
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-failure" <<'EOF'
#!/bin/bash
echo "RTNETLINK answers: Operation not permitted" >&2
exit 1
EOF
    
}

# Scenario 8: Zero packet delta (no traffic)
create_no_traffic_scenario() {
    
    # Mock ip command with PCI interface
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip-no-traffic" <<'EOF'
#!/bin/bash
cat <<'IP_OUTPUT'
[
  {
    "ifindex": 2,
    "ifname": "eth0",
    "flags": ["BROADCAST", "MULTICAST", "UP", "LOWER_UP"],
    "mtu": 1500,
    "qdisc": "mq",
    "operstate": "UP",
    "linkmode": "DEFAULT",
    "group": "default",
    "txqlen": 1000,
    "link_type": "ether",
    "address": "00:15:5d:01:02:03",
    "broadcast": "ff:ff:ff:ff:ff:ff",
    "parentbus": "pci"
  }
]
IP_OUTPUT
EOF
    
    # Mock ethtool with same values as baseline (no traffic)
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool-eth0-no-traffic" <<'EOF'
#!/bin/bash
cat <<'ETHTOOL_OUTPUT'
NIC statistics:
     rx_packets: 1000000
     tx_packets: 800000
     rx_bytes: 1500000000
     tx_bytes: 1200000000
     rx_out_of_buffer: 10
     rx_errors: 5
     tx_errors: 2
     rx_dropped: 10
     tx_dropped: 8
     multicast: 50000
     collisions: 0
ETHTOOL_OUTPUT
EOF
}

# Create RX buffer specific jq mock
create_rx_jq_mock() {
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/jq" <<'EOF'
#!/bin/bash
# RX buffer specific jq mock
# Extract interface names from JSON where parentbus == "pci"

case "$*" in
    *'select(.parentbus == "pci")'*)
        # Filter PCI interfaces and extract ifname
        python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    pci_interfaces = [item['ifname'] for item in data if item.get('parentbus') == 'pci']
    for iface in pci_interfaces:
        print(iface)
except:
    pass
"
        ;;
    *)
        # For RX buffer tests, fall back to real jq if available
        if command -v /usr/bin/jq > /dev/null 2>&1; then
            exec /usr/bin/jq "$@"
        else
            echo "jq: command not found" >&2
            exit 127
        fi
        ;;
esac
EOF
}

# Create dispatcher commands in rx-buffer directory
create_rx_dispatchers() {
    # Create ethtool dispatcher for rx-buffer directory
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ethtool" <<'EOF'
#!/bin/bash
# Ethtool dispatcher for RX buffer tests
INTERFACE="$2"  # $1 is -S, $2 is interface name
SCENARIO="${RX_BUFFER_SCENARIO:-normal}"

# Look for scenario-specific mock in rx-buffer directory
if [ -f "/mock-commands/rx-buffer/ethtool-$INTERFACE-$SCENARIO" ]; then
    exec "/mock-commands/rx-buffer/ethtool-$INTERFACE-$SCENARIO"
fi

# Default failure
echo "Cannot get device statistics: No such device" >&2
exit 1
EOF

    # Create ip dispatcher for rx-buffer directory  
    cat > "$SCRIPT_DIR/mock-commands/rx-buffer/ip" <<'EOF'
#!/bin/bash
# IP dispatcher for RX buffer tests
SCENARIO="${RX_BUFFER_SCENARIO:-normal}"

case "$*" in
    "-j -d link show")
        # Return appropriate interface list based on scenario
        case "$SCENARIO" in
            "high-errors")
                exec "/mock-commands/rx-buffer/ip-high-errors"
                ;;
            "normal")
                exec "/mock-commands/rx-buffer/ip-normal"
                ;;
            "no-pci")
                exec "/mock-commands/rx-buffer/ip-no-pci"
                ;;
            "missing-metrics"|"invalid-data"|"failure"|"no-traffic")
                exec "/mock-commands/rx-buffer/ip-missing-metrics"
                ;;
        esac
        ;;
    "-s -s link show dev"*)
        # Mock detailed link stats for diagnostics
        INTERFACE=$(echo "$*" | awk '{print $NF}')
        cat <<LINK_STATS
2: $INTERFACE: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP mode DEFAULT group default qlen 1000
    link/ether 00:15:5d:01:02:03 brd ff:ff:ff:ff:ff:ff
    RX: bytes  packets  errors  dropped overrun mcast   
    1500000000 1000000  5       10      0       50000   
    TX: bytes  packets  errors  dropped carrier collsns 
    1200000000 800000   2       8       0       0       
LINK_STATS
        ;;
esac
EOF
}

# Create mock state data directories for stateful testing
create_mock_state_directories() {
    
    # Scenario 1: Baseline (first run) - empty state directory
    local baseline_dir="$SCRIPT_DIR/mock-data/rx-buffer-baseline"
    mkdir -p "$baseline_dir/var/lib/node-problem-detector/rx-buffer-check"
    # Ensure clean state for baseline test - remove any existing state files
    rm -f "$baseline_dir/var/lib/node-problem-detector/rx-buffer-check/"*.state
    
    # Scenario 2: Normal operation with previous state (low error rates)
    local normal_dir="$SCRIPT_DIR/mock-data/rx-buffer-normal"
    mkdir -p "$normal_dir/var/lib/node-problem-detector/rx-buffer-check"
    
    # Always recreate state file for eth0 with previous low error counts (tests may have overwritten it)
    # Format: timestamp buffer_errors packets
    local prev_timestamp=$(($(date +%s) - 60))  # 1 minute ago
    echo "$prev_timestamp 5 5000000" > "$normal_dir/var/lib/node-problem-detector/rx-buffer-check/eth0.state"
    
    # Scenario 3: High error progression
    local high_errors_dir="$SCRIPT_DIR/mock-data/rx-buffer-high-errors"
    mkdir -p "$high_errors_dir/var/lib/node-problem-detector/rx-buffer-check"
    
    # Always recreate state files to ensure clean test data (tests may have overwritten them)
    # eth0: previous state with low errors (will show high delta when combined with high-errors ethtool data)
    # Current: 1150 errors, 2000000 packets -> Previous: 150 errors, 1000000 packets -> Delta: 1000/1000000 = 0.001 (above 0.0001 threshold)
    echo "$prev_timestamp 150 1000000" > "$high_errors_dir/var/lib/node-problem-detector/rx-buffer-check/eth0.state"
    # eth1: similar pattern with even higher ratio
    # Current: 1500 errors, 3000000 packets -> Previous: 500 errors, 2000000 packets -> Delta: 1000/1000000 = 0.001 (above threshold)
    echo "$prev_timestamp 500 2000000" > "$high_errors_dir/var/lib/node-problem-detector/rx-buffer-check/eth1.state"
    
    # Scenario 4: No traffic scenario (same packet counts)
    local no_traffic_dir="$SCRIPT_DIR/mock-data/rx-buffer-no-traffic"
    mkdir -p "$no_traffic_dir/var/lib/node-problem-detector/rx-buffer-check"
    
    # State file with exactly same packet count as current (will result in zero delta)
    echo "$prev_timestamp 10 1000000" > "$no_traffic_dir/var/lib/node-problem-detector/rx-buffer-check/eth0.state"
    
    # Scenario 5: Error cases - corrupted state files
    local corrupted_dir="$SCRIPT_DIR/mock-data/rx-buffer-corrupted-state"
    mkdir -p "$corrupted_dir/var/lib/node-problem-detector/rx-buffer-check"
    
    # Create corrupted state file
    echo "invalid state data" > "$corrupted_dir/var/lib/node-problem-detector/rx-buffer-check/eth0.state"
    
    # Scenario 6: Missing permissions
    local readonly_dir="$SCRIPT_DIR/mock-data/rx-buffer-readonly-state"
    mkdir -p "$readonly_dir/var/lib/node-problem-detector/rx-buffer-check"
    
    # Create readonly directory to simulate permission issues
    chmod 444 "$readonly_dir/var/lib/node-problem-detector/rx-buffer-check" 2>/dev/null || true
}

# Run all scenario creation functions
create_high_error_scenario
create_normal_scenario
create_no_interfaces_scenario
create_missing_metrics_scenario
create_invalid_data_scenario
create_ethtool_failure_scenario
create_command_failures
create_no_traffic_scenario
create_rx_jq_mock
create_rx_dispatchers
create_mock_state_directories


# Make all mock commands executable
# Use -exec to handle filenames safely without depending on xargs word splitting
find "$SCRIPT_DIR/mock-commands" \( -name "ip-*" -o -name "ethtool-*" \) -exec chmod +x {} +
find "$SCRIPT_DIR/mock-commands/rx-buffer" -type f -exec chmod +x {} +

echo "RX buffer error mock data creation complete"
echo "Created scenarios: high-errors, normal, no-pci, missing-metrics, invalid-data, failure, no-traffic"
echo "Created mock state directories: baseline, normal, high-errors, no-traffic, corrupted-state, readonly-state"