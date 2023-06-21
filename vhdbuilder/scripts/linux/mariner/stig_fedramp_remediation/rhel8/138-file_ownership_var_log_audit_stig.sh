#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 138/364: 'file_ownership_var_log_audit_stig'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then

chown root /var/log/audit/audit.log*

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
