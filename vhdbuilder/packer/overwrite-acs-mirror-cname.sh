#!/bin/bash
apt update
apt install -y bind9 bind9utils bind9-doc
sed -i s/"-u bind"/"-u bind -4"/g /etc/default/named

cat <<EOF > /etc/bind/named.conf.options
options {
	      directory "/var/cache/bind";
        recursion yes;
        version "not currently available";
        allow-recursion { 127.0.0.1; 192.168.1.0/24; }; 
        response-policy { zone "rpz"; };
	      dnssec-validation auto;
};
EOF

cat <<EOF > /etc/bind/db.rpz
\$TTL 60
@            IN    SOA  localhost. root.localhost.  (
                          2015112501   ; serial
                          1h           ; refresh
                          30m          ; retry
                          1w           ; expiry
                          30m)         ; minimum
                   IN     NS    localhost.

localhost       A   127.0.0.1

acs-mirror.azureedge.net   CNAME    acs-mirror-traffic-manager-stg.trafficmanager.net.
EOF

cat <<EOF > /etc/bind/named.conf.local
zone "rpz" {
  type master;
  file "/etc/bind/db.rpz";
};
EOF

systemctl restart named
systemctl enable named

#####
# OVERWRITE LOCAL DNS
####

sed -i s/#DNS=/DNS=127.0.0.1/g /etc/systemd/resolved.conf 
service systemd-resolved restart
ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf