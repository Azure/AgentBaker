#!/bin/bash
# Create comprehensive memory pressure mock data for testing
# This script creates various memory pressure scenarios including PSI metrics, meminfo, and OOM events

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# High Memory Pressure Scenario
create_high_memory_pressure() {
    echo "Create high memory pressure mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-high-pressure"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    
    # /proc/meminfo - showing low available memory (high pressure)
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        4096000 kB
MemFree:          128000 kB
MemAvailable:     256000 kB
Buffers:           32000 kB
Cached:            96000 kB
EOF

    # /proc/uptime for OOM timestamp calculations
    echo "3600.00 7200.00" > "$dir/proc/uptime"

    # PSI memory metrics showing high pressure
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=45.00 avg60=50.00 avg300=55.00 total=123456789
full avg10=15.00 avg60=20.00 avg300=25.00 total=23456789
EOF
}

# Normal Memory Scenario
create_normal_memory() {
    echo "Create normal memory mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-normal"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    
    # /proc/meminfo - showing adequate available memory (normal conditions)
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        8192000 kB
MemFree:         4096000 kB
MemAvailable:    6144000 kB
Buffers:         1024000 kB
Cached:          1024000 kB
EOF

    # Same uptime as high pressure
    cp "$SCRIPT_DIR/mock-data/memory-high-pressure/proc/uptime" "$dir/proc/uptime"

    # PSI memory metrics showing low pressure
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=2.00 avg60=3.00 avg300=4.00 total=123456789
full avg10=0.50 avg60=1.00 avg300=1.50 total=23456789
EOF
}

# Create mock dmesg commands for different OOM scenarios
create_mock_dmesg_commands() {
    echo "Create mock dmesg commands"
    local cmd_dir="$SCRIPT_DIR/mock-commands/memory"
    mkdir -p "$cmd_dir"
    
    # Remove any existing dmesg mock files that might conflict
    # Safe removal; ensure cmd_dir is set and not empty to avoid /* expansion
    rm -rf "${cmd_dir:?}"/*
    
    # Legacy dmesg mocks (for backward compatibility with existing tests)
    
    # Mock dmesg with recent OOM events
    cat > "$cmd_dir/dmesg-with-oom" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) 
[    1.234567] Memory: 8192000K/8388608K available
[   10.123456] systemd[1]: Started Kernel log.
[ 3500.567890] Out of memory: Kill process 1234 (chrome) score 850 or sacrifice child
[ 3500.567891] Killed process 1234 (chrome) total-vm:2345678kB, anon-rss:1234567kB, file-rss:0kB, shmem-rss:0kB
[ 3520.789012] oom-kill:constraint=CONSTRAINT_NONE,nodemask=(null),cpuset=/,mems_allowed=0,global_oom,task_memcg=/user.slice/user-1000.slice/session-2.scope,task=node,pid=5678,uid=1000
[ 3520.789013] Out of memory: Kill process 5678 (node) score 500 or sacrifice child
[ 3520.789014] Killed process 5678 (node) total-vm:1234567kB, anon-rss:987654kB, file-rss:123456kB, shmem-rss:0kB
[ 3590.123456] systemd[1]: Started some-service.service.
DMESG_EOF
EOF
    
    # Mock dmesg with old OOM events (outside 5-minute window)
    cat > "$cmd_dir/dmesg-old-oom" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) 
[    1.234567] Memory: 8192000K/8388608K available
[   10.123456] systemd[1]: Started Kernel log.
[ 3200.567890] Out of memory: Kill process 1234 (chrome) score 850 or sacrifice child
[ 3200.567891] Killed process 1234 (chrome) total-vm:2345678kB, anon-rss:1234567kB, file-rss:0kB, shmem-rss:0kB
[ 3220.789012] oom-kill:constraint=CONSTRAINT_NONE,nodemask=(null),cpuset=/,mems_allowed=0,global_oom,task_memcg=/user.slice/user-1000.slice/session-2.scope,task=node,pid=5678,uid=1000
[ 3590.123456] systemd[1]: Started some-service.service.
DMESG_EOF
EOF
    
    # Mock dmesg with no OOM events
    cat > "$cmd_dir/dmesg-no-oom" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) 
[    1.234567] Memory: 8192000K/8388608K available
[   10.123456] systemd[1]: Started Kernel log.
[ 3590.123456] systemd[1]: Started some-service.service.
[ 3600.789012] systemd[1]: Reached target multi-user.target.
DMESG_EOF
EOF
    
    # Mock dmesg with malformed timestamps
    cat > "$cmd_dir/dmesg-malformed" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[INVALID] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) 
Memory: 8192000K/8388608K available
Out of memory: Kill process 1234 (chrome) score 850 or sacrifice child
[ 3590.abc] systemd[1]: Started some-service.service.
DMESG_EOF
EOF
    
    # Mock dmesg permission denied
    cat > "$cmd_dir/dmesg-permission-denied" <<'EOF'
#!/bin/bash
echo "dmesg: read kernel buffer failed: Operation not permitted" >&2
exit 1
EOF
    
    # Mock dmesg with recent OOM events (within 5-minute window)
    cat > "$cmd_dir/dmesg-memory-oom-recent" <<'EOF'
#!/bin/bash
# Generate OOM events with recent timestamps (within last 5 minutes)
# Get current uptime and calculate recent timestamps
UPTIME_SECONDS=$(cat /proc/uptime | awk '{print $1}' | cut -d. -f1)
RECENT_TIME1=$((UPTIME_SECONDS - 120))  # 2 minutes ago
RECENT_TIME2=$((UPTIME_SECONDS - 60))   # 1 minute ago

cat <<DMESG_EOF
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.1) ) #69~20.04.1-Ubuntu SMP Wed Jan 12 22:28:45 UTC 2022
[    1.234567] Memory: 4096000K/4194304K available (12288K kernel code, 2048K rwdata, 4096K rodata, 1536K init, 1024K bss, 98304K reserved, 0K cma-reserved)
[   10.123456] systemd[1]: Started Kernel Logging Service.
[$RECENT_TIME1.567890] chrome invoked oom-killer: gfp_mask=0x6200ca(GFP_HIGHUSER_MOVABLE), order=0, oom_score_adj=300
[$RECENT_TIME1.567891] CPU: 0 PID: 1234 Comm: chrome Not tainted 5.4.0-1064-azure #69~20.04.1-Ubuntu
[$RECENT_TIME1.567892] Out of memory: Kill process 1234 (chrome) score 850 or sacrifice child
[$RECENT_TIME1.567893] Killed process 1234 (chrome) total-vm:2345678kB, anon-rss:1234567kB, file-rss:0kB, shmem-rss:0kB
[$RECENT_TIME2.789012] node invoked oom-killer: gfp_mask=0x6200ca(GFP_HIGHUSER_MOVABLE), order=0, oom_score_adj=0
[$RECENT_TIME2.789013] oom-kill:constraint=CONSTRAINT_NONE,nodemask=(null),cpuset=/,mems_allowed=0,global_oom,task_memcg=/user.slice/user-1000.slice/session-2.scope,task=node,pid=5678,uid=1000
[$RECENT_TIME2.789014] Out of memory: Kill process 5678 (node) score 500 or sacrifice child
[$RECENT_TIME2.789015] Killed process 5678 (node) total-vm:1234567kB, anon-rss:987654kB, file-rss:123456kB, shmem-rss:0kB
[$RECENT_TIME2.234567] systemd[1]: session-2.scope: A process of this unit has been killed by the OOM killer.
[$UPTIME_SECONDS.123456] systemd[1]: Started some-service.service.
DMESG_EOF
EOF
    
    # Mock dmesg with old OOM events (outside 5-minute window)
    cat > "$cmd_dir/dmesg-memory-oom-old" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.1) ) #69~20.04.1-Ubuntu SMP Wed Jan 12 22:28:45 UTC 2022
[    1.234567] Memory: 4096000K/4194304K available (12288K kernel code, 2048K rwdata, 4096K rodata, 1536K init, 1024K bss, 98304K reserved, 0K cma-reserved)
[   10.123456] systemd[1]: Started Kernel Logging Service.
[ 3200.567890] chrome invoked oom-killer: gfp_mask=0x6200ca(GFP_HIGHUSER_MOVABLE), order=0, oom_score_adj=300
[ 3200.567891] Out of memory: Kill process 1234 (chrome) score 850 or sacrifice child
[ 3200.567892] Killed process 1234 (chrome) total-vm:2345678kB, anon-rss:1234567kB, file-rss:0kB, shmem-rss:0kB
[ 3220.789012] oom-kill:constraint=CONSTRAINT_NONE,nodemask=(null),cpuset=/,mems_allowed=0,global_oom,task_memcg=/user.slice/user-1000.slice/session-2.scope,task=node,pid=5678,uid=1000
[ 3220.789013] Out of memory: Kill process 5678 (node) score 500 or sacrifice child
[ 3590.123456] systemd[1]: Started some-service.service.
[ 3595.789012] systemd[1]: Reached target multi-user.target.
DMESG_EOF
EOF
    
    # Mock dmesg with no OOM events
    cat > "$cmd_dir/dmesg-memory-no-oom" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.1) ) #69~20.04.1-Ubuntu SMP Wed Jan 12 22:28:45 UTC 2022
[    1.234567] Memory: 4096000K/4194304K available (12288K kernel code, 2048K rwdata, 4096K rodata, 1536K init, 1024K bss, 98304K reserved, 0K cma-reserved)
[   10.123456] systemd[1]: Started Kernel Logging Service.
[ 3590.123456] systemd[1]: Started some-service.service.
[ 3595.789012] systemd[1]: Reached target multi-user.target.
[ 3600.345678] systemd[1]: Startup finished in 45.123s (kernel) + 12.456s (initrd) + 23.789s (userspace) = 81.368s.
DMESG_EOF
EOF
    
    # Mock dmesg with complex OOM events and special characters
    cat > "$cmd_dir/dmesg-memory-oom-complex" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) (gcc version 9.4.0 (Ubuntu 9.4.0-1ubuntu1~20.04.1) ) #69~20.04.1-Ubuntu SMP Wed Jan 12 22:28:45 UTC 2022
[    1.234567] Memory: 4096000K/4194304K available
[ 3510.567890] "app with spaces" invoked oom-killer: gfp_mask=0x6200ca(GFP_HIGHUSER_MOVABLE), order=0, oom_score_adj=0
[ 3510.567891] Out of memory: Kill process 2222 ("app with spaces") score 600 or sacrifice child
[ 3510.567892] Killed process 2222 ("app with spaces") total-vm:1111111kB, anon-rss:888888kB, file-rss:222222kB, shmem-rss:0kB
[ 3515.789012] app\with\backslashes invoked oom-killer: gfp_mask=0x6200ca(GFP_HIGHUSER_MOVABLE), order=0, oom_score_adj=0
[ 3515.789013] Out of memory: Kill process 3333 (app\with\backslashes) score 400 or sacrifice child
[ 3515.789014] Killed process 3333 (app\with\backslashes) total-vm:999999kB, anon-rss:666666kB, file-rss:111111kB, shmem-rss:0kB
[ 3525.234567] app	with	tabs invoked oom-killer: gfp_mask=0x6200ca(GFP_HIGHUSER_MOVABLE), order=0, oom_score_adj=0
[ 3525.234568] Out of memory: Kill process 4444 (app	with	tabs) score 300 or sacrifice child
[ 3590.123456] systemd[1]: Started some-service.service.
DMESG_EOF
EOF
    
    # Mock dmesg with malformed timestamps
    cat > "$cmd_dir/dmesg-memory-malformed" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[INVALID.TIMESTAMP] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) 
Memory: 4096000K/4194304K available
[abc.def] Out of memory: Kill process 1234 (test-app) score 500 or sacrifice child
[ 3590.xyz] systemd[1]: Started some-service.service.
[    ] Out of memory: Kill process 5678 (another-app) score 400 or sacrifice child
Out of memory: Kill process 9999 (no-timestamp-app) score 200 or sacrifice child
[ 3595.789012] systemd[1]: Reached target multi-user.target.
DMESG_EOF
EOF
    
    # Mock dmesg permission denied
    cat > "$cmd_dir/dmesg-memory-permission-denied" <<'EOF'
#!/bin/bash
echo "dmesg: read kernel buffer failed: Operation not permitted" >&2
exit 1
EOF
    
    # Mock dmesg command unavailable
    cat > "$cmd_dir/dmesg-memory-unavailable" <<'EOF'
#!/bin/bash
echo "bash: dmesg: command not found" >&2
exit 127
EOF
    
    # Mock dmesg with memory information but no OOM
    cat > "$cmd_dir/dmesg-memory-info-only" <<'EOF'
#!/bin/bash
cat <<'DMESG_EOF'
[    0.000000] Linux version 5.4.0-1064-azure (buildd@lcy02-amd64-112) 
[    1.234567] Memory: 4096000K/4194304K available (12288K kernel code, 2048K rwdata, 4096K rodata, 1536K init, 1024K bss, 98304K reserved, 0K cma-reserved)
[   10.123456] systemd[1]: Started Kernel Logging Service.
[ 3500.567890] Memory cgroup stats for /system.slice: cache:1024000KB rss:2048000KB rss_huge:0KB shmem:0KB mapped_file:512000KB dirty:64000KB writeback:0KB
[ 3510.789012] Memory usage statistics: total_cache:2048000KB total_rss:4096000KB total_rss_huge:0KB total_shmem:128000KB total_mapped_file:1024000KB
[ 3520.234567] Memory reclaim statistics: pages_scanned:123456 pages_reclaimed:12345 pages_evicted:2345
[ 3590.123456] systemd[1]: Started some-service.service.
DMESG_EOF
EOF
    
    # Create dmesg dispatcher for memory directory
    cat > "$cmd_dir/dmesg" <<'EOF'
#!/bin/bash
# dmesg dispatcher for memory tests
SCENARIO="${MEMORY_DMESG_SCENARIO:-no-oom}"

# Look for scenario-specific mock in memory directory
if [ -f "/mock-commands/memory/dmesg-$SCENARIO" ]; then
    exec "/mock-commands/memory/dmesg-$SCENARIO"
fi

# Default to no-oom scenario
exec "/mock-commands/memory/dmesg-no-oom"
EOF

    # Make all dmesg mock files executable
    chmod +x "$cmd_dir"/*
}

# High Memory PSI Pressure Scenario (PSI-only test)
create_high_memory_psi() {
    echo "Create high memory PSI pressure mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-high-psi"
    
    mkdir -p "$dir/sys/fs/cgroup"
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=25.00 avg60=30.00 avg300=35.00 total=12345678
full avg10=5.00 avg60=8.00 avg300=10.00 total=987654
EOF
    
    # Create normal meminfo for PSI-only test
    mkdir -p "$dir/proc"
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        8192000 kB
MemFree:         2048000 kB
MemAvailable:    3072000 kB
Buffers:          512000 kB
Cached:           512000 kB
EOF
}

# Low Available Memory Scenario
create_low_available_memory() {
    echo "Create low available memory mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-low-available"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        8192000 kB
MemFree:          256000 kB
MemAvailable:     409600 kB
Buffers:           64000 kB
Cached:           192000 kB
EOF
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=5.00 avg60=8.00 avg300=10.00 total=12345678
full avg10=2.00 avg60=3.00 avg300=4.00 total=987654
EOF
}

# OOM Events Scenario
create_oom_events_memory() {
    echo "Create OOM events memory mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-oom-events"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        4096000 kB
MemFree:          128000 kB
MemAvailable:     256000 kB
Buffers:          256000 kB
Cached:           256000 kB
EOF
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=8.00 avg60=12.00 avg300=15.00 total=12345678
full avg10=3.00 avg60=5.00 avg300=7.00 total=987654
EOF
    # Create uptime file for OOM timestamp calculations
    echo "3600.00 7200.00" > "$dir/proc/uptime"
}

# Combined High Pressure Scenario (PSI + low available + OOM)
create_combined_memory_pressure() {
    echo "Create combined memory pressure mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-combined-pressure"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        4096000 kB
MemFree:          128000 kB
MemAvailable:     256000 kB
Buffers:           32000 kB
Cached:            96000 kB
EOF
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=45.00 avg60=50.00 avg300=55.00 total=12345678
full avg10=15.00 avg60=20.00 avg300=25.00 total=987654
EOF
    echo "3600.00 7200.00" > "$dir/proc/uptime"
}

# No Pressure Baseline Scenario
create_no_memory_pressure() {
    echo "Create no memory pressure mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-no-pressure"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        8192000 kB
MemFree:         4096000 kB
MemAvailable:    6144000 kB
Buffers:         1024000 kB
Cached:          1024000 kB
EOF
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=2.00 avg60=3.00 avg300=4.00 total=12345678
full avg10=0.50 avg60=1.00 avg300=1.50 total=987654
EOF
}

# Legacy Kernel Scenario (no MemAvailable field)
create_legacy_kernel_memory() {
    echo "Create legacy kernel memory mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-legacy-kernel"
    
    mkdir -p "$dir/proc" "$dir/sys/fs/cgroup"
    cat > "$dir/proc/meminfo" <<'EOF'
MemTotal:        4096000 kB
MemFree:          512000 kB
Buffers:          256000 kB
Cached:           768000 kB
EOF
    cat > "$dir/sys/fs/cgroup/memory.pressure" <<'EOF'
some avg10=5.00 avg60=8.00 avg300=10.00 total=12345678
full avg10=2.00 avg60=3.00 avg300=4.00 total=987654
EOF
}

# Missing PSI File Scenario
create_missing_psi_memory() {
    echo "Create missing PSI file mock data"
    local dir="$SCRIPT_DIR/mock-data/memory-missing-psi"
    
    # Create a sys directory without memory.pressure for testing missing file scenario
    mkdir -p "$dir/sys/fs/cgroup"
    # Deliberately not creating memory.pressure file here
}


# Run all setup
create_high_memory_pressure
create_normal_memory
create_high_memory_psi
create_low_available_memory
create_oom_events_memory
create_combined_memory_pressure
create_no_memory_pressure
create_legacy_kernel_memory
create_missing_psi_memory
create_mock_dmesg_commands

echo "Memory mock data creation complete!"