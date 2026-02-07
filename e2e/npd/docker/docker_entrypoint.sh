#!/bin/bash
# Entrypoint that sets up bind mounts inside the container

# Check if we should simulate missing mpstat
if [ "$HIDE_MPSTAT" = "true" ]; then
    # Remove mpstat from PATH by renaming it
    if [ -f "/usr/bin/mpstat" ]; then
        mv /usr/bin/mpstat /usr/bin/mpstat.hidden 2>/dev/null || true
    fi
fi

# Only try to bind mount if we have mock directories
if [ -d "/mock-proc" ]; then
    # For files that exist in mock-proc, use bind mount
    for file in /mock-proc/*; do
        if [ -f "$file" ]; then
            filename=$(basename "$file")
            # Use bind mount to overlay the mock file
            mount --bind "$file" "/proc/$filename" 2>/dev/null || true
        fi
    done

    # Handle /proc/net subdirectory for DNS tests
    if [ -d "/mock-proc/net" ]; then
        mkdir -p /proc/net
        for file in /mock-proc/net/*; do
            if [ -f "$file" ]; then
                filename=$(basename "$file")
                # Copy mock file over real /proc file (bind mount doesn't work reliably in containers)
                cp "$file" "/proc/net/$filename" 2>/dev/null || true
            fi
        done
    fi
fi

if [ -d "/mock-sys" ]; then
    # Mount mock sys over real sys subdirectories
    if [ -d "/mock-sys/fs/cgroup" ]; then
        mkdir -p /sys/fs/cgroup
        mount --bind /mock-sys/fs/cgroup /sys/fs/cgroup
    fi
fi

if [ -d "/mock-tmp" ]; then
    mkdir -p /tmp

    if [ -f "/mock-tmp/npd_xid_last_seen.cache" ]; then
        cp /mock-tmp/npd_xid_last_seen.cache /tmp/npd_xid_last_seen.cache 2>/dev/null
    fi
fi

if [ -d "/mock-etc" ]; then
    # Copy mock configuration files for NPD startup tests (avoid mount conflicts)
    if [ -d "/mock-etc/node-problem-detector.d" ]; then
        echo "Copying mock NPD configuration files..."
        cp -r /mock-etc/node-problem-detector.d/* /etc/node-problem-detector.d/ 2>/dev/null || true
    fi
fi

# Update public settings based on environment variables for NPD startup tests
if [ -f "/etc/node-problem-detector.d/public-settings.json" ]; then
    # Update NPD validation toggle based on environment variable
    if [ -n "${NPD_VALIDATE_ENABLED:-}" ]; then
        echo "Updating npd-validate-in-prod to: $NPD_VALIDATE_ENABLED"
        jq --arg value "$NPD_VALIDATE_ENABLED" '.["npd-validate-in-prod"] = $value' /etc/node-problem-detector.d/public-settings.json > /tmp/public-settings.json && mv /tmp/public-settings.json /etc/node-problem-detector.d/public-settings.json
    fi
fi

if [ -d "/mock-var" ]; then
    # Mount mock var directories for NPD startup tests
    if [ -d "/mock-var/lib/kubelet" ]; then
        mkdir -p /var/lib/kubelet
        mount --bind /mock-var/lib/kubelet /var/lib/kubelet 2>/dev/null || {
            echo "Mount failed, copying files instead..."
            cp -r /mock-var/lib/kubelet/* /var/lib/kubelet/ 2>/dev/null || true
        }
    fi

    # Mount the /var/log/syslog file if the folder /var/log exists
    if [ -d "/mock-var/log" ] && [ -f "/mock-var/log/syslog" ]; then
        mkdir -p /var/log
        cp /mock-var/log/syslog /var/log/syslog 2>/dev/null
    fi
fi

if [ -d "/mock-home" ]; then
    # Mount mock home directories for NPD startup tests
    if [ -d "/mock-home/azureuser" ]; then
        mkdir -p /home/azureuser
        mount --bind /mock-home/azureuser /home/azureuser 2>/dev/null || {
            echo "Mount failed, copying files instead..."
            cp -r /mock-home/azureuser/* /home/azureuser/ 2>/dev/null || true
        }
    fi
fi

echo "Setting up required filesystems for IG..."

# Create mount points for IG filesystems
mkdir -p /sys/kernel/debug /sys/kernel/tracing /sys/fs/bpf
# Mount debugfs (required by IG)
if ! mountpoint -q /sys/kernel/debug; then
    echo "Mounting debugfs..."
    mount -t debugfs none /sys/kernel/debug || {
        echo "WARNING: Failed to mount debugfs. IG may need --auto-mount-filesystems"
    }
else
    echo "debugfs already mounted"
fi
# Mount tracefs (required by IG)
if ! mountpoint -q /sys/kernel/tracing; then
    echo "Mounting tracefs..."
    mount -t tracefs none /sys/kernel/tracing || {
        echo "WARNING: Failed to mount tracefs. IG may need --auto-mount-filesystems"
    }
else
    echo "tracefs already mounted"
fi
# Mount bpf filesystem (required by IG)
if ! mountpoint -q /sys/fs/bpf; then
    echo "Mounting bpf filesystem..."
    mount -t bpf none /sys/fs/bpf || {
        echo "WARNING: Failed to mount bpf filesystem. IG may need --auto-mount-filesystems"
    }
else
    echo "bpf filesystem already mounted"
fi

echo "Filesystem setup for IG complete."

# Execute the command
exec "$@"
