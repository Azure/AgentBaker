#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 302/364: 'service_systemd-coredump_disabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
# Service doesn't exist... allow this to fail
"$SYSTEMCTL_EXEC" stop 'systemd-coredump.service' || true
"$SYSTEMCTL_EXEC" disable 'systemd-coredump.service'  || true
"$SYSTEMCTL_EXEC" mask 'systemd-coredump.service'
# Disable socket activation if we have a unit file for it
if "$SYSTEMCTL_EXEC" list-unit-files | grep -q '^systemd-coredump.socket'; then
    "$SYSTEMCTL_EXEC" stop 'systemd-coredump.socket'
    "$SYSTEMCTL_EXEC" mask 'systemd-coredump.socket'
fi
# The service may not be running because it has been started and failed,
# so let's reset the state so OVAL checks pass.
# Service should be 'inactive', not 'failed' after reboot though.
"$SYSTEMCTL_EXEC" reset-failed 'systemd-coredump.service' || true

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
