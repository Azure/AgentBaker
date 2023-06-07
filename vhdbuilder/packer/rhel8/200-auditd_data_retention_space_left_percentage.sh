#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 200/364: 'auditd_data_retention_space_left_percentage'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then


var_auditd_space_left_percentage="25"



grep -q "^space_left[[:space:]]*=.*$" /etc/audit/auditd.conf && \
  sed -i "s/^space_left[[:space:]]*=.*$/space_left = $var_auditd_space_left_percentage%/g" /etc/audit/auditd.conf || \
  echo "space_left = $var_auditd_space_left_percentage%" >> /etc/audit/auditd.conf

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
