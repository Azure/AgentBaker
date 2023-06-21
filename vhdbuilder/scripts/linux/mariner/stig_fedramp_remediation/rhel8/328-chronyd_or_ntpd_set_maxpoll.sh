#!/usr/bin/env bash
set -e
(>&2 echo "Remediating rule 328/364: 'chronyd_or_ntpd_set_maxpoll'")
# Remediation is applicable only in certain platforms
if [ ! -f /.dockerenv ] && [ ! -f /run/.containerenv ] && { rpm --quiet -q chrony || rpm --quiet -q ntp; }; then


var_time_service_set_maxpoll="16"




config_file="/etc/ntp.conf"
/usr/sbin/pidof ntpd || config_file="/etc/chrony.conf"


# Set maxpoll values to var_time_service_set_maxpoll
sed -i "s/^\(server.*maxpoll\) [0-9][0-9]*\(.*\)$/\1 $var_time_service_set_maxpoll \2/" "$config_file"

# Add maxpoll to server entries without maxpoll
grep "^server" "$config_file" | grep -v maxpoll | while read -r line ; do
        sed -i "s/$line/& maxpoll $var_time_service_set_maxpoll/" "$config_file"
done

else
    >&2 echo 'Remediation is not applicable, nothing was done'
fi
