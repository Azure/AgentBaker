#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 269/364: 'service_autofs_disabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" stop 'autofs.service'
"$SYSTEMCTL_EXEC" disable 'autofs.service'
"$SYSTEMCTL_EXEC" mask 'autofs.service'
# Disable socket activation if we have a unit file for it
if "$SYSTEMCTL_EXEC" list-unit-files | grep -q '^autofs.socket'; then
    "$SYSTEMCTL_EXEC" stop 'autofs.socket'
    "$SYSTEMCTL_EXEC" mask 'autofs.socket'
fi
# The service may not be running because it has been started and failed,
# so let's reset the state so OVAL checks pass.
# Service should be 'inactive', not 'failed' after reboot though.
"$SYSTEMCTL_EXEC" reset-failed 'autofs.service' || true

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
