#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 136/364: 'directory_permissions_var_log_audit'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then

if LC_ALL=C grep -m 1 -q ^log_group /etc/audit/auditd.conf; then
  GROUP=$(awk -F "=" '/log_group/ {print $2}' /etc/audit/auditd.conf | tr -d ' ')
  if ! [ "${GROUP}" == 'root' ] ; then
    chmod 0750 /var/log/audit
  else
    chmod 0700 /var/log/audit
  fi
else
  chmod 0700 /var/log/audit
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
