#!/bin/bash

# This check is designed to fail fast, starting by failing if the system doesn't support systemd. From that point, we should order
#   the checks to be as high level to low level as possible, ensuring that obvious failures are picked up first and exit quickly.

OK=0
NOTOK=1
UNKNOWN=2
COMMAND_TIMEOUT_SECONDS=10

if ! which systemctl >/dev/null; then
    echo "Systemd is not supported"
    exit $UNKNOWN
fi

# Check to ensure that a kubelet service is running
# In some cases, we observed situations where service was still active, but this check reported it as being down because systemctl command timed out
# We want to be able to distinguish between systemctl failing to execute and service being down (note - just because it timed out, it doesn't necessarily mean that the service is up)
timeout ${COMMAND_TIMEOUT_SECONDS} systemctl -q is-active kubelet.service
kubelet_service_exit_code=$?
if [ $kubelet_service_exit_code -ne 0 ]; then
    if [ $kubelet_service_exit_code -eq 124 ]; then
        echo "Systemctl command to check kubelet service timed out after ${COMMAND_TIMEOUT_SECONDS} seconds"
        exit $OK
    else
        echo "Kubelet service is not running, exit code: ${kubelet_service_exit_code}"
        exit $NOTOK
    fi
fi

# Check to ensure that kubelet is healthy
if ! output=$(curl -m "${COMMAND_TIMEOUT_SECONDS}" -f -s -S http://127.0.0.1:10248/healthz 2>&1); then
    echo "kubelet healthcheck failed: ${output}"
    exit $NOTOK
fi

echo "Kubelet service passed all checks and is running"
exit $OK
