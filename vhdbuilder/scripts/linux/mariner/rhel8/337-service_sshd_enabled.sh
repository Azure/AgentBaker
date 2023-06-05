#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 337/364: 'service_sshd_enabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'sshd.service'
"$SYSTEMCTL_EXEC" start 'sshd.service'
"$SYSTEMCTL_EXEC" enable 'sshd.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
