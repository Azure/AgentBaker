#!/bin/bash
#
#
#
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 80 -j DROP
iptables -I FORWARD -d 168.63.129.16 -p tcp --dport 32526 -j DROP
