#!/bin/bash
# Script to create mock command data for pressure_common.sh function testing
# This script generates realistic output for top, systemd-cgtop, and crictl commands

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MOCK_COMMANDS_DIR="$SCRIPT_DIR/mock-commands"
CPU_COMMANDS_DIR="$MOCK_COMMANDS_DIR/cpu"

# Ensure mock commands directory exists
mkdir -p "$MOCK_COMMANDS_DIR" "$CPU_COMMANDS_DIR"

# Remove any existing test files that might conflict (but preserve other mock files)
rm -rf "$MOCK_COMMANDS_DIR"/top-test-* "$MOCK_COMMANDS_DIR"/systemd-cgtop-test-* "$MOCK_COMMANDS_DIR"/crictl-test-* "$MOCK_COMMANDS_DIR"/ig-test-* "$MOCK_COMMANDS_DIR"/integration-*
rm -rf "$CPU_COMMANDS_DIR"/top-test-* "$CPU_COMMANDS_DIR"/systemd-cgtop-test-* "$CPU_COMMANDS_DIR"/crictl-test-* "$CPU_COMMANDS_DIR"/ig-test-*

echo "Create mock command scripts for pressure_common tests"

# =============================================================================
# TOP COMMAND MOCK SCRIPTS
# =============================================================================

# Top normal CPU output
cat > "$CPU_COMMANDS_DIR/top-test-cpu-normal" <<'EOF'
#!/bin/bash
cat <<'TOP_EOF'
top - 14:32:45 up 5 days,  3:21,  2 users,  load average: 2.15, 1.98, 1.75
Tasks: 287 total,   3 running, 284 sleeping,   0 stopped,   0 zombie
%Cpu(s): 25.3 us,  8.2 sy,  0.0 ni, 64.1 id,  2.1 wa,  0.0 hi,  0.3 si,  0.0 st
MiB Mem :   7856.4 total,   1234.5 free,   4321.2 used,   2300.7 buff/cache
MiB Swap:   2048.0 total,   2048.0 free,      0.0 used.   2876.8 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root     20   0  123456  45678  12345 R  45.2   0.6   1:23.45 high-cpu-process
   2345 user1    20   0   98765  23456   7890 S  23.1   0.3   0:45.67 medium-cpu-app
   3456 user2    20   0   67890  12345   4567 S  12.5   0.2   0:23.45 some-service
   4567 root     20   0   45678   8901   2345 S   8.3   0.1   0:12.34 background-task
   5678 user3    20   0   34567   6789   1234 S   5.2   0.1   0:08.90 idle-process
   6789 user1    20   0   23456   4567    890 S   3.1   0.1   0:05.67 small-app
   7890 root     20   0   12345   2345    567 S   2.1   0.0   0:03.45 system-daemon
   8901 user2    20   0    9876   1234    234 S   1.5   0.0   0:01.23 monitoring
   9012 user3    20   0    8765    987    123 S   0.8   0.0   0:00.45 log-reader
  10123 root     20   0    7654    876     98 S   0.5   0.0   0:00.23 cleanup-task
TOP_EOF
EOF

# Top normal memory output
cat > "$CPU_COMMANDS_DIR/top-test-memory-normal" <<'EOF'
#!/bin/bash
cat <<'TOP_EOF'
top - 14:32:45 up 5 days,  3:21,  2 users,  load average: 1.15, 1.05, 0.95
Tasks: 287 total,   1 running, 286 sleeping,   0 stopped,   0 zombie
%Cpu(s):  5.3 us,  2.2 sy,  0.0 ni, 91.1 id,  1.1 wa,  0.0 hi,  0.3 si,  0.0 st
MiB Mem :   7856.4 total,   1234.5 free,   4321.2 used,   2300.7 buff/cache
MiB Swap:   2048.0 total,   2048.0 free,      0.0 used.   2876.8 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 user1    20   0 2345678 654321  98765 S   5.2  32.1   2:34.56 memory-hog-app
   2345 root     20   0 1234567 432109  76543 S   3.1  25.3   1:23.45 database-server
   3456 user2    20   0  987654 321098  54321 S   2.1  18.7   0:56.78 web-application
   4567 user3    20   0  765432 210987  43210 S   1.5  12.4   0:34.56 cache-service
   5678 root     20   0  543210 109876  32109 S   1.2   8.9   0:23.45 container-runtime
   6789 user1    20   0  432109  87654  21098 S   0.8   6.3   0:12.34 log-aggregator
   7890 user2    20   0  321098  65432  10987 S   0.5   4.7   0:08.90 monitoring-agent
   8901 user3    20   0  210987  43210   8765 S   0.3   3.2   0:05.67 backup-process
   9012 root     20   0  109876  21098   4321 S   0.2   2.1   0:03.45 system-service
  10123 user1    20   0   98765  10987   2109 S   0.1   1.5   0:01.23 small-utility
TOP_EOF
EOF

# Top with many CPU cores (for line limiting test)
cat > "$CPU_COMMANDS_DIR/top-test-many-cores" <<'EOF'
#!/bin/bash
cat <<'TOP_EOF'
top - 14:32:45 up 5 days,  3:21,  2 users,  load average: 8.15, 7.98, 7.75
Tasks: 1287 total,  8 running, 1279 sleeping,   0 stopped,   0 zombie
%Cpu0  : 95.0/5.0   100[||||||||||||||||||||||||||||||||||||||||||||||||]
%Cpu1  : 92.3/7.7   100[|||||||||||||||||||||||||||||||||||||||||||||||  ]
%Cpu2  : 89.5/10.5  100[||||||||||||||||||||||||||||||||||||||||||||||   ]
%Cpu3  : 87.2/12.8  100[|||||||||||||||||||||||||||||||||||||||||||||    ]
%Cpu4  : 85.1/14.9  100[||||||||||||||||||||||||||||||||||||||||||||     ]
%Cpu5  : 83.3/16.7  100[|||||||||||||||||||||||||||||||||||||||||||      ]
%Cpu6  : 81.5/18.5  100[||||||||||||||||||||||||||||||||||||||||||       ]
%Cpu7  : 79.8/20.2  100[|||||||||||||||||||||||||||||||||||||||||        ]
%Cpu8  : 78.1/21.9  100[||||||||||||||||||||||||||||||||||||||||         ]
%Cpu9  : 76.4/23.6  100[|||||||||||||||||||||||||||||||||||||||          ]
%Cpu10 : 74.7/25.3  100[||||||||||||||||||||||||||||||||||||||           ]
%Cpu11 : 73.0/27.0  100[|||||||||||||||||||||||||||||||||||||            ]
%Cpu12 : 71.3/28.7  100[||||||||||||||||||||||||||||||||||||             ]
%Cpu13 : 69.6/30.4  100[|||||||||||||||||||||||||||||||||||              ]
%Cpu14 : 67.9/32.1  100[||||||||||||||||||||||||||||||||||               ]
%Cpu15 : 66.2/33.8  100[|||||||||||||||||||||||||||||||||                ]
MiB Mem :  32768.0 total,   4096.0 free,  20480.0 used,   8192.0 buff/cache
MiB Swap:   4096.0 total,   4096.0 free,      0.0 used.  10240.0 avail Mem 

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root     20   0 4567890 1234567 234567 R  85.2   3.8  12:34.56 cpu-intensive-app
   2345 user1    20   0 3456789  987654 123456 R  72.1   3.0   8:45.67 parallel-compute
   3456 user2    20   0 2345678  765432  98765 R  65.3   2.3   6:23.45 data-processing
   4567 user3    20   0 1234567  543210  76543 S  58.7   1.7   4:56.78 simulation-job
   5678 root     20   0  987654  432109  65432 S  45.2   1.3   3:12.34 compression-task
TOP_EOF
EOF

# Top timeout (simulate slow/hanging command)
cat > "$CPU_COMMANDS_DIR/top-test-timeout" <<'EOF'
#!/bin/bash
# Simulate a command that takes too long
sleep 15
echo "This should not appear due to timeout"
EOF

# Top permission denied
cat > "$CPU_COMMANDS_DIR/top-test-permission" <<'EOF'
#!/bin/bash
echo "top: Permission denied" >&2
exit 1
EOF

# =============================================================================
# SYSTEMD-CGTOP COMMAND MOCK SCRIPTS
# =============================================================================

# Systemd-cgtop normal CPU output
cat > "$CPU_COMMANDS_DIR/systemd-cgtop-test-cpu" <<'EOF'
#!/bin/bash
cat <<'CGTOP_EOF'
Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1234    5.2     4.5G        -        -
/system.slice                                              456    2.1     1.2G        -        -
/system.slice/docker.service                               123   15.3   512.5M        -        -
/system.slice/kubelet.service                               89   12.7   256.7M        -        -
/system.slice/containerd.service                            67    8.4   128.3M        -        -
/user.slice                                                234    1.8   890.2M        -        -

Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1234   25.7     4.6G        -        -
/system.slice                                              456   12.4     1.3G        -        -
/system.slice/docker.service                               123   45.8   523.1M        -        -
/system.slice/kubelet.service                               89   38.2   267.4M        -        -
/system.slice/containerd.service                            67   28.9   134.7M        -        -
/user.slice                                                234    8.3   901.5M        -        -
CGTOP_EOF
EOF

# Systemd-cgtop normal memory output
cat > "$CPU_COMMANDS_DIR/systemd-cgtop-test-memory" <<'EOF'
#!/bin/bash
cat <<'CGTOP_EOF'
Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1234    5.2     4.5G        -        -
/system.slice                                              456    2.1     1.2G        -        -
/system.slice/docker.service                               123    3.3     2.1G        -        -
/system.slice/kubelet.service                               89    2.7     1.8G        -        -
/system.slice/containerd.service                            67    1.4     1.2G        -        -
/user.slice                                                234    1.8   890.2M        -        -

Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1234    8.7     4.8G        -        -
/system.slice                                              456    4.2     1.4G        -        -
/system.slice/docker.service                               123    5.8     2.3G        -        -
/system.slice/kubelet.service                               89    4.1     1.9G        -        -
/system.slice/containerd.service                            67    2.9     1.3G        -        -
/user.slice                                                234    3.2   945.7M        -        -
CGTOP_EOF
EOF

# Systemd-cgtop with clear second iteration (for extraction test)
cat > "$CPU_COMMANDS_DIR/systemd-cgtop-test-second-iteration" <<'EOF'
#!/bin/bash
cat <<'CGTOP_EOF'
Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1000    0.0     3.0G        -        -
/system.slice                                              300    0.0     800M        -        -
/system.slice/docker.service                               100    0.0     200M        -        -

Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1000   50.5     3.2G        -        -
/system.slice                                              300   35.2     850M        -        -
/system.slice/docker.service                               100   75.8     250M        -        -
/system.slice/kubelet.service                               50   45.3     180M        -        -
/user.slice                                                200   15.7     600M        -        -
CGTOP_EOF
EOF

# Systemd-cgtop error
cat > "$CPU_COMMANDS_DIR/systemd-cgtop-test-error" <<'EOF'
#!/bin/bash
echo "systemd-cgtop: Error: Unable to access cgroup hierarchy" >&2
exit 1
EOF

# =============================================================================
# CRICTL COMMAND MOCK SCRIPTS
# =============================================================================

# Crictl normal CPU stats
cat > "$CPU_COMMANDS_DIR/crictl-test-cpu" <<'EOF'
#!/bin/bash
cat <<'CRICTL_EOF'
{
  "stats": [
    {
      "attributes": {
        "id": "container1",
        "metadata": {
          "name": "nginx-container"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "default",
          "io.kubernetes.pod.name": "nginx-pod-12345"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "0"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "750000000"
        },
        "throttlingData": {
          "periods": 1000,
          "throttledPeriods": 50
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "134217728"
        }
      }
    },
    {
      "attributes": {
        "id": "container2",
        "metadata": {
          "name": "app-container"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "kube-system",
          "io.kubernetes.pod.name": "app-pod-67890"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "2"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "500000000"
        },
        "throttlingData": {
          "periods": 800,
          "throttledPeriods": 20
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "67108864"
        }
      }
    },
    {
      "attributes": {
        "id": "container3",
        "metadata": {
          "name": "db-container"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "database",
          "io.kubernetes.pod.name": "mysql-pod-abcde"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "1"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "250000000"
        },
        "throttlingData": {
          "periods": 600,
          "throttledPeriods": 10
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "268435456"
        }
      }
    }
  ]
}
CRICTL_EOF
EOF

# Crictl normal memory stats (same structure, different sorting expectation)
cat > "$CPU_COMMANDS_DIR/crictl-test-memory" <<'EOF'
#!/bin/bash
cat <<'CRICTL_EOF'
{
  "stats": [
    {
      "attributes": {
        "id": "container1",
        "metadata": {
          "name": "memory-hog"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "default",
          "io.kubernetes.pod.name": "memory-intensive-pod"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "0"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "100000000"
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "1073741824"
        }
      }
    },
    {
      "attributes": {
        "id": "container2",
        "metadata": {
          "name": "normal-app"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "apps",
          "io.kubernetes.pod.name": "normal-app-pod"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "0"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "200000000"
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "134217728"
        }
      }
    }
  ]
}
CRICTL_EOF
EOF

# Crictl with high throttling data
cat > "$CPU_COMMANDS_DIR/crictl-test-throttling" <<'EOF'
#!/bin/bash
cat <<'CRICTL_EOF'
{
  "stats": [
    {
      "attributes": {
        "id": "throttled-container",
        "metadata": {
          "name": "cpu-limited-app"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "production",
          "io.kubernetes.pod.name": "throttled-pod-xyz"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "5"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "1000000000"
        },
        "throttlingData": {
          "periods": 10000,
          "throttledPeriods": 8500
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "536870912"
        }
      }
    }
  ]
}
CRICTL_EOF
EOF

# Crictl with many containers (for limiting test)
# Remove existing file first to ensure clean generation
rm -rf "$CPU_COMMANDS_DIR/crictl-test-many-containers"

cat > "$CPU_COMMANDS_DIR/crictl-test-many-containers" <<'EOF'
#!/bin/bash
cat <<'CRICTL_EOF'
{
  "stats": [
EOF

# Generate 20 containers to test the limiting functionality
for i in $(seq 1 20); do
    cpu_usage=$((1000000000 - i * 40000000))
    memory_usage=$((134217728 + i * 10485760))
    
    cat >> "$CPU_COMMANDS_DIR/crictl-test-many-containers" <<EOF
    {
      "attributes": {
        "id": "container$i",
        "metadata": {
          "name": "app-container-$i"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "test",
          "io.kubernetes.pod.name": "test-pod-$i"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "0"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "$cpu_usage"
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "$memory_usage"
        }
      }
    }$([ $i -lt 20 ] && echo ",")
EOF
done

cat >> "$CPU_COMMANDS_DIR/crictl-test-many-containers" <<'EOF'
  ]
}
CRICTL_EOF
EOF

# Crictl error
cat > "$CPU_COMMANDS_DIR/crictl-test-error" <<'EOF'
#!/bin/bash
echo "ERRO[0000] validate service connection: CRI v1 runtime API is not implemented for endpoint" >&2
exit 1
EOF

# Crictl empty output
cat > "$CPU_COMMANDS_DIR/crictl-test-empty" <<'EOF'
#!/bin/bash
cat <<'CRICTL_EOF'
{
  "stats": []
}
CRICTL_EOF
EOF

# =============================================================================
# INSPEKTOR GADGET COMMAND MOCK SCRIPTS
# =============================================================================

# IG normal top_process output
cat > "$CPU_COMMANDS_DIR/ig-test-normal" <<'EOF'
#!/bin/bash
cat <<'IG_EOF'
[
  {
    "comm": "test-process-1",
    "pid": 1
  }
]
[
  {
    "comm": "test-process-2",
    "pid": 2
  }
]
IG_EOF
EOF

# IG top_process output with only one array (for error handling test)
cat > "$CPU_COMMANDS_DIR/ig-test-single-array" <<'EOF'
#!/bin/bash
cat <<'IG_EOF'
[
  {
    "comm": "test-process-1",
    "pid": 1
  }
]
IG_EOF
EOF

# IG command timeout (sleeps longer than 10s timeout)
cat > "$CPU_COMMANDS_DIR/ig-test-timeout" <<'EOF'
#!/bin/bash
sleep 12
echo "This should not be reached due to timeout"
EOF

# IG command with empty output
cat > "$CPU_COMMANDS_DIR/ig-test-empty" <<'EOF'
#!/bin/bash
# Simulate IG returning nothing
EOF

# IG top_process output with error message included
cat > "$CPU_COMMANDS_DIR/ig-test-with-error" <<'EOF'
#!/bin/bash
cat <<'IG_EOF'
ERRO[0000] Example of an error message from IG
[
  {
    "comm": "test-process-1",
    "pid": 1
  }
]
[
  {
    "comm": "test-process-2",
    "pid": 2
  }
]
IG_EOF
EOF

# =============================================================================
# INTEGRATION TEST MOCK SCRIPTS
# =============================================================================

# Integration mock that supports all commands
cat > "$MOCK_COMMANDS_DIR/integration-all-commands" <<'EOF'
#!/bin/bash
# This script acts as top, systemd-cgtop, and crictl depending on its name
COMMAND_NAME=$(basename "$0")

case "$COMMAND_NAME" in
    "top")
        cat <<'TOP_EOF'
top - 14:32:45 up 5 days,  3:21,  2 users,  load average: 2.15, 1.98, 1.75
Tasks: 287 total,   3 running, 284 sleeping,   0 stopped,   0 zombie
%Cpu(s): 25.3 us,  8.2 sy,  0.0 ni, 64.1 id,  2.1 wa,  0.0 hi,  0.3 si,  0.0 st
MiB Mem :   7856.4 total,   1234.5 free,   4321.2 used,   2300.7 buff/cache

    PID USER      PR  NI    VIRT    RES    SHR S  %CPU  %MEM     TIME+ COMMAND
   1234 root     20   0  123456  45678  12345 R  45.2   0.6   1:23.45 integration-test-process
TOP_EOF
        ;;
    "systemd-cgtop")
        cat <<'CGTOP_EOF'
Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1000   25.5     3.2G        -        -
/system.slice                                              300   15.2     850M        -        -
CGTOP_EOF
        ;;
    "crictl")
        cat <<'CRICTL_EOF'
{
  "stats": [
    {
      "attributes": {
        "id": "integration-container",
        "metadata": {
          "name": "integration-test"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "integration",
          "io.kubernetes.pod.name": "integration-pod"
        }
      },
      "cpu": {
        "usageNanoCores": {
          "value": "500000000"
        }
      },
      "memory": {
        "workingSetBytes": {
          "value": "134217728"
        }
      }
    }
  ]
}
CRICTL_EOF
        ;;
esac
EOF

# Crictl chunking test (large output)
# Remove existing file first to ensure clean generation
rm -rf "$CPU_COMMANDS_DIR/crictl-test-chunking"

cat > "$CPU_COMMANDS_DIR/crictl-test-chunking" <<'EOF'
#!/bin/bash
cat <<'CRICTL_EOF'
{
  "stats": [
EOF

# Generate 50 containers to test chunking
for i in $(seq 1 50); do
    cat >> "$CPU_COMMANDS_DIR/crictl-test-chunking" <<EOF
    {
      "attributes": {
        "id": "chunking-container-$i",
        "metadata": {
          "name": "very-long-container-name-for-chunking-test-container-number-$i-with-extra-data"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "chunking-test-namespace-with-long-name",
          "io.kubernetes.pod.name": "chunking-test-pod-with-very-long-name-$i"
        },
        "annotations": {
          "io.kubernetes.container.restartCount": "$((i % 10))",
          "very.long.annotation.key.for.testing.purposes": "very-long-annotation-value-to-increase-json-size-$i"
        }
      },
      "cpu": {
        "timestamp": "2024-01-01T10:00:00Z",
        "usageNanoCores": {
          "value": "$((1000000000 - i * 15000000))"
        },
        "throttlingData": {
          "periods": $((1000 + i * 100)),
          "throttledPeriods": $((i * 5))
        }
      },
      "memory": {
        "timestamp": "2024-01-01T10:00:00Z",
        "workingSetBytes": {
          "value": "$((134217728 + i * 20971520))"
        }
      }
    }$([ $i -lt 50 ] && echo ",")
EOF
done

cat >> "$CPU_COMMANDS_DIR/crictl-test-chunking" <<'EOF'
  ]
}
CRICTL_EOF
EOF

# Integration resource calculation test
cat > "$MOCK_COMMANDS_DIR/integration-resource-calc" <<'EOF'
#!/bin/bash
# This script supports both systemd-cgtop and crictl for resource calculation testing
COMMAND_NAME=$(basename "$0")

case "$COMMAND_NAME" in
    "systemd-cgtop")
        cat <<'CGTOP_EOF'
Control Group                                            Tasks   %CPU   Memory  Input/s Output/s
/                                                         1234   45.7     6.2G        -        -
/system.slice/docker.service                               123   78.3     2.1G        -        -
/system.slice/kubelet.service                               89   65.2     1.8G        -        -
CGTOP_EOF
        ;;
    "crictl")
        cat <<'CRICTL_EOF'
{
  "stats": [
    {
      "attributes": {
        "id": "resource-calc-container",
        "metadata": {
          "name": "resource-test"
        },
        "labels": {
          "io.kubernetes.pod.namespace": "resource-calc",
          "io.kubernetes.pod.name": "resource-calc-pod"
        }
      },
      "cpu": {
        "usageNanoCores": {
          "value": "1500000000"
        }
      },
      "memory": {
        "workingSetBytes": {
          "value": "2147483648"
        }
      }
    }
  ]
}
CRICTL_EOF
        ;;
esac
EOF

# Add the missing crictl error script (already created above, removing duplicate)
# cat > "$CPU_COMMANDS_DIR/crictl-test-error" <<'EOF'
# #!/bin/bash
# echo "ERRO[0000] validate service connection: CRI v1 runtime API is not implemented for endpoint" >&2
# exit 1
# EOF

# Copy integration-resource-calc from integration-all-commands (only if source exists and target doesn't)
if [ -f "$MOCK_COMMANDS_DIR/integration-all-commands" ] && [ ! -e "$MOCK_COMMANDS_DIR/integration-resource-calc" ]; then
    cp "$MOCK_COMMANDS_DIR/integration-all-commands" "$MOCK_COMMANDS_DIR/integration-resource-calc"
fi

# Make all mock scripts executable (only files, not directories)
find "$CPU_COMMANDS_DIR" -name "top-test-*" -type f -exec chmod +x {} \;
find "$CPU_COMMANDS_DIR" -name "systemd-cgtop-test-*" -type f -exec chmod +x {} \;
find "$CPU_COMMANDS_DIR" -name "crictl-test-*" -type f -exec chmod +x {} \;
find "$CPU_COMMANDS_DIR" -name "ig-test-*" -type f -exec chmod +x {} \;
find "$MOCK_COMMANDS_DIR" -name "integration-*" -type f -exec chmod +x {} \;

# echo "Created mock command scripts:"
# ls -la "$MOCK_COMMANDS_DIR"/*test* "$MOCK_COMMANDS_DIR"/integration*

# echo "Mock command data creation complete!"