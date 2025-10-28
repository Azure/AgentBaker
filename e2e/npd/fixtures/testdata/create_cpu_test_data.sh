#!/bin/bash
# test/node-problem-detector/docker/create_cpu_test_data.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# High CPU Pressure Scenario
create_high_pressure() {
    echo "Create high CPU pressure mock data"
    local dir="$SCRIPT_DIR/mock-data/cpu-high-pressure"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    
    # /proc/stat - showing high CPU usage (80% busy)
    cat > "$dir/proc/stat" <<'EOF'
cpu  8000000 0 2000000 2000000 0 0 0 0 0 0
cpu0 4000000 0 1000000 1000000 0 0 0 0 0 0
cpu1 4000000 0 1000000 1000000 0 0 0 0 0 0
EOF

    # /proc/cpuinfo - 2 CPUs
    cat > "$dir/proc/cpuinfo" <<'EOF'
processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 79
model name	: Intel(R) Xeon(R) CPU E5-2673 v4 @ 2.30GHz

processor	: 1
vendor_id	: GenuineIntel
cpu family	: 6
model		: 79
model name	: Intel(R) Xeon(R) CPU E5-2673 v4 @ 2.30GHz
EOF

    # /proc/uptime
    echo "3600.00 7200.00" > "$dir/proc/uptime"

    # PSI metrics showing high pressure
    cat > "$dir/sys/fs/cgroup/cpu.pressure" <<'EOF'
some avg10=35.00 avg60=30.00 avg300=25.00 total=123456789
full avg10=15.00 avg60=12.00 avg300=10.00 total=23456789
EOF
}

# Normal CPU Scenario
create_normal() {
    echo "Create normal CPU mock data"
    local dir="$SCRIPT_DIR/mock-data/cpu-normal"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    
    # /proc/stat - showing normal CPU usage (20% busy)
    cat > "$dir/proc/stat" <<'EOF'
cpu  2000000 0 500000 8000000 0 0 0 0 0 0
cpu0 1000000 0 250000 4000000 0 0 0 0 0 0
cpu1 1000000 0 250000 4000000 0 0 0 0 0 0
EOF

    # /proc/cpuinfo - 2 CPUs (same as high pressure)
    cat > "$dir/proc/cpuinfo" <<'EOF'
processor	: 0
vendor_id	: GenuineIntel
cpu family	: 6
model		: 79
model name	: Intel(R) Xeon(R) CPU E5-2673 v4 @ 2.30GHz

processor	: 1
vendor_id	: GenuineIntel
cpu family	: 6
model		: 79
model name	: Intel(R) Xeon(R) CPU E5-2673 v4 @ 2.30GHz
EOF

    # /proc/uptime
    echo "3600.00 7200.00" > "$dir/proc/uptime"

    # PSI metrics showing low pressure
    cat > "$dir/sys/fs/cgroup/cpu.pressure" <<'EOF'
some avg10=5.00 avg60=8.00 avg300=10.00 total=123456789
full avg10=2.00 avg60=3.00 avg300=4.00 total=23456789
EOF
}

# Create additional mock data scenarios (moved from test script)
create_additional_mock_data() {
    echo "Create additional mock data scenarios"
    
    # High IO pressure scenario
    mkdir -p "$SCRIPT_DIR/mock-data/high-io-pressure/sys/fs/cgroup"
    cat > "$SCRIPT_DIR/mock-data/high-io-pressure/sys/fs/cgroup/cpu.pressure" <<EOF
some avg10=15.00 avg60=25.00 avg300=30.00 total=12345678
full avg10=0.50 avg60=2.00 avg300=5.00 total=987654
EOF
    cat > "$SCRIPT_DIR/mock-data/high-io-pressure/sys/fs/cgroup/io.pressure" <<EOF
some avg10=50.00 avg60=60.00 avg300=70.00 total=23456789
full avg10=10.00 avg60=15.00 avg300=20.00 total=1234567
EOF
    
    # CPU throttling scenario
    mkdir -p "$SCRIPT_DIR/mock-data/cpu-throttling/sys/fs/cgroup"
    cat > "$SCRIPT_DIR/mock-data/cpu-throttling/sys/fs/cgroup/cpu.pressure" <<EOF
some avg10=10.00 avg60=15.00 avg300=20.00 total=12345678
full avg10=0.50 avg60=2.00 avg300=5.00 total=987654
EOF
    cat > "$SCRIPT_DIR/mock-data/cpu-throttling/sys/fs/cgroup/cpu.stat" <<EOF
usage_usec 123456789
user_usec 12345678
system_usec 2345678
nr_periods 100
nr_throttled 50
throttled_usec 5000000
EOF
    
    # High iowait scenario
    mkdir -p "$SCRIPT_DIR/mock-data/high-iowait/proc"
    cat > "$SCRIPT_DIR/mock-data/high-iowait/proc/loadavg" <<EOF
2.50 2.30 2.10 2/200 12345
EOF
}

# Create isolated test scenarios (moved from test script)
create_isolated_test_scenarios() {
    echo "Create isolated test scenarios"
    
    # Scenario 1: ONLY PSI CPU pressure (no IO, no iowait, no steal)
    mkdir -p "$SCRIPT_DIR/mock-data/psi-cpu-only/proc" "$SCRIPT_DIR/mock-data/psi-cpu-only/sys/fs/cgroup"
    cat > "$SCRIPT_DIR/mock-data/psi-cpu-only/proc/loadavg" <<EOF
1.50 1.30 1.10 2/200 12345
EOF
    cat > "$SCRIPT_DIR/mock-data/psi-cpu-only/sys/fs/cgroup/cpu.pressure" <<EOF
some avg10=70.00 avg60=75.00 avg300=80.00 total=12345678
full avg10=10.00 avg60=15.00 avg300=20.00 total=987654
EOF
    
    # Scenario 2: ONLY PSI IO pressure (no CPU PSI, no iowait, no steal)  
    mkdir -p "$SCRIPT_DIR/mock-data/psi-io-only/proc" "$SCRIPT_DIR/mock-data/psi-io-only/sys/fs/cgroup"
    cat > "$SCRIPT_DIR/mock-data/psi-io-only/proc/loadavg" <<EOF
1.50 1.30 1.10 2/200 12345
EOF
    cat > "$SCRIPT_DIR/mock-data/psi-io-only/sys/fs/cgroup/cpu.pressure" <<EOF
some avg10=10.00 avg60=15.00 avg300=20.00 total=12345678
full avg10=1.00 avg60=2.00 avg300=3.00 total=987654
EOF
    cat > "$SCRIPT_DIR/mock-data/psi-io-only/sys/fs/cgroup/io.pressure" <<EOF
some avg10=60.00 avg60=70.00 avg300=80.00 total=23456789
full avg10=20.00 avg60=25.00 avg300=30.00 total=1234567
EOF
    
    # Scenario 3: ONLY high iowait (no PSI pressure)
    mkdir -p "$SCRIPT_DIR/mock-data/iowait-only/proc" "$SCRIPT_DIR/mock-data/iowait-only/sys/fs/cgroup"
    cat > "$SCRIPT_DIR/mock-data/iowait-only/proc/loadavg" <<EOF
2.50 2.30 2.10 2/200 12345
EOF
    cat > "$SCRIPT_DIR/mock-data/iowait-only/sys/fs/cgroup/cpu.pressure" <<EOF
some avg10=5.00 avg60=8.00 avg300=10.00 total=12345678
full avg10=1.00 avg60=2.00 avg300=3.00 total=987654
EOF
    cat > "$SCRIPT_DIR/mock-data/iowait-only/sys/fs/cgroup/io.pressure" <<EOF
some avg10=5.00 avg60=10.00 avg300=15.00 total=12345678
full avg10=0.50 avg60=2.00 avg300=5.00 total=987654
EOF
    
    # Scenario 4: NO pressure detected anywhere (baseline)
    mkdir -p "$SCRIPT_DIR/mock-data/no-pressure/proc" "$SCRIPT_DIR/mock-data/no-pressure/sys/fs/cgroup"
    cat > "$SCRIPT_DIR/mock-data/no-pressure/proc/loadavg" <<EOF
1.50 1.30 1.10 2/200 12345
EOF
    cat > "$SCRIPT_DIR/mock-data/no-pressure/sys/fs/cgroup/cpu.pressure" <<EOF
some avg10=5.00 avg60=8.00 avg300=10.00 total=12345678
full avg10=1.00 avg60=2.00 avg300=3.00 total=987654
EOF
    cat > "$SCRIPT_DIR/mock-data/no-pressure/sys/fs/cgroup/io.pressure" <<EOF
some avg10=15.00 avg60=20.00 avg300=25.00 total=23456789
full avg10=5.00 avg60=8.00 avg300=10.00 total=1234567
EOF
}

