#!/bin/bash

OK=0
NOTOK=1

if [ ! -f /run/systemd/resolve/resolv.conf ]; then
    # skip check for Ubuntu 16.04
    exit $OK
fi

out_1=$( udevadm info /sys/class/net/eth0 | grep ID_NET_DRIVER )

if [ -z "$out_1" ]; then
    echo "Node network state unhealthy"
    if ! udevadm trigger -c add -y eth0; then
        echo "udevadm trigger failed"
        exit $NOTOK
    fi
fi

out_2=$( grep -Ei '^nameserver' /run/systemd/resolve/resolv.conf )

if [ -z "$out_2" ]; then
    echo "Missing DNS server"
    if ! systemctl restart systemd-networkd; then
        echo "restart systemd-networkd failed"
        exit $NOTOK
    fi
fi

# if any of the if blocks touched, problem was present
if [ -z "$out_1" ] || [ -z "$out_2" ]; then
    exit $NOTOK
fi

exit $OK
