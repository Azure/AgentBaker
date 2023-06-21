#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 361/364: 'service_usbguard_enabled'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

SYSTEMCTL_EXEC='/usr/bin/systemctl'
"$SYSTEMCTL_EXEC" unmask 'usbguard.service'
"$SYSTEMCTL_EXEC" start 'usbguard.service'
"$SYSTEMCTL_EXEC" enable 'usbguard.service'

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
