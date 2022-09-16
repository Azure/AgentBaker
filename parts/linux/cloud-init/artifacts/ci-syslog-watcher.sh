#!/usr/bin/env bash

set -o nounset
set -o pipefail

[ ! -f "/var/run/mdsd/update.status" ] && exit 0
status=$(cat /var/run/mdsd/update.status)

if [[ "$status" == "add" ]]; then
        echo "Status changed to $status."
        cat >/etc/rsyslog.d/60-rsyslog-forward-mdsd.conf << 'EOL'
$ModLoad omuxsock 
$OMUxSockSocket /var/run/mdsd/default_syslog.socket 
template(name="MDSD_RSYSLOG_TraditionalForwardFormat" type="string" string="<%PRI%>%TIMESTAMP% %HOSTNAME% %syslogtag%%msg:::sp-if-no-1st-sp%%msg%")
$OMUxSockDefaultTemplate MDSD_RSYSLOG_TraditionalForwardFormat
*.* :omuxsock:
EOL
elif [[ "$status" == "remove" ]]; then
        echo "Status changed to $status."
        [ -f "/etc/rsyslog.d/60-rsyslog-forward-mdsd.conf" ] && rm /etc/rsyslog.d/60-rsyslog-forward-mdsd.conf
else
        echo "Unexpected status change to $status. Exiting"
        exit 1
fi

echo "Restarting rsyslogd"
service rsyslog restart

exit 0