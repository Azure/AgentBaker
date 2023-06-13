#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 51/364: 'package_krb5-workstation_removed'")

# CAUTION: This remediation script will remove krb5-workstation
#	   from the system, and may remove any packages
#	   that depend on krb5-workstation. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "krb5-workstation" ; then

    dnf remove -y "krb5-workstation"

fi
