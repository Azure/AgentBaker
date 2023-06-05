#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 90/364: 'package_opensc_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "opensc" ; then
    dnf install -y "opensc"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
