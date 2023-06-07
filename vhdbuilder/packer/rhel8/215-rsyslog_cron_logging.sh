#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 215/364: 'rsyslog_cron_logging'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then

if ! grep -s "^\s*cron\.\*\s*/var/log/cron$" /etc/rsyslog.conf /etc/rsyslog.d/*.conf; then
	mkdir -p /etc/rsyslog.d
	echo "cron.*	/var/log/cron" >> /etc/rsyslog.d/cron.conf
fi


systemctl restart rsyslog.service

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
