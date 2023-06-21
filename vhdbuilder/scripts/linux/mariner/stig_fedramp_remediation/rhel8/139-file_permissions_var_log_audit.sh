#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 139/364: 'file_permissions_var_log_audit'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q audit; then

if LC_ALL=C grep -m 1 -q ^log_group /etc/audit/auditd.conf; then
  GROUP=$(awk -F "=" '/log_group/ {print $2}' /etc/audit/auditd.conf | tr -d ' ')
  if ! [ "${GROUP}" == 'root' ] ; then
    chmod 0640 /var/log/audit/audit.log
    if ls /var/log/audit/audit.log.* 1> /dev/null 2>&1; then
      # Rotated logs may not exist, this errors if they are missing
      chmod 0440 /var/log/audit/audit.log.*
    fi
  else
    chmod 0600 /var/log/audit/audit.log
    if ls /var/log/audit/audit.log.* 1> /dev/null 2>&1; then
      # Rotated logs may not exist, this errors if they are missing
      chmod 0400 /var/log/audit/audit.log.*
    fi
  fi
else
  chmod 0600 /var/log/audit/audit.log
  if ls /var/log/audit/audit.log.* 1> /dev/null 2>&1; then
    # Rotated logs may not exist, this errors if they are missing
    chmod 0400 /var/log/audit/audit.log.*
  fi
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
