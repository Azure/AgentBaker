#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 335/364: 'service_rngd_enabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'rngd.service'
"$SYSTEMCTL_EXEC" start 'rngd.service'
"$SYSTEMCTL_EXEC" enable 'rngd.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
