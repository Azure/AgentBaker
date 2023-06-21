#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 120/364: 'package_audit_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "audit" ; then
    dnf install -y "audit"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
