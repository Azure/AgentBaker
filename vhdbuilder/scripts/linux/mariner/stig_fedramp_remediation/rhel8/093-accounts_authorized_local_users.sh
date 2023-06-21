#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 93/364: 'accounts_authorized_local_users'")

var_accounts_authorized_local_users_regex="^(mariner_user|bin|chrony|daemon|messagebus|nobody|sshd|systemd-bus-proxy|systemd-journal-gateway|systemd-journal-remote|systemd-journal-upload|systemd-network|systemd-resolve|systemd-timesync)$"



# never delete the root user
default_os_user="^root$"

# delete users that is in /etc/passwd but neither in default_os_user
# nor in var_accounts_authorized_local_users_regex
for username in $( sed 's/:.*//' /etc/passwd ) ; do
	if [[ ! "$username" =~ ($default_os_user|$var_accounts_authorized_local_users_regex) ]];
        then
		userdel $username ;
	fi
done
