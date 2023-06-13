#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 46/364: 'package_abrt-plugin-logger_removed'")

# CAUTION: This remediation script will remove abrt-plugin-logger
#	   from the system, and may remove any packages
#	   that depend on abrt-plugin-logger. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "abrt-plugin-logger" ; then

    dnf remove -y "abrt-plugin-logger"

fi
