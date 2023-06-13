#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 322/364: 'postfix_prevent_unrestricted_relay'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q postfix; then

if ! grep -q ^smtpd_client_restrictions /etc/postfix/main.cf; then
	echo "smtpd_client_restrictions = permit_mynetworks,reject" >> /etc/postfix/main.cf
else
	sed -i "s/^smtpd_client_restrictions.*/smtpd_client_restrictions = permit_mynetworks,reject/g" /etc/postfix/main.cf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
