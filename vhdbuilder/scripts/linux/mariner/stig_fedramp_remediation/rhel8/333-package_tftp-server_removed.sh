#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 333/364: 'package_tftp-server_removed'")

# CAUTION: This remediation script will remove tftp-server
#	   from the system, and may remove any packages
#	   that depend on tftp-server. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "tftp-server" ; then

    dnf remove -y "tftp-server"

fi
