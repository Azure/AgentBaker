#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 41/364: 'package_rng-tools_installed'")

if ! rpm -q --quiet "rng-tools" ; then
    dnf install -y "rng-tools"
fi
