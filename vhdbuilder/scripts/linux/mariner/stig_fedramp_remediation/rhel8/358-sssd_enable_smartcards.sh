#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 358/364: 'sssd_enable_smartcards'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q sssd-common && { [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; }; then

SSSD_CONF="/etc/sssd/sssd.conf"
SSSD_OPT="pam_cert_auth"
SSSD_OPT_VAL=true
PAM_REGEX="[[:space:]]*\[pam]"
PAM_OPT_REGEX="${PAM_REGEX}([^\n\[]*\n+)+?[[:space:]]*${SSSD_OPT}"

if grep -qzosP $PAM_OPT_REGEX $SSSD_CONF; then
	sed -i "s/${SSSD_OPT}[^(\n)]*/${SSSD_OPT} = ${SSSD_OPT_VAL}/" $SSSD_CONF
elif grep -qs $PAM_REGEX $SSSD_CONF; then
	sed -i "/$PAM_REGEX/a ${SSSD_OPT} = ${SSSD_OPT_VAL}" $SSSD_CONF
else
	mkdir -p /etc/sssd
	touch $SSSD_CONF
	echo -e "[pam]\n${SSSD_OPT} = ${SSSD_OPT_VAL}" >> $SSSD_CONF
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
