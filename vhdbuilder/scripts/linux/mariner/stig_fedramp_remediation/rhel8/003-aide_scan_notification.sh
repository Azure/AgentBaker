#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 3/364: 'aide_scan_notification'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ]; then



if ! rpm -q --quiet "aide" ; then
    dnf install -y "aide"
fi
var_aide_scan_notification_email="root@localhost"



    




CRONTAB=/etc/crontab
CRONDIRS='/etc/cron.d /etc/cron.daily /etc/cron.weekly /etc/cron.monthly'

# NOTE: on some platforms, /etc/crontab may not exist
if [ -f /etc/crontab ]; then
	CRONTAB_EXIST=/etc/crontab
fi

if [ -f /var/spool/cron/root ]; then
	VARSPOOL=/var/spool/cron/root
fi

if ! grep -qR '^.*/usr/sbin/aide\s*\-\-check.*|.*\/bin\/mail\s*-s\s*".*"\s*.*@.*$' $CRONTAB_EXIST $VARSPOOL $CRONDIRS; then
	echo "0 5 * * * root /usr/sbin/aide  --check | /bin/mail -s \"\$(hostname) - AIDE Integrity Check\" $var_aide_scan_notification_email" >> $CRONTAB
fi

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
