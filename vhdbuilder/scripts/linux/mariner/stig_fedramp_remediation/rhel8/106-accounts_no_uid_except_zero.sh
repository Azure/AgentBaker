#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 106/364: 'accounts_no_uid_except_zero'")
awk -F: '$3 == 0 && $1 != "root" { print $1 }' /etc/passwd | xargs --max-lines=1 --no-run-if-empty passwd -l
