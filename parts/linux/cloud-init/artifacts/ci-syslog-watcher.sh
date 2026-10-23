#!/usr/bin/env bash

set -o nounset
set -o pipefail

[ ! -f "/var/run/mdsd-ci/update.status" ] && exit 0
status=$(cat /var/run/mdsd-ci/update.status)

if [ "$status" = "add" ]; then
        echo "Status changed to $status."
        # By design: the rsyslog config dropped here is not content-validated before being
        # copied into /etc/rsyslog.d and loaded via `systemctl restart rsyslog` below. The
        # trust boundary for this script is the /var/run/mdsd-ci directory itself, which is
        # populated by the privileged Container Insights (mdsd/AMA) add-on via a hostPath
        # mount - a component deployed and access-controlled outside this repo. Anything
        # capable of writing into that directory already has node-level privilege equivalent
        # to what this script could additionally restrict, so adding content validation here
        # would not meaningfully reduce risk; the real control point is that add-on's
        # hostPath/RBAC configuration, not this script.
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
