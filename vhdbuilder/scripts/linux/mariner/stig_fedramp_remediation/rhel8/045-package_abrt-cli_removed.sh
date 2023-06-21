#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 45/364: 'package_abrt-cli_removed'")

# CAUTION: This remediation script will remove abrt-cli
#	   from the system, and may remove any packages
#	   that depend on abrt-cli. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "abrt-cli" ; then

    dnf remove -y "abrt-cli"

fi
