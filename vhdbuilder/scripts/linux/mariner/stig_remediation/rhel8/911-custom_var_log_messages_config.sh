#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule: 'custom_var_log_messages_config'")

sed -i 's/authpriv\.\*\s*\/var\/log\/messages/\
if prifilt("authpriv.*") then {\
    action(type="omfile" file="\/var\/log\/messages" FileCreateMode="0640" fileGroupNum="0" fileOwnerNum="0")\
}/g' /etc/rsyslog.conf
