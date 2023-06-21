#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 318/364: 'package_vsftpd_removed'")

# CAUTION: This remediation script will remove vsftpd
#	   from the system, and may remove any packages
#	   that depend on vsftpd. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "vsftpd" ; then

    dnf remove -y "vsftpd"

fi
