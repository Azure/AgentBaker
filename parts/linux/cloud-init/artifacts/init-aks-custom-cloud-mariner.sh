#!/bin/bash
set -x
mkdir -p /root/AzureCACertificates
# http://168.63.129.16 is a constant for the host's wireserver endpoint
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
if [ -z "$marinerRepoDepotEndpoint" ]; then
  >&2 echo "repo depot endpoint empty while running custom-cloud init script"
else
  for f in /etc/yum.repos.d/*.repo
  do
      sed -i -e "s|https://packages.microsoft.com|${marinerRepoDepotEndpoint}/mariner/packages.microsoft.com|" "$f"
      echo "## REPO - $f - MODIFIED"
  done
fi

# Set the chrony config to use the PHC /dev/ptp0 clock
cat > /etc/chrony.conf <<EOF
# This directive specify the location of the file containing ID/key pairs for
# NTP authentication.
keyfile /etc/chrony.keys

# This directive specify the file into which chronyd will store the rate
# information.
driftfile /var/lib/chrony/drift

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

systemctl restart chronyd

#EOF