#!/usr/bin/env python3

import http.client
import sys
from xml.dom import minidom


def query_nmagent_interfaces():
    conn = http.client.HTTPConnection("168.63.129.16", timeout=5)
    conn.request("GET", "/machine/plugins?comp=nmagent&type=getinterfaceinfov1")
    resp = conn.getresponse()
    return resp.read()


def get_primary_ip_from_nmagent_interfaces(data):
    # expected format:
    # <Interfaces><Interface MacAddress="6045BD80C08F" IsPrimary="true"><IPSubnet Prefix="10.224.0.0/16"><IPAddress Address="10.224.0.4" IsPrimary="true"/></IPSubnet></Interface></Interfaces>
    # nmagent returns only the IPv4 address even on dual-stack nodes.
    xml = minidom.parseString(data)
    for iface in xml.getElementsByTagName('Interface'):
        if iface.getAttribute('IsPrimary') == 'true':
            for ip in iface.getElementsByTagName('IPAddress'):
                if ip.getAttribute('IsPrimary') == 'true':
                    return ip.getAttribute('Address')


ip = get_primary_ip_from_nmagent_interfaces(query_nmagent_interfaces())
if ip is not None:
    print(ip)
