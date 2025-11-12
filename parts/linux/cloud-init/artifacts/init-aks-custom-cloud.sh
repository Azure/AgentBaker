#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates

# For Flatcar: systemd timer instead of cron, skip cloud-init/apt ops, chronyd service name).
IS_FLATCAR=0
if [ -f /etc/os-release ] && grep -qi '^ID=flatcar' /etc/os-release; then
  IS_FLATCAR=1
fi

# http://168.63.129.16 is a constant for the host's wireserver endpoint
certs=$(curl "http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json")
IFS_backup=$IFS
IFS=$'\r\n'
certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
ext=".crt"
if [ "$IS_FLATCAR" -eq 1 ]; then
    ext=".pem"
fi
for i in ${!certBodies[@]}; do
    echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "/root/AzureCACertificates/$(echo ${certNames[$i]} | sed "s/.cer/.${ext}/g")"
done
IFS=$IFS_backup

if [ "$IS_FLATCAR" -eq 0 ]; then
    cp /root/AzureCACertificates/*.crt /usr/local/share/ca-certificates/
    update-ca-certificates

    # This copies the updated bundle to the location used by OpenSSL which is commonly used
    cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem
else
    cp /root/AzureCACertificates/*.pem /etc/ssl/certs/
    update-ca-certificates
fi

# This section creates a cron job to poll for refreshed CA certs daily
# It can be removed if not needed or desired
action=${1:-init}
if [ "$action" = "ca-refresh" ]; then
    exit
fi

if [ "$IS_FLATCAR" -eq 0 ]; then
    (crontab -l ; echo "0 19 * * * $0 ca-refresh") | crontab -

    cloud-init status --wait
    repoDepotEndpoint="${REPO_DEPOT_ENDPOINT}"
    sudo sed -i "s,http://.[^ ]*,$repoDepotEndpoint,g" /etc/apt/sources.list
else
    script_path="$(readlink -f "$0")"
    svc="/etc/systemd/system/azure-ca-refresh.service"
    tmr="/etc/systemd/system/azure-ca-refresh.timer"
    cat >"$svc" <<EOF
[Unit]
Description=Refresh Azure Custom Cloud CA certificates
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=$script_path ca-refresh
EOF
    cat >"$tmr" <<EOF
[Unit]
Description=Daily refresh of Azure Custom Cloud CA certificates

[Timer]
OnCalendar=19:00
Persistent=true
RandomizedDelaySec=300

[Install]
WantedBy=timers.target
EOF
    systemctl daemon-reload
    systemctl enable --now azure-ca-refresh.timer
fi

# Disable systemd-timesyncd and install chrony and uses local time source
chrony_conf="/etc/chrony/chrony.conf"
if [ "$IS_FLATCAR" -eq 0 ]; then
    systemctl stop systemd-timesyncd
    systemctl disable systemd-timesyncd

    if [ ! -e "$chrony_conf" ]; then
        apt-get update
        apt-get install chrony -y
    fi
else
    rm -f ${chrony_conf}
fi

cat > $chrony_conf <<EOF
# Welcome to the chrony configuration file. See chrony.conf(5) for more
# information about usuable directives.

# This will use (up to):
# - 4 sources from ntp.ubuntu.com which some are ipv6 enabled
# - 2 sources from 2.ubuntu.pool.ntp.org which is ipv6 enabled as well
# - 1 source from [01].ubuntu.pool.ntp.org each (ipv4 only atm)
# This means by default, up to 6 dual-stack and up to 2 additional IPv4-only
# sources will be used.
# At the same time it retains some protection against one of the entries being
# down (compare to just using one of the lines). See (LP: #1754358) for the
# discussion.
#
# About using servers from the NTP Pool Project in general see (LP: #104525).
# Approved by Ubuntu Technical Board on 2011-02-08.
# See http://www.pool.ntp.org/join.html for more information.
#pool ntp.ubuntu.com        iburst maxsources 4
#pool 0.ubuntu.pool.ntp.org iburst maxsources 1
#pool 1.ubuntu.pool.ntp.org iburst maxsources 1
#pool 2.ubuntu.pool.ntp.org iburst maxsources 2

# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/chrony.drift

# Uncomment the following line to turn logging on.
#log tracking measurements statistics

# Log files location.
logdir /var/log/chrony

# Stop bad estimates upsetting machine clock.
maxupdateskew 100.0

# This directive enables kernel synchronisation (every 11 minutes) of the
# real-time clock. Note that it can’t be used along with the 'rtcfile' directive.
rtcsync

# Settings come from: https://docs.microsoft.com/en-us/azure/virtual-machines/linux/time-sync
refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

if [ "$IS_FLATCAR" -eq 0 ]; then
    systemctl restart chrony
else
    systemctl restart chronyd
fi

#EOF