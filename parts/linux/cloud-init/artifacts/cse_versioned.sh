#!/usr/bin/env bash
ERR_FILE_WATCH_TIMEOUT=6 {{/* Timeout waiting for a file */}}
wait_for_file() {
    retries=$1; wait_sleep=$2; filepath=$3
    paved=/opt/azure/cloud-init-files.paved
    grep -Fq "${filepath}" $paved && return 0
    for i in $(seq 1 $retries); do
        grep -Fq '#EOF' $filepath && break
        if [ $i -eq $retries ]; then
            return 1
        else
            sleep $wait_sleep
        fi
    done
    sed -i "/#EOF/d" $filepath
    echo $filepath >> $paved
}

SCRIPT="{{GetVersionedCSEScriptFilepath}}"
wait_for_file 1200 1 "$SCRIPT" || exit $ERR_FILE_WATCH_TIMEOUT
/usr/bin/nohup /bin/bash -c "/bin/bash '$SCRIPT'"
