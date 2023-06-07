#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 4/364: 'aide_verify_acls'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! rpm -q --quiet "aide" ; then
    dnf install -y "aide"
fi

aide_conf="/etc/aide.conf"

groups=$(LC_ALL=C grep "^[A-Z][A-Za-z_]*" $aide_conf | grep -v "^ALLXTRAHASHES" | cut -f1 -d '=' | tr -d ' ' | sort -u)

for group in $groups
do
	config=$(grep "^$group\s*=" $aide_conf | cut -f2 -d '=' | tr -d ' ')

	if ! [[ $config = *acl* ]]
	then
		if [[ -z $config ]]
		then
			config="acl"
		else
			config=$config"+acl"
		fi
	fi
	sed -i "s/^$group\s*=.*/$group = $config/g" $aide_conf
done

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
