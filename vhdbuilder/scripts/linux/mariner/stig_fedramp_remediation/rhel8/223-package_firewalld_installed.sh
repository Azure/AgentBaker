#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 223/364: 'package_firewalld_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "firewalld" ; then
    dnf install -y "firewalld"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
