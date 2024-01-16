#!/usr/bin/env bash
set -eux

prefetch() {
    local image=$1
    local files=$2
    
    mount_dir=$(mktemp -d)
    ctr -n k8s.io images mount "$image" "$mount_dir"

    for f in $files; do
        echo "prefetching $f in $image"
        path="$mount_dir/$f"
        stat -c %s "$path"
        cat "$path" > /dev/null
    done

    ctr -n k8s.io images unmount "$mount_dir"
}
prefetch "mcr.microsoft.com/containernetworking/cni-dropgz:v0.0.4.2" "dropgz"
prefetch "mcr.microsoft.com/containernetworking/cni-dropgz:v0.1.1" "dropgz"
prefetch "mcr.microsoft.com/oss/calico/cni:v3.24.6" "opt/cni/bin/bandwidth opt/cni/bin/calico opt/cni/bin/calico-ipam opt/cni/bin/flannel opt/cni/bin/host-local opt/cni/bin/install opt/cni/bin/loopback opt/cni/bin/portmap opt/cni/bin/tuning"
prefetch "mcr.microsoft.com/oss/calico/pod2daemon-flexvol:v3.24.6" "usr/local/bin/flexvol"