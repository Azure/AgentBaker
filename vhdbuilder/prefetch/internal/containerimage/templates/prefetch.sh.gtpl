#!/usr/bin/env bash
set -eux

prefetch() {
    local image=$1
    local files=$2
    
    mount_dir=$(mktemp -d)
    ctr -n k8s.io images mount "$image" "$mount_dir"

    for f in $files; do
        echo "prefetching $f in $image"
        path="${mount_dir}${f}"
        stat -c %s "$path"
        cat "$path" > /dev/null
    done

    ctr -n k8s.io images unmount "$mount_dir"
}

{{- range $image := .Images}}
prefetch "{{$image.Tag}}" "{{range $index, $binary := $image.Binaries}}{{if $index}} {{end}}{{$binary}}{{end}}"
{{- end}}