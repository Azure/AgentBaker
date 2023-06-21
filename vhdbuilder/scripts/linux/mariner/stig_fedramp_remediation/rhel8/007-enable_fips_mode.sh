#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 7/364: 'enable_fips_mode'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

fips-mode-setup --enable

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
