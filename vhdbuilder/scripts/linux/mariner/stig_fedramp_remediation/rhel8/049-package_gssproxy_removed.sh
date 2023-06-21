#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 49/364: 'package_gssproxy_removed'")

# CAUTION: This remediation script will remove gssproxy
#	   from the system, and may remove any packages
#	   that depend on gssproxy. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "gssproxy" ; then

    dnf remove -y "gssproxy"

fi
