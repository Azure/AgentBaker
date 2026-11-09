#!/bin/bash
set -euo pipefail

check_trace_target() {
    local backend="$1" base_version="$2" image_label="$3"
    local os_id os_version
    os_id=$(sed -n 's/^ID=//p' /etc/os-release | tr -d '"')
    os_version=$(sed -n 's/^VERSION_ID=//p' /etc/os-release | tr -d '"')
    if [[ "$os_id" != ubuntu || "$os_version" != 26.04 || "$(uname -m)" != x86_64 ]]; then
        echo "Boot tracing only supports Ubuntu 26.04 x86_64" >&2
        return 1
    fi
    if [[ "$backend" != ftrace && "$backend" != lttng ]]; then
        echo "Expected ftrace or lttng" >&2
        return 1
    fi
    if [[ ! "$base_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ || ! "$image_label" =~ ^[a-zA-Z0-9._-]+$ ]]; then
        echo "An exact base image version and a safe image label are required" >&2
        return 1
    fi
    if [[ "$(uname -r)" != 7.0.0-1020-azure ]]; then
        echo "Recheck boot-trace compatibility before changing the pinned kernel" >&2
        return 1
    fi
}

install_ftrace_recorder() (
    set -euo pipefail
    local directory
    directory=$(mktemp -d)
    trap 'rm -f -- "$directory/trace-cmd.deb" "$directory/libtraceevent.deb" "$directory/plugins.deb"; rmdir -- "$directory"' EXIT
    chmod 0755 "$directory"
    # Resolute's trace-cmd 3.3.3 lacks -K and would erase the early boot trace.
    curl -fsSL https://archive.ubuntu.com/ubuntu/pool/main/t/trace-cmd/trace-cmd_3.4-2_amd64.deb \
        -o "$directory/trace-cmd.deb"
    curl -fsSL https://archive.ubuntu.com/ubuntu/pool/main/libt/libtraceevent/libtraceevent1_1.9.0-2_amd64.deb \
        -o "$directory/libtraceevent.deb"
    curl -fsSL https://archive.ubuntu.com/ubuntu/pool/main/libt/libtraceevent/libtraceevent1-plugin_1.9.0-2_amd64.deb \
        -o "$directory/plugins.deb"
    (
        cd "$directory"
        sha512sum -c <<'EOF'
12d77cc31987a68f7da44342cd9ba2130ad70fc9a03c7c1b571ad2ac9a87314ac294e8fb786d3ab409cee852736a59af223a27c25c9d1e7329548f6dee21e5b0  trace-cmd.deb
49adf52e3e8ee5fede5ce5db03e1960b8fdf50d2864c8396c4553868951a1fe18185529f9dba6d6c609b4784c78a13f6a3937f8899638dafb4327a5a3f7f3d51  libtraceevent.deb
873c4d8e01ee45407068a39e9be3e9e11513531c69e54e30a6bf72d37ffbc145c1eed672d0d25822cfb74162ab2f771d1a83314c0f116ba921feddcaa86a5d57  plugins.deb
EOF
    )
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
        "$directory/trace-cmd.deb" "$directory/libtraceevent.deb" "$directory/plugins.deb"
    apt-mark manual trace-cmd libtraceevent1 libtraceevent1-plugin
)

prepare_trace_image() {
    local backend="$1" base_version="$2" image_label="$3" source_dir="$4"
    local max_mib=256
    check_trace_target "$backend" "$base_version" "$image_label"
    if [[ "$backend" == ftrace ]]; then
        max_mib=512
    fi
    install -d -m 0755 /opt/aks-node-boot-trace
    install -m 0644 "$source_dir/capture.py" "$source_dir/stop_capture.py" "$source_dir/diagnostics.py" \
        "$source_dir/ftrace-grub.cfg" /opt/aks-node-boot-trace/
    install -m 0755 "$source_dir/install.sh" /opt/aks-node-boot-trace/install.sh
    install -m 0644 "$source_dir/aks-node-boot-trace.service" /etc/systemd/system/
    printf 'BACKEND=%s\nMAX_SECONDS=600\nMAX_MIB=%s\nIMAGE_LABEL=%s\n' \
        "$backend" "$max_mib" "$image_label" > /etc/aks-node-boot-trace.env
    chmod 0600 /etc/aks-node-boot-trace.env
    printf 'backend=%s\nbase_version=%s\nkernel=%s\n' \
        "$backend" "$base_version" "$(uname -r)" > /opt/aks-node-boot-trace/image.txt

    if [[ "$backend" == ftrace ]]; then
        install_ftrace_recorder
    else
        apt-get update
        DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
            lttng-tools=2.14.1-1build1 lttng-modules-dkms=2.15.0-1ubuntu1.1
        apt-mark manual lttng-tools lttng-modules-dkms "linux-headers-$(uname -r)"
        dkms autoinstall -k "$(uname -r)"
        depmod -a "$(uname -r)"
        modinfo -k "$(uname -r)" lttng-tracer
        systemctl disable --now lttng-sessiond.service
        install -d -m 0755 /etc/systemd/system/lttng-sessiond.service.d \
            /etc/systemd/system/aks-node-boot-trace.service.d
        cat > /etc/systemd/system/lttng-sessiond.service.d/boot-trace.conf <<'EOF'
[Unit]
DefaultDependencies=no
After=local-fs.target
Before=aks-node-boot-trace.service sysinit.target shutdown.target
Conflicts=shutdown.target

[Service]
EnvironmentFile=
Environment=DAEMON_ARGS=
EOF
        cat > /etc/systemd/system/aks-node-boot-trace.service.d/lttng.conf <<'EOF'
[Unit]
Requires=lttng-sessiond.service
After=lttng-sessiond.service
EOF
    fi
    dpkg-query -W -f='${binary:Package}=${Version}\n' python3 "linux-image-$(uname -r)" \
        >> /opt/aks-node-boot-trace/image.txt
    if [[ "$backend" == ftrace ]]; then
        dpkg-query -W -f='${binary:Package}=${Version}\n' trace-cmd libtraceevent1 libtraceevent1-plugin \
            >> /opt/aks-node-boot-trace/image.txt
    else
        dpkg-query -W -f='${binary:Package}=${Version}\n' lttng-tools lttng-modules-dkms \
            >> /opt/aks-node-boot-trace/image.txt
    fi
    systemctl daemon-reload
    systemctl disable aks-node-boot-trace.service
}

arm_trace_image() {
    local backend="$1" base_version="$2" image_label="$3"
    local kernel config flag event
    check_trace_target "$backend" "$base_version" "$image_label"
    kernel=$(uname -r)
    config="/boot/config-$kernel"
    for flag in MODULES KALLSYMS HIGH_RES_TIMERS TRACEPOINTS KPROBES EVENT_TRACING; do
        grep -qx "CONFIG_${flag}=y" "$config"
    done
    if [[ -d /var/log/aks-node-boot-trace ]]; then
        echo "A capture already exists on the builder; refusing to seal this image" >&2
        return 1
    fi
    if [[ "$backend" == ftrace ]]; then
        if [[ ! -e /sys/kernel/tracing/available_events ]]; then
            mount -t tracefs tracefs /sys/kernel/tracing
        fi
        for event in sched:sched_switch sched:sched_wakeup sched:sched_wakeup_new \
            sched:sched_process_fork sched:sched_process_exec sched:sched_process_exit \
            block:block_rq_issue block:block_rq_complete \
            net:net_dev_xmit net:netif_receive_skb_entry \
            net:netif_receive_skb_list_entry net:netif_rx_entry; do
            grep -qx "$event" /sys/kernel/tracing/available_events
        done
        install -m 0644 /opt/aks-node-boot-trace/ftrace-grub.cfg \
            /etc/default/grub.d/99-aks-boot-trace.cfg
        update-grub
    else
        modinfo -k "$kernel" lttng-tracer
        if [[ -e /etc/default/grub.d/99-aks-boot-trace.cfg ]]; then
            echo "LTTng image must not contain ftrace boot arming" >&2
            return 1
        fi
    fi
    python3 /opt/aks-node-boot-trace/diagnostics.py \
        --instrument-cloud /opt/azure/containers/init-aks-cloud.sh
    systemctl enable aks-node-boot-trace.service
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    if [[ "$#" != 4 ]]; then
        echo "Usage: install.sh prepare|arm ftrace|lttng BASE_VERSION IMAGE_LABEL" >&2
        exit 1
    fi
    case "$1" in
        prepare) prepare_trace_image "$2" "$3" "$4" "$(dirname "$0")" ;;
        arm) arm_trace_image "$2" "$3" "$4" ;;
        *) echo "Expected prepare or arm" >&2; exit 1 ;;
    esac
fi
