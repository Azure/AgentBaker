#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 101/364: 'accounts_password_set_min_life_existing'")

SECURE_MIN_PASS_AGE=1

usrs_min_pass_age=( $(awk -F: -v val="$SECURE_MIN_PASS_AGE" '$4 < val || $4 == "" {print $1}' /etc/shadow) )
for i in ${usrs_min_pass_age[@]};
do
  passwd -n $SECURE_MIN_PASS_AGE $i
done
