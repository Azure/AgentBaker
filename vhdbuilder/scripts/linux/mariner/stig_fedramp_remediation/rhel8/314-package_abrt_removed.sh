#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 314/364: 'package_abrt_removed'")

# CAUTION: This remediation script will remove abrt
#	   from the system, and may remove any packages
#	   that depend on abrt. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "abrt" ; then

    dnf remove -y "abrt"

fi
