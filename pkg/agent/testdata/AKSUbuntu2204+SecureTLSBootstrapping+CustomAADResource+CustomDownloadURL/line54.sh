#! /usr/bin/env python3

import urllib3
import uuid
import xml.etree.ElementTree as ET

http = urllib3.PoolManager()

goal_state_xml = http.request(
        'GET',
        'http://168.63.129.16/machine/?comp=goalstate',
        headers={
            'x-ms-version': '2012-11-30'
        }
    )
goal_state = ET.fromstring(goal_state_xml.data.decode('utf-8'))
container_id = goal_state.findall('./Container/ContainerId')[0].text
role_config_name = goal_state.findall('./Container/RoleInstanceList/RoleInstance/Configuration/ConfigName')[0].text
deployment_id = role_config_name.split('.')[0]

with open('/var/lib/waagent/logcollector/logs.zip', 'rb') as logs:
    logs_data = logs.read()
    upload_logs = http.request(
        'PUT',
        'http://168.63.129.16:32526/vmAgentLog',
        headers={
            'x-ms-version': '2015-09-01',
            'x-ms-client-correlationid': str(uuid.uuid4()),
            'x-ms-client-name': 'AKSCSEPlugin',
            'x-ms-client-version': '0.1.0',
            'x-ms-containerid': container_id,
            'x-ms-vmagentlog-deploymentid': deployment_id,
        },
        body=logs_data,
    )

if upload_logs.status == 200:
    print("Successfully uploaded logs")
    exit(0)
else:
    print('Failed to upload logs')
    print(f'Response status: {upload_logs.status}')
    print(f'Response body:\n{upload_logs.data.decode("utf-8")}')
    exit(1)
