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

# These are the most important, since they install the CNI binaries and conflist.
# If this is too much data, we could live with just the most recent version.
# (We can *maybe* remove pod2daemon-flexvol from Calico entirely.)
#
#    Image:                                  All:    Prefetched Files:
#    containernetworking/cni-dropgz:v0.0.9    72M             72M
#    calico/podman2daemon-flexvol:v3.24.6     15M              5M
#    calico/cni:v3.24.6                      203M            145M
prefetch "mcr.microsoft.com/containernetworking/cni-dropgz:v0.0.9" "dropgz"
prefetch "mcr.microsoft.com/oss/calico/pod2daemon-flexvol:v3.24.6" "usr/local/bin/flexvol"
prefetch "mcr.microsoft.com/oss/calico/cni:v3.24.6" "opt/cni/bin/bandwidth opt/cni/bin/calico opt/cni/bin/calico-ipam opt/cni/bin/flannel opt/cni/bin/host-local opt/cni/bin/install opt/cni/bin/loopback opt/cni/bin/portmap opt/cni/bin/tuning"

# These technically won't block node readiness,
# but until they're running, CNI invocations will fail.
#
#   Image:                                    All:    Prefetched Files:
#   containernetworking/azure-cns:v1.5.5      255M            66M
#   cilium/cilium:1.12.10-1                   452M            77M
#   calico/node:v3.24.6                       245M           103M
prefetch "mcr.microsoft.com/containernetworking/azure-cns:v1.5.5" "usr/local/bin/azure-cns"
prefetch "mcr.microsoft.com/oss/cilium/cilium:1.12.10-1" "cni-install.sh init-container.sh usr/bin/cilium-agent"
prefetch "mcr.microsoft.com/oss/calico/node:v3.24.6" "usr/bin/calico-node"
