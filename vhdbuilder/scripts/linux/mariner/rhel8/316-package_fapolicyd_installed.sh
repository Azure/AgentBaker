#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 316/364: 'package_fapolicyd_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "fapolicyd" ; then
    dnf install -y "fapolicyd"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
