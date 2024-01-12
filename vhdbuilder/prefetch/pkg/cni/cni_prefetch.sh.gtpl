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

{{range $image := .Images}}
prefetch "{{$image.FullyQualifiedTag}}" "{{range $index, $binary := .Binaries}}{{if $index}} {{end}}{{$binary}}{{end}}"
{{end}}