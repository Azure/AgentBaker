#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 247/364: 'wireless_disable_interfaces'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

nmcli radio wifi off

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
