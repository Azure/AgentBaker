#!/usr/bin/env bash

set -o nounset
set -o pipefail

[ ! -f "/var/run/mdsd-ci/update.status" ] && exit 0
status=$(cat /var/run/mdsd-ci/update.status)

if [ "$status" = "add" ]; then
        echo "Status changed to $status."
        [ -f "/var/run/mdsd-ci/70-rsyslog-forward-mdsd-ci.conf" ] && cp /var/run/mdsd-ci/70-rsyslog-forward-mdsd-ci.conf /etc/rsyslog.d
elif [ "$status" = "remove" ]; then
        echo "Status changed to $status."
        [ -f "/etc/rsyslog.d/70-rsyslog-forward-mdsd-ci.conf" ] && rm /etc/rsyslog.d/70-rsyslog-forward-mdsd-ci.conf
else
        echo "Unexpected status change to $status. Exiting"
        exit 1
fi

echo "Restarting rsyslog"
systemctl restart rsyslog

exit 0
