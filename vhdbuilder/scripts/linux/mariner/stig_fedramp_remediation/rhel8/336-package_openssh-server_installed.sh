#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 336/364: 'package_openssh-server_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "openssh-server" ; then
    dnf install -y "openssh-server"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
