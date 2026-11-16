#!/bin/bash

install_azurelinux_ftrace_recorder() (
    set -euo pipefail
    local directory=/home/packer/boot-trace-build
    local prefix=/opt/aks-node-boot-trace
    local package help_status=0
    local -a added_build_packages=()
    if [[ -e "$directory" ]]; then
        echo "Boot-trace build directory already exists: $directory" >&2
        return 1
    fi
    mkdir -p "$directory"
    trap 'rm -rf -- /home/packer/boot-trace-build' EXIT

    # Azure Linux's recorder lacks -K. Build the same upstream version as Ubuntu.
    curl -fsSL --connect-timeout 30 --max-time 120 \
        https://archive.ubuntu.com/ubuntu/pool/main/t/trace-cmd/trace-cmd_3.4.orig.tar.gz \
        -o "$directory/trace-cmd.tar.gz"
    curl -fsSL --connect-timeout 30 --max-time 120 \
        https://archive.ubuntu.com/ubuntu/pool/main/libt/libtraceevent/libtraceevent_1.9.0.orig.tar.xz \
        -o "$directory/libtraceevent.tar.xz"
    (
        cd "$directory"
        sha256sum -c <<'EOF'
604085ab395cc2cb84397dc3d209ea0d0b15ae506f8ea91ff45c86e5a050213c  trace-cmd.tar.gz
1ed2e47cfd44080fda9c9e33be4b128680958001ced286fe719abc85136b259d  libtraceevent.tar.xz
EOF
    )
    for package in gcc binutils make pkgconf-pkg-config glibc-devel kernel-headers libtracefs-devel gawk; do
        if ! rpm -q "$package" > /dev/null 2>&1; then
            added_build_packages+=("$package")
        fi
    done
    tdnf install -y gcc binutils make pkgconf-pkg-config glibc-devel kernel-headers \
        libtracefs-devel-1.8.0-2.azl3 libtracefs-1.8.0-2.azl3 gawk python3 tar xz
    tar -xf "$directory/trace-cmd.tar.gz" -C "$directory"
    tar -xf "$directory/libtraceevent.tar.xz" -C "$directory"
    make -C "$directory/libtraceevent-1.9.0" -j2 prefix="$prefix" libdir="$prefix/lib"
    make -C "$directory/libtraceevent-1.9.0" prefix="$prefix" libdir="$prefix/lib" install
    PKG_CONFIG_PATH="$prefix/lib/pkgconfig" make -C "$directory/trace-cmd-v3.4" -j2 \
        NO_PYTHON=1 NO_LIBZSTD=1 \
        LDFLAGS="-L$prefix/lib -Wl,--disable-new-dtags,-rpath,$prefix/lib" all_cmd
    install -d -m 0755 "$prefix/bin" "$prefix/licenses"
    install -m 0755 "$directory/trace-cmd-v3.4/tracecmd/trace-cmd" "$prefix/bin/trace-cmd"
    install -m 0644 "$directory/trace-cmd-v3.4/COPYING" "$prefix/licenses/trace-cmd"
    install -m 0644 "$directory/trace-cmd-v3.4/COPYING.LIB" "$prefix/licenses/trace-cmd-library"
    install -m 0644 "$directory/libtraceevent-1.9.0/LICENSES/GPL-2.0" "$prefix/licenses/libtraceevent-GPL-2.0"
    install -m 0644 "$directory/libtraceevent-1.9.0/LICENSES/LGPL-2.1" "$prefix/licenses/libtraceevent-LGPL-2.1"
    if [[ ${#added_build_packages[@]} -gt 0 ]]; then
        tdnf remove -y "${added_build_packages[@]}"
    fi
    "$prefix/bin/trace-cmd" record -h > "$directory/record-help.txt" 2>&1 || help_status=$?
    if [[ "$help_status" != 0 && "$help_status" != 255 ]] ||
        ! grep -q -- '-K' "$directory/record-help.txt"; then
        cat "$directory/record-help.txt" >&2
        echo "Built trace-cmd does not expose the required record -K option" >&2
        return 1
    fi
    printf 'trace-cmd=3.4\nboot_trace_libtraceevent=1.9.0\n' >> "$prefix/image.txt"
    rpm -q --queryformat '%{NAME}=%{VERSION}-%{RELEASE}\n' python3 libtracefs libtraceevent \
        >> "$prefix/image.txt"
)

arm_azurelinux_ftrace() {
    local grub=/etc/default/grub
    if [[ ! -f "$grub" || ! -f /boot/grub2/grub.cfg ]]; then
        echo "Expected the Azure Linux 3 GRUB boot configuration" >&2
        return 1
    fi
    if ! grep -qxF '. /opt/aks-node-boot-trace/ftrace-grub.cfg' "$grub"; then
        printf '\n. /opt/aks-node-boot-trace/ftrace-grub.cfg\n' >> "$grub"
    fi
    grub2-mkconfig -o /boot/grub2/grub.cfg
    grub2-script-check /boot/grub2/grub.cfg
}
