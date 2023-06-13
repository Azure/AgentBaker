#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 268/274: 'sshd_use_approved_macs_ordered_stig'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if grep -q -P '^\s*MACs\s+' /etc/ssh/sshd_config; then
  sed -i 's/^\s*MACs.*/MACs hmac-sha2-512,hmac-sha2-256/' /etc/ssh/sshd_config
else
  echo "MACs hmac-sha2-512,hmac-sha2-256" >> /etc/ssh/sshd_config
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
