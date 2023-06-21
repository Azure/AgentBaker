#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 250/364: 'file_permissions_etc_audit_auditd'")


chmod 0640 /etc/audit/auditd.conf
