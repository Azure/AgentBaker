#!/bin/bash
# Disallow container from reaching out to the special IP address 168.63.129.16
# for TCP protocol (which http uses)
#
# 168.63.129.16 contains protected settings that have priviledged info.
# HostGAPlugin (Host-GuestAgent-Plugin) is a web server process that runs on the physical host that serves the operational and diagnostic needs of the in-VM Guest Agent.  
# IT listens on both port 80 and 32526 hence access is only needed for agent but not the containers.
#
# The host can still reach 168.63.129.16 because it goes through the OUTPUT chain, not FORWARD.
#
# Note: we should not block all traffic to 168.63.129.16. For example UDP traffic is still needed
# for DNS.
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 32526 -j DROP
