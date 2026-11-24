#!/bin/bash

lpsColdObserve() {
    timeout --signal=TERM --kill-after=2s 20s python3 \
        /opt/azure/containers/lps-json-cold-ab/observe.py "$1" ||
        echo "LPS cold observation $1 failed; evidence invalid, provisioning continues"
}

lpsAsyncPrefetch() (
    set +x
    umask 077
    mkdir -p /run/aks-lps-json-prefetch || return 1
    export -p > /run/aks-lps-json-prefetch/environment.sh || return 1
    local result=0
    timeout --signal=TERM --kill-after=2s 15s systemd-run \
        --unit=aks-lps-json-prefetch \
        --property=Type=exec --property=RuntimeMaxSec=330 \
        --property=TimeoutStopSec=10 --property=KillMode=control-group \
        --property=UMask=0077 \
        --property='ExecStopPost=/bin/bash /opt/azure/containers/lps-json-cold-ab/worker.sh unit-exit' \
        /bin/bash /opt/azure/containers/lps-json-cold-ab/worker.sh "$1" || result=$?
    python3 /opt/azure/containers/lps-json-cold-ab/observe.py launch-result "$result" || true
    return "$result"
)
