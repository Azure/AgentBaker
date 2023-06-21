#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 50/364: 'package_iprutils_removed'")

# CAUTION: This remediation script will remove iprutils
#	   from the system, and may remove any packages
#	   that depend on iprutils. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "iprutils" ; then

    dnf remove -y "iprutils"

fi
