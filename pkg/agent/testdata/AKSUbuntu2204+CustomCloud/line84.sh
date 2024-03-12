#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates
certs=$(curl "http://168.63.129.16/machine?comp=acmspackage&type=cacertificates&ext=json")
IFS_backup=$IFS
IFS=$'\r\n'
certNames=($(echo $certs | grep -oP '(?<=Name\": \")[^\"]*'))
certBodies=($(echo $certs | grep -oP '(?<=CertBody\": \")[^\"]*'))
for i in ${!certBodies[@]}; do
    echo ${certBodies[$i]}  | sed 's/\\r\\n/\n/g' | sed 's/\\//g' > "/root/AzureCACertificates/$(echo ${certNames[$i]} | sed 's/.cer/.crt/g')"
done
IFS=$IFS_backup

cp /root/AzureCACertificates/*.crt /usr/local/share/ca-certificates/
/usr/sbin/update-ca-certificates

cp /etc/ssl/certs/ca-certificates.crt /usr/lib/ssl/cert.pem

action=${1:-init}
if [ $action == "ca-refresh" ]
then
    exit
fi

(crontab -l ; echo "0 19 * * * $0 ca-refresh") | crontab -

cloud-init status --wait
repoDepotEndpoint="${REPO_DEPOT_ENDPOINT}"
sudo sed -i "s,http://.[^ ]*,$repoDepotEndpoint,g" /etc/apt/sources.list

systemctl stop systemd-timesyncd
systemctl disable systemd-timesyncd

chrony_conf="/etc/chrony/chrony.conf"
if [ ! -e "$chrony_conf" ]; then
    apt-get update
    apt-get install chrony -y
fi

cat > $chrony_conf <<EOF

#
#pool ntp.ubuntu.com        iburst maxsources 4
#pool 0.ubuntu.pool.ntp.org iburst maxsources 1
#pool 1.ubuntu.pool.ntp.org iburst maxsources 1
#pool 2.ubuntu.pool.ntp.org iburst maxsources 2

keyfile /etc/chrony/chrony.keys

driftfile /var/lib/chrony/chrony.drift

#log tracking measurements statistics

logdir /var/log/chrony

maxupdateskew 100.0

rtcsync

refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

systemctl restart chrony

#EOF