#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 251/364: 'file_permissions_etc_audit_rulesd'")


readarray -t files < <(find /etc/audit/rules.d/)
for file in "${files[@]}"; do
    if basename $file | grep -q '^.*rules$'; then
        chmod 0640 $file
    fi    
done
