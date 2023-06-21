#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 213/364: 'package_rsyslog_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "rsyslog" ; then
    dnf install -y "rsyslog"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
