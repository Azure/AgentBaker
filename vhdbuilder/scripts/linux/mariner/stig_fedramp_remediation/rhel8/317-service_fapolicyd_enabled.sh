#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 317/364: 'service_fapolicyd_enabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'fapolicyd.service'
"$SYSTEMCTL_EXEC" start 'fapolicyd.service'
"$SYSTEMCTL_EXEC" enable 'fapolicyd.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
