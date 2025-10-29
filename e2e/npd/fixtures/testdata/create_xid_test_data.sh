#!/bin/bash
# test/node-problem-detector/fixtures/testdata/create_xid_test_data.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

create_xid_setup() {
    echo "Setting up mock data for XID error tests..."
    local dir="$SCRIPT_DIR/mock-data/check-xid-no-errors"

    mkdir -p "$dir/mock-commands"

 # Working nvidia-smi with NVLink support
    cat > "$dir/mock-commands/nvidia-smi" <<'EOF'
#!/bin/bash
exit 0
EOF

    cat > "$dir/mock-commands/date" <<'EOF'
#!/bin/bash

# Check if the first argument is %+Y
if [[ "$1" == "+%Y" ]]; then
    echo "2025"
    exit 0
fi

# Check if the first argument is +%s
if [[ "$1" == "+%s" ]]; then
    echo "1754506801"
    exit 0
fi

if [[ "$1" == "--date" && "$2" == "Aug 06 12:00:00 2025" ]]; then
    echo "1754506800"
    exit 0
fi

echo "foo"
EOF

    # Make all commands executable
    chmod +x "$dir/mock-commands"/*
}

create_xid_error_48() {
    echo "Creating mock XID error 48 log entry..."
    local dir="$SCRIPT_DIR/mock-data/check-xid-errors-48"
    mkdir -p "$dir"
    cp -r "$SCRIPT_DIR/mock-data/check-xid-no-errors"/* "$dir/"

    mkdir -p "$dir/var/log"
    syslog="$dir/var/log/syslog"

    # Create a mock syslog entry for XID error 48
    echo "Aug 06 12:00:00 hostname NVRM: Xid (0000:03:00): 48, Channel 00000001" > "$syslog"

    mkdir -p "$dir/tmp"
    echo "1754506700" > "$dir/tmp/npd_xid_last_seen.cache"
}

create_xid_error_48_56() {
    echo "Creating mock XID error 48 and 56 log entry..."
    local dir="$SCRIPT_DIR/mock-data/check-xid-errors-48-56"
    mkdir -p "$dir"
    cp -r "$SCRIPT_DIR/mock-data/check-xid-errors-48"/* "$dir/"

    mkdir -p "$dir/var/log"
    syslog="$dir/var/log/syslog"

    # Create a mock syslog entry for XID error 56, note that this is in addition
    # to the previous error 48.
    echo "Aug 06 12:00:00 hostname NVRM: Xid (0000:03:00): 56, Channel 00000001" >> "$syslog"

    mkdir -p "$dir/tmp"
    echo "1754506700" > "$dir/tmp/npd_xid_last_seen.cache"
}

create_xid_setup
create_xid_error_48
create_xid_error_48_56
