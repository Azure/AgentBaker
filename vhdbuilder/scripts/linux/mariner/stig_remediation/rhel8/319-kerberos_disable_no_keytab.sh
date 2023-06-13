#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 319/364: 'kerberos_disable_no_keytab'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

rm -f /etc/*.keytab

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
