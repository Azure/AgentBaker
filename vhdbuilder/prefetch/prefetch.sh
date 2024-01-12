#!/usr/bin/env bash

set -xe

prefetch() {
    image=$1
    files=$2
    mntdir=$(mktemp -d)
    ctr -n k8s.io images mount "$image" "$mntdir"
    for f in $files; do
        echo "prefetching $f in $image"
        fullpath="$mntdir/$f"
        stat -c %s "$fullpath"
        cat "$fullpath" > /dev/null
    done
    umount $mntdir
}


prefetch "mcr.microsoft.com/containernetworking/cni-dropgz:v0.0.4.2" "dropgz"

prefetch "mcr.microsoft.com/containernetworking/cni-dropgz:v0.1.1" "dropgz"

prefetch "mcr.microsoft.com/oss/calico/cni:v3.24.6" "opt/cni/bin/bandwidth opt/cni/bin/calico opt/cni/bin/calico-ipam opt/cni/bin/flannel opt/cni/bin/host-local opt/cni/bin/install opt/cni/bin/loopback opt/cni/bin/portmap opt/cni/bin/tuning"

prefetch "mcr.microsoft.com/oss/calico/pod2daemon-flexvol:v3.24.6" "usr/local/bin/flexvol"
