#!/usr/bin/env python3

import http.client
import sys
from xml.dom import minidom

conn = http.client.HTTPConnection("168.63.129.16", timeout=5)
conn.request("GET", "/machine/plugins?comp=nmagent&type=getinterfaceinfov1")
resp = conn.getresponse()
xml = minidom.parseString(resp.read())
for iface in xml.getElementsByTagName('Interface'):
    if iface.getAttribute('IsPrimary') == 'true':
        for ip in iface.getElementsByTagName('IPAddress'):
            if ip.getAttribute('IsPrimary') == 'true':
                print(ip.getAttribute('Address'))
                sys.exit(0)
