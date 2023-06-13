#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 85/364: 'package_tmux_installed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "tmux" ; then
    dnf install -y "tmux"
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
