#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 43/364: 'package_abrt-addon-kerneloops_removed'")

# CAUTION: This remediation script will remove abrt-addon-kerneloops
#	   from the system, and may remove any packages
#	   that depend on abrt-addon-kerneloops. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "abrt-addon-kerneloops" ; then

    dnf remove -y "abrt-addon-kerneloops"

fi
