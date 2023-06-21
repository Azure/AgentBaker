#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 1/364: 'package_aide_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "aide" ; then
    dnf install -y "aide"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
