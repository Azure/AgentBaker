#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 332/364: 'package_telnet-server_removed'")

# CAUTION: This remediation script will remove telnet-server
#	   from the system, and may remove any packages
#	   that depend on telnet-server. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "telnet-server" ; then

    dnf remove -y "telnet-server"

fi
