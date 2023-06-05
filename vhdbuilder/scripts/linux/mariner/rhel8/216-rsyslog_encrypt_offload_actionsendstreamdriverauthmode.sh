#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 216/364: 'rsyslog_encrypt_offload_actionsendstreamdriverauthmode'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! grep -s "\$ActionSendStreamDriverAuthMode\s*x509/name" /etc/rsyslog.conf /etc/rsyslog.d/*.conf; then
	mkdir -p /etc/rsyslog.d
    sed -i '/^.*\$ActionSendStreamDriverAuthMode.*/d' /etc/rsyslog.conf /etc/rsyslog.d/*.conf
    echo "\$ActionSendStreamDriverAuthMode x509/name" > /etc/rsyslog.d/stream_driver_auth.conf
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
