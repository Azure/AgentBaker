#!/bin/bash
set -uo pipefail
set +x
base=/opt/azure/containers/lps-json-cold-ab
if [ "${1:-}" = unit-exit ]; then
    exec python3 "$base/observe.py" unit-exit
fi
# Root-only context preserves scriptless JSON input, feature flags, proxy/CA and IMDS settings.
if [ "${1:-}" = sync ]; then
    export LPS_PREFETCH_MODE=sync
    shift
else
    native_invocation_id="${INVOCATION_ID:-}"
    # shellcheck source=/dev/null
    source /run/aks-lps-json-prefetch/environment.sh
    # Exported CSE context may contain its parent's invocation ID.
    export INVOCATION_ID="$native_invocation_id"
    export LPS_PREFETCH_MODE=async
fi
export LPS_PREFETCH_WORKER_PID=$$
python3 "$base/observe.py" worker-start || true
timeout --signal=TERM --kill-after=10s 300s "$1" prepull-images --source networking
result=$?
if ! python3 "$base/observe.py" worker-end "$result"; then
    [ "$result" -ne 0 ] || result=1
fi
exit "$result"
