#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 118/364: 'accounts_umask_etc_profile'")

var_accounts_user_umask="077"



grep -q umask /etc/profile && \
  sed -i "s/umask.*/umask $var_accounts_user_umask/g" /etc/profile
if ! [ $? -eq 0 ]; then
    echo "umask $var_accounts_user_umask" >> /etc/profile
fi
