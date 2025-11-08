#! /bin/bash

set -uo pipefail
set -x
set -e

NFTABLES_RULESET_FILE=/etc/systemd/system/ipv6_nftables

# query IMDS to check if node has IPv6
# example interface
# [
#   {
#     "ipv4": {
#       "ipAddress": [
#         {
#           "privateIpAddress": "10.224.0.4",
#           "publicIpAddress": ""
#         }
#       ],
#       "subnet": [
#         {
#           "address": "10.224.0.0",
#           "prefix": "16"
#         }
#       ]
#     },
#     "ipv6": {
#       "ipAddress": [
#         {
#           "privateIpAddress": "fd85:534e:4cd6:ab02::5"
#         }
#       ]
#     },
#     "macAddress": "000D3A98DA20"
#   }
# ]

# check the number of IPv6 addresses this instance has from IMDS
IPV6_ADDR_COUNT=$(curl -sSL -H "Metadata: true" "http://169.254.169.254/metadata/instance/network/interface?api-version=2021-02-01" | \
    jq '[.[].ipv6.ipAddress[] | select(.privateIpAddress != "")] | length')

if [ "$IPV6_ADDR_COUNT" -eq 0 ];
then
    echo "instance is not configured with IPv6, skipping nftables rules"
else
    echo "writing nftables from $NFTABLES_RULESET_FILE"
    nft -f $NFTABLES_RULESET_FILE
fi
