#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 329/364: 'package_rsh-server_removed'")

# CAUTION: This remediation script will remove rsh-server
#	   from the system, and may remove any packages
#	   that depend on rsh-server. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "rsh-server" ; then

    dnf remove -y "rsh-server"

fi
