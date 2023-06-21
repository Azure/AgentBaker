#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 354/364: 'sshd_use_strong_rng'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if [ -e "/etc/sysconfig/sshd" ] ; then
    
    LC_ALL=C sed -i "/^\s*SSH_USE_STRONG_RNG=/d" "/etc/sysconfig/sshd"
else
    touch "/etc/sysconfig/sshd"
fi
cp "/etc/sysconfig/sshd" "/etc/sysconfig/sshd.bak"
# Insert before the line matching the regex '^#\s*SSH_USE_STRONG_RNG'.
line_number="$(LC_ALL=C grep -n "^#\s*SSH_USE_STRONG_RNG" "/etc/sysconfig/sshd.bak" | LC_ALL=C sed 's/:.*//g')"
if [ -z "$line_number" ]; then
    # There was no match of '^#\s*SSH_USE_STRONG_RNG', insert at
    # the end of the file.
    printf '%s\n' "SSH_USE_STRONG_RNG=32" >> "/etc/sysconfig/sshd"
else
    head -n "$(( line_number - 1 ))" "/etc/sysconfig/sshd.bak" > "/etc/sysconfig/sshd"
    printf '%s\n' "SSH_USE_STRONG_RNG=32" >> "/etc/sysconfig/sshd"
    tail -n "+$(( line_number ))" "/etc/sysconfig/sshd.bak" >> "/etc/sysconfig/sshd"
fi
# Clean up after ourselves.
rm "/etc/sysconfig/sshd.bak"

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
