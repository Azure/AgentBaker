#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 218/364: 'rsyslog_encrypt_offload_defaultnetstreamdriver'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

#!/bin/bash
if [ -e "/etc/rsyslog.d/encrypt.conf" ] ; then
    
    LC_ALL=C sed -i "/^\s*\\$DefaultNetstreamDriver /Id" "/etc/rsyslog.d/encrypt.conf"
else
    touch "/etc/rsyslog.d/encrypt.conf"
fi
cp "/etc/rsyslog.d/encrypt.conf" "/etc/rsyslog.d/encrypt.conf.bak"
# Insert at the end of the file
printf '%s\n' "\$DefaultNetstreamDriver gtls" >> "/etc/rsyslog.d/encrypt.conf"
# Clean up after ourselves.
rm "/etc/rsyslog.d/encrypt.conf.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
