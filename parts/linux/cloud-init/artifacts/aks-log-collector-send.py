#! /usr/bin/env python3

import urllib3
import uuid
import time
import xml.etree.ElementTree as ET

MAX_RETRIES = 10

def upload_logs():
    print('Uploading logs via Wireserver...')

    # retry policy for each request made via the pool manager
    retries = urllib3.util.Retry(
        total=MAX_RETRIES,
        backoff_factor=0.5,
        backoff_max=10,
        status_forcelist=[429, 500, 502, 503, 504],
    )
    
    for retry in range(MAX_RETRIES):
        try:
            http = urllib3.PoolManager(retries=retries)

            # Get the container_id and deployment_id from the Goal State
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

            # Upload the logs
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
                print(f'(retry={retry}) Successfully uploaded logs')
                exit(0)
            else:
                print(f'(retry={retry}) Failed to upload logs')
                print(f'(retry={retry}) Response status: {upload_logs.status}')
                print(f'(retry={retry}) Response body:\n{upload_logs.data.decode("utf-8")}')

        except Exception as e:
            print(f'(retry={retry}) Failed to upload logs, encountered exception: {e}')
            if retry < MAX_RETRIES - 1:
                print(f'will attempt upload again in 3 seconds')
                time.Sleep(3)
    
    print(f'Failed to upload logs after {MAX_RETRIES} retries')
    exit(1)

if __name__ == "__main__":
    upload_logs()