#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 214/364: 'service_rsyslog_enabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'rsyslog.service'
"$SYSTEMCTL_EXEC" start 'rsyslog.service'
"$SYSTEMCTL_EXEC" enable 'rsyslog.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
