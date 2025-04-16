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

cp /root/AzureCACertificates/*.crt /etc/pki/ca-trust/source/anchors/
/usr/bin/update-ca-trust

cloud-init status --wait

marinerRepoDepotEndpoint="$(echo "${REPO_DEPOT_ENDPOINT}" | sed 's/\/ubuntu//')"
if [ "$marinerRepoDepotEndpoint" = "" ]; then
  >&2 echo "repo depot endpoint empty while running custom-cloud init script"
else
  for f in /etc/yum.repos.d/*.repo
  do
      sed -i -e "s|https://packages.microsoft.com|${marinerRepoDepotEndpoint}/mariner/packages.microsoft.com|" "$f"
      echo "## REPO - $f - MODIFIED"
  done
fi

cat > /etc/chrony.conf <<EOF
keyfile /etc/chrony.keys

driftfile /var/lib/chrony/drift

#log tracking measurements statistics

logdir /var/log/chrony

maxupdateskew 100.0

rtcsync

refclock PHC /dev/ptp0 poll 3 dpoll -2 offset 0
makestep 1.0 -1
EOF

systemctl restart chronyd

#EOF