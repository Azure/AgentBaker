#!/bin/bash
source /etc/node-problem-detector.d/plugin/runtime.sh

# This check is designed to fail fast, starting by failing if the system doesn't support systemd. From that point, we should order
#   the checks to be as high level to low level as possible, ensuring that obvious failures are picked up first and exit quickly.

OK=0
NOTOK=1
UNKNOWN=2

HEALTHCHECK_COMMAND="${HEALTHCHECK_COMMAND:-docker ps}"
HEALTHCHECK_COMMAND_TIMEOUT="${HEALTHCHECK_COMMAND_TIMEOUT:-60}"

if ! which systemctl >/dev/null; then
    echo "Systemd is not supported"
    exit $UNKNOWN
fi

# As long as the runtime endpoint returns a socket containing "containerd.sock", we can assume that containerd is in use.
# But we have seen other container runtimes used in BYO, these were also using the "containerd.sock" endpoint.
# So while we're calling this containerd it's possible for other runtimes to use the same endpoint.
# Upstream NPD only checks for the "containerd.sock" endpoint, so we'll do the same.
CONTAINER_RUNTIME=$(getKubeletRuntime)
if [[ ${CONTAINER_RUNTIME} == "containerd" ]]; then
  # default timeout is 2 seconds. Under load, crictl sometimes takes longer causing NPD to incorrectly mark
  # containerd as failed. At the time of writing, the default HEALTHCHECK_COMMAND_TIMEOUT was 3 minutes, so let's use
  # a 60s timeout to crictl. Don't want to use the HEALTHCHECK_COMMAND_TIMEOUT value as crictl has quite specific parsing
  # rules for the timeout.
  HEALTHCHECK_COMMAND="crictl -t 60s pods --latest"
fi

# Check to ensure that a container runtime service is running
# In some cases, we observed situations where service was still active, but this check reported it as being down because systemctl command timed out
# We want to be able to distinguish between systemctl failing to execute and service being down (note - just because it timed out, it doesn't necessarily mean that the service is up)
timeout ${HEALTHCHECK_COMMAND_TIMEOUT} systemctl -q is-active ${CONTAINER_RUNTIME}.service
runtime_service_exit_code=$?
if [ $runtime_service_exit_code -ne 0 ]; then
    if [ $runtime_service_exit_code -eq 124 ]; then
        echo "Systemctl command to check container runtime service timed out after ${HEALTHCHECK_COMMAND_TIMEOUT} seconds"
        exit $OK
    else
        echo "${CONTAINER_RUNTIME} service is not running, exit code: ${runtime_service_exit_code}"
        exit $NOTOK
    fi
fi

# Check to ensure that a container runtime service is functioning
if ! timeout ${HEALTHCHECK_COMMAND_TIMEOUT} ${HEALTHCHECK_COMMAND} >/dev/null; then
    echo "${HEALTHCHECK_COMMAND} failed!"
    exit $NOTOK
fi

echo "${CONTAINER_RUNTIME} service passed all checks and is running"
exit $OK