# Create mock commands (updated to include all variants)
create_mock_commands() {
    echo "Create mock commands"
    local cmd_dir="$SCRIPT_DIR/mock-commands/cpu"
    mkdir -p "$cmd_dir"
    
    # Remove any existing files/directories that might conflict
    # Safe removal; ensure cmd_dir is set and not empty to avoid accidental root deletion
    rm -rf "${cmd_dir:?}"/*
    
    # Mock mpstat for high CPU
    cat > "$cmd_dir/mpstat-high" <<'EOF'
#!/bin/bash
cat <<END
Linux 5.4.0-1064-azure (test-node) 	01/01/2024 	_x86_64_	(2 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   45.00    0.00   35.00    0.00    0.00    0.00    0.00    0.00    0.00   20.00
END
EOF

    # Mock mpstat for normal CPU
    cat > "$cmd_dir/mpstat-normal" <<'EOF'
#!/bin/bash
cat <<END
Linux 5.4.0-1064-azure (test-node) 	01/01/2024 	_x86_64_	(2 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   10.00    0.00   10.00    0.00    0.00    0.00    0.00    0.00    0.00   80.00
END
EOF

    # Mock mpstat for high iowait
    cat > "$cmd_dir/mpstat-high-iowait" <<'EOF'
#!/bin/bash
cat <<'MPSTAT_EOF'
Linux 5.4.0-1043-azure (test-node)   01/01/70        _x86_64_        (4 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   10.50    0.00    5.20   25.30    0.00    2.10    1.50    0.00    0.00   55.40
MPSTAT_EOF
EOF

    # Mock mpstat for high steal time
    cat > "$cmd_dir/mpstat-high-steal" <<'EOF'
#!/bin/bash
cat <<'MPSTAT_EOF'
Linux 5.4.0-1043-azure (test-node)   01/01/70        _x86_64_        (4 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   30.50    0.00   10.20    5.30    0.00    2.10   15.50    0.00    0.00   36.40
MPSTAT_EOF
EOF

    # Mock mpstat with low iowait to avoid iowait pressure
    cat > "$cmd_dir/mpstat-low-iowait" <<'EOF'
#!/bin/bash
cat <<'MPSTAT_EOF'
Linux 5.4.0-1043-azure (test-node)   01/01/70        _x86_64_        (4 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   60.00    0.00   20.00    5.00    0.00    2.00    3.00    0.00    0.00   10.00
MPSTAT_EOF
EOF

    # Mock mpstat with HIGH iowait only
    cat > "$cmd_dir/mpstat-iowait-only" <<'EOF'
#!/bin/bash
cat <<'MPSTAT_EOF'
Linux 5.4.0-1043-azure (test-node)   01/01/70        _x86_64_        (4 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   20.00    0.00   10.00   35.00    0.00    2.00    3.00    0.00    0.00   30.00
MPSTAT_EOF
EOF

    # Mock mpstat with all low values
    cat > "$cmd_dir/mpstat-all-low" <<'EOF'
#!/bin/bash
cat <<'MPSTAT_EOF'
Linux 5.4.0-1043-azure (test-node)   01/01/70        _x86_64_        (4 CPU)

Average:     CPU    %usr   %nice    %sys %iowait    %irq   %soft  %steal  %guest  %gnice   %idle
Average:     all   30.00    0.00   15.00    5.00    0.00    2.00    3.00    0.00    0.00   45.00
MPSTAT_EOF
EOF

    # Edge case: missing mpstat - create a non-executable file
    echo "# This simulates mpstat not being available" > "$cmd_dir/mpstat-missing"
    chmod -x "$cmd_dir/mpstat-missing" 2>/dev/null || true

    # Mock iotop
    cat > "$cmd_dir/iotop" <<'EOF'
#!/bin/bash
echo "Total DISK READ: 0.00 B/s | Total DISK WRITE: 0.00 B/s"
echo "  PID  USER      DISK READ  DISK WRITE  SWAPIN     IO%  COMMAND"
echo " 1234  root      0.00 B/s    0.00 B/s    0.00 %   0.00 % test-process"
EOF

    # Create mpstat dispatcher for cpu directory
    cat > "$cmd_dir/mpstat" <<'EOF'
#!/bin/bash
# mpstat dispatcher for CPU tests
SCENARIO="${CPU_SCENARIO:-normal}"

# Look for scenario-specific mock in cpu directory
if [ -f "/mock-commands/cpu/mpstat-$SCENARIO" ]; then
    exec "/mock-commands/cpu/mpstat-$SCENARIO"
fi

# Default to normal scenario
exec "/mock-commands/cpu/mpstat-normal"
EOF

    # Create iotop dispatcher for cpu directory
    cat > "$cmd_dir/iotop" <<'EOF'
#!/bin/bash
# iotop dispatcher for CPU tests
SCENARIO="${IOTOP_SCENARIO:-test-standard}"

# Look for scenario-specific mock in cpu directory
if [ -f "/mock-commands/cpu/iotop-$SCENARIO" ]; then
    exec "/mock-commands/cpu/iotop-$SCENARIO"
fi

# Default to standard scenario
exec "/mock-commands/cpu/iotop-test-standard"
EOF

    # Make all mock commands executable
    chmod +x "$cmd_dir"/*
}

# Run all setup
create_high_pressure
create_normal
create_additional_mock_data
create_isolated_test_scenarios
create_mock_commands

echo "Mock data creation complete"