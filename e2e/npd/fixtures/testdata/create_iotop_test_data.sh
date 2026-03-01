#!/bin/bash
# Create comprehensive iotop mock data for testing
# This script creates various iotop output scenarios to test JSON processing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Create iotop mock commands directory in cpu subdirectory
MOCK_COMMANDS_DIR="$SCRIPT_DIR/mock-commands/cpu"
mkdir -p "$MOCK_COMMANDS_DIR"

# Remove any existing iotop test files that might conflict
rm -rf "$MOCK_COMMANDS_DIR"/iotop-test-*


# Scenario 1: Standard iotop output with known values for validation
create_iotop_standard() {
    # Include a number of different units of measure to test the parsing of the iotop output (B/s, K/s, M/s, G/s)   
    cat > "$MOCK_COMMANDS_DIR/iotop-test-standard" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 15.67 B/s | Total DISK WRITE: 8.90 K/s
Current DISK READ: 5.23 M/s | Current DISK WRITE: 3.45 G/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
 1234    be/4 root      10.50 M/s    4.20 M/s  ?unavailable?  [kworker/u16:2]
 5678    be/4 postgres   3.17 M/s    2.70 M/s  ?unavailable?  postgres: writer process
 9012    be/4 mysql      2.00 M/s    2.00 M/s  ?unavailable?  mysqld --defaults-file=/etc/mysql/my.cnf
IOTOP_OUTPUT
EOF
}

# Scenario 2: iotop with special characters in commands (tests JSON escaping)
create_iotop_special_chars() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-special-chars" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 2.34 K/s | Total DISK WRITE: 1.56 K/s
Current DISK READ: 1.12 K/s | Current DISK WRITE: 0.78 K/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
 1111    be/4 user      1.50 K/s    1.00 K/s  ?unavailable? node "app with spaces.js" --config="test.json"
 2222    be/4 root      0.84 K/s    0.56 K/s  ?unavailable? bash -c 'echo "test" | grep "pattern"'
 3333    be/4 app       0.34 K/s    0.22 K/s  ?unavailable? /usr/bin/app --data=/path/with\backslashes --quote="value"
IOTOP_OUTPUT
EOF
}

# Scenario 3: iotop with very long command names (tests truncation)
create_iotop_long_commands() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-long-commands" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 1.23 K/s | Total DISK WRITE: 0.89 K/s
Current DISK READ: 0.67 K/s | Current DISK WRITE: 0.45 K/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
 3333    be/4 app       1.23 K/s    0.89 K/s  ?unavailable? /usr/bin/very-long-application-name-that-exceeds-normal-limits --with-many-arguments --config=/very/long/path/to/configuration/file.conf --data-dir=/another/very/long/path/that/should/be/truncated --log-level=debug --enable-feature-x --enable-feature-y
IOTOP_OUTPUT
EOF
}

# Scenario 4: iotop with mixed data units (B/s, K/s, M/s, G/s)
create_iotop_mixed_units() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-mixed-units" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 1.25 G/s | Total DISK WRITE: 856.34 M/s
Current DISK READ: 45.67 M/s | Current DISK WRITE: 23.45 M/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
 1001    be/4 root      500.00 M/s  200.00 M/s  ?unavailable? dd if=/dev/zero of=/tmp/test
 1002    be/4 user      15.67 M/s   12.34 M/s  ?unavailable? rsync -av /source/ /dest/
 1003    be/4 postgres  8.90 M/s    6.78 M/s  ?unavailable? postgres: checkpointer process
 1004    be/4 mysql     2.34 K/s    1.56 K/s  ?unavailable? mysqld --innodb-buffer-pool-size=1G
 1005    be/4 app       567.89 B/s  234.56 B/s  ?unavailable? /usr/bin/app --low-io-mode
IOTOP_OUTPUT
EOF
}

# Scenario 5: iotop with many processes (tests process count limiting)
create_iotop_many_processes() {
    # Remove existing file first
    rm -rf "$MOCK_COMMANDS_DIR/iotop-test-many-processes"
    
    cat > "$MOCK_COMMANDS_DIR/iotop-test-many-processes" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 50.25 M/s | Total DISK WRITE: 30.45 M/s
Current DISK READ: 25.12 M/s | Current DISK WRITE: 15.23 M/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
EOF

# Generate many processes to test head -n 20 limitation behavior
for i in {2000..2025}; do
    cat >> "$MOCK_COMMANDS_DIR/iotop-test-many-processes" <<EOF
 $i    be/4 user$((i % 5))   $((RANDOM % 10 + 1)).$(printf "%02d" $((RANDOM % 100))) M/s   $((RANDOM % 5 + 1)).$(printf "%02d" $((RANDOM % 100))) M/s  ?unavailable?  process-$i --config=/etc/config$i.conf
EOF
done
}

# Scenario 6: iotop permission denied (tests error handling)
create_iotop_permission_denied() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-permission-denied" <<'EOF'
#!/bin/bash
echo "iotop: Permission denied" >&2
exit 1
EOF
}

# Scenario 7: iotop empty output (tests empty data handling)
create_iotop_empty_output() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-empty" <<'EOF'
#!/bin/bash
# Return empty output
exit 0
EOF
}

# Scenario 8: iotop malformed output (tests error handling)
create_iotop_malformed_output() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-malformed" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
This is not valid iotop output
It should be handled gracefully
No proper headers or data format
IOTOP_OUTPUT
EOF
}

# Scenario 9: iotop with Unicode characters (tests character encoding)
create_iotop_unicode_chars() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-unicode" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 3.45 K/s | Total DISK WRITE: 2.34 K/s
Current DISK READ: 1.67 K/s | Current DISK WRITE: 1.23 K/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
 4444    be/4 测试用户    2.34 K/s    1.56 K/s  ?unavailable? /usr/bin/测试应用 --配置=/etc/测试.conf
 5555    be/4 user      1.11 K/s    0.78 K/s  ?unavailable? python3 -c "print('Hello 世界')"
IOTOP_OUTPUT
EOF
}

# Scenario 10: iotop with tab characters and newlines in commands (tests escaping)
create_iotop_control_chars() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-control-chars" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ: 1.89 K/s | Total DISK WRITE: 1.23 K/s
Current DISK READ: 0.95 K/s | Current DISK WRITE: 0.67 K/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
 6666    be/4 root      1.89 K/s    1.23 K/s  ?unavailable? bash -c $'echo "line1\nline2\ttab"'
 7777    be/4 user      0.45 K/s    0.34 K/s  ?unavailable? awk 'BEGIN{print "test\ttab"}'
IOTOP_OUTPUT
EOF
}

# Scenario 11: Production issue - java command ending with dash (tests real production bug)
create_iotop_production_issue_end_dash() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-production-issue-end-dash" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ:         0.00 B/s | Total DISK WRITE:       143.35 K/s
Current DISK READ:       0.00 B/s | Current DISK WRITE:       0.00 B/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
    394 be/3 root        0.00 B/s   43.00 K/s  ?unavailable?  systemd-journald
2373181 be/4 testuser    0.00 B/s   57.34 K/s  ?unavailable?  java -Dapp.config.url=http://config.example.com -Dapp.database.host=db.example.com -Dapp.redis.host=redis.example.com:6379 -Dapp.logging.level=INFO -Dapp.feature.enabled=true -Dapp.timeout.ms=30000 -Dapp.pool.size=10 -Dapp.cache.ttl=3600 -Dapp.auth.enabled=false -Dapp.ssl.verify=false -Dapp.metrics.enabled=true -Dapp.batch.size=100 -
IOTOP_OUTPUT
EOF
}

# Scenario 12: Production issue - commandline containing `ERROR` detected as failure to run iotop (tests real production bug)
create_iotop_production_issue_commandline_containing_error() {
    cat > "$MOCK_COMMANDS_DIR/iotop-test-production-issue-commandline_containing_error" <<'EOF'
#!/bin/bash
cat <<'IOTOP_OUTPUT'
Total DISK READ:         0.00 B/s | Total DISK WRITE:       143.35 K/s
Current DISK READ:       0.00 B/s | Current DISK WRITE:       0.00 B/s
    TID  PRIO  USER     DISK READ  DISK WRITE  SWAPIN      IO    COMMAND
    394 be/3 root        0.00 B/s   43.00 K/s  ?unavailable?  systemd-journald
2373181 be/4 testuser    0.00 B/s   57.34 K/s  ?unavailable?  java -DLOG_LEVEL=DEBUG,ERROR,INFO -Ds3proxy.endpoint=http://0.0.0.0:8080 -Ds3proxy.secure-endpoint= -Ds3proxy.virtual-host= -Ds3proxy.keystore-path=***REDACTED*** -Ds3proxy.keystore-password=***REDACTED*** -Ds3proxy.authorization=***REDACTED***
IOTOP_OUTPUT
EOF
}

create_iotop_standard
create_iotop_special_chars
create_iotop_long_commands
create_iotop_mixed_units
create_iotop_many_processes
create_iotop_permission_denied
create_iotop_empty_output
create_iotop_malformed_output
create_iotop_unicode_chars
create_iotop_control_chars
create_iotop_production_issue_end_dash
create_iotop_production_issue_commandline_containing_error

# Make all iotop test commands executable (only files, not directories)
find "$MOCK_COMMANDS_DIR" -name "iotop-test-*" -type f -exec chmod +x {} \;

#ls -la "$MOCK_COMMANDS_DIR"/iotop-test-* | sed 's/^/  /' > /dev/null 2>&1
