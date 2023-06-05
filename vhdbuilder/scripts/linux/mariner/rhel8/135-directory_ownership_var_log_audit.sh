#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 135/364: 'directory_ownership_var_log_audit'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then

chown root /var/log/audit

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
