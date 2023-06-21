#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 311/364: 'package_policycoreutils_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "policycoreutils" ; then
    dnf install -y "policycoreutils"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
