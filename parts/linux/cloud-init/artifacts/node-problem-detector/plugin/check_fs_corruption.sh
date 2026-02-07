#!/bin/bash

OK=0
NOTOK=1
UNKNOWN=2

if ! which systemctl >/dev/null; then
    echo "Systemd is not supported"
    exit $UNKNOWN
fi

# Check for filesystem corruption surfaced by docker
if journalctl -u docker --since "5 min ago" | grep -q "structure needs cleaning"; then
    echo "Found 'structure needs cleaning' in Docker journal."
    exit $NOTOK
fi

exit $OK