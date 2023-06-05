#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 320/364: 'package_sendmail_removed'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

# CAUTION: This remediation script will remove sendmail
#	   from the system, and may remove any packages
#	   that depend on sendmail. Execute this
#	   remediation AFTER testing on a non-production
#	   system!

if rpm -q --quiet "sendmail" ; then

    dnf remove -y "sendmail"

fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
