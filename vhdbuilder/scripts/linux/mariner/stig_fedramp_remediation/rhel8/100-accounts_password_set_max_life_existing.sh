#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 100/364: 'accounts_password_set_max_life_existing'")

SECURE_MAX_PASS_AGE=60

usrs_max_pass_age=( $(awk -F: -v val="$SECURE_MAX_PASS_AGE" '$5 > val || $5 == "" {print $1}' /etc/shadow) )
for i in ${usrs_max_pass_age[@]};
do
  passwd -x $SECURE_MAX_PASS_AGE $i
done