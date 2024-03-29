#!/bin/sh
# This script was originally generated by logrotate automatically and placed in /etc/cron.daily/logrotate
# This will be saved on the target VM within /usr/local/bin/logrotate.sh and invoked by logrotate.service

# Clean non existent log file entries from status file
cd /var/lib/logrotate
test -e status || touch status
head -1 status > status.clean
sed 's/"//g' status | while read logfile date
do
    [ -e "$logfile" ] && echo "\"$logfile\" $date"
done >> status.clean
mv status.clean status

test -x /usr/sbin/logrotate || exit 0
/usr/sbin/logrotate --verbose /etc/logrotate.conf