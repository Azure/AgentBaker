#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 6/274: 'package_dracut-fips_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "dracut-fips" ; then
    dnf install -y "dracut-fips"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
