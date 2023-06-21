#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 48/364: 'package_abrt-plugin-sosreport_removed'")

# CAUTION: This remediation script will remove abrt-plugin-sosreport
#	   from the system, and may remove any packages
#	   that depend on abrt-plugin-sosreport. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "abrt-plugin-sosreport" ; then

    dnf remove -y "abrt-plugin-sosreport"

fi
