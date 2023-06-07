#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 212/364: 'package_rsyslog-gnutls_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "rsyslog-gnutls" ; then
    dnf install -y "rsyslog-gnutls"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
