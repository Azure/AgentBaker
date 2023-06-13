#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 224/364: 'service_firewalld_enabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'firewalld.service'
"$SYSTEMCTL_EXEC" start 'firewalld.service'
"$SYSTEMCTL_EXEC" enable 'firewalld.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
