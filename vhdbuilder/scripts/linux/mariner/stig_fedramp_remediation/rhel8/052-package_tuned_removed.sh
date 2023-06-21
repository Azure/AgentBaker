#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 52/364: 'package_tuned_removed'")

# CAUTION: This remediation script will remove tuned
#	   from the system, and may remove any packages
#	   that depend on tuned. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "tuned" ; then

    dnf remove -y "tuned"

fi
