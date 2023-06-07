#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 42/364: 'package_abrt-addon-ccpp_removed'")

# CAUTION: This remediation script will remove abrt-addon-ccpp
#	   from the system, and may remove any packages
#	   that depend on abrt-addon-ccpp. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "abrt-addon-ccpp" ; then

    dnf remove -y "abrt-addon-ccpp"

fi
