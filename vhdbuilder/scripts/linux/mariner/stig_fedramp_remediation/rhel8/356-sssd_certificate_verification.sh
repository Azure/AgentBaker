#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 356/364: 'sssd_certificate_verification'")
# Remediation is applicable only in certain platforms
if rpm --quiet -q sssd-common; then

# include our remediation functions library

var_sssd_certificate_verification_digest_function="sha1"



found=false
for f in /etc/sssd/sssd.conf /etc/sssd/conf.d/*.conf; do
	if [ ! -e "$f" ]; then
		continue
	fi
	cert=$( awk '/^\s*\[/{f=0} /^\s*\[sssd\]/{f=1} f{nu=gensub("^\\s*certificate_verification\\s*=\\s*ocsp_dgst\\s*=\\s*(\\w+).*","\\1",1); if($0!=nu){cert=nu}} END{print cert}' "$f" )
	if [ -n "$cert" ] ; then
		if [ "$cert" != $var_sssd_certificate_verification_digest_function ] ; then
			sed -i "s/^certificate_verification\s*=.*/certificate_verification = ocsp_dgst = $var_sssd_certificate_verification_digest_function/" "$f"
		fi
		found=true
	fi
done

if ! $found ; then
	SSSD_CONF="/etc/sssd/conf.d/certificate_verification.conf"
	mkdir -p $( dirname $SSSD_CONF )
	touch $SSSD_CONF
	chown root:root $SSSD_CONF
	chmod 600 $SSSD_CONF
	echo -e "[sssd]\ncertificate_verification = ocsp_dgst = $var_sssd_certificate_verification_digest_function" >> $SSSD_CONF
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
