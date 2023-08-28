#!/usr/bin/env python3

import json
import datetime as dt
import subprocess

def duration(x, y):
    start = dt.datetime.strptime(x, "%Y-%m-%dT%H:%M:%SZ")
    end = dt.datetime.strptime(y, "%Y-%m-%dT%H:%M:%SZ")
    return end-start


def kubectl(args):
    result = subprocess.run(["kubectl", "-ojson"] + args.split(" "), check=True, capture_output=True)
    stdout = result.stdout.decode("utf-8")
    return json.loads(stdout)


output = []
cns_pods = kubectl("get pods -n kube-system -l k8s-app=azure-cns")
for p in cns_pods["items"]:
    nodeName = p["spec"]["nodeName"]
    created = p["metadata"]["creationTimestamp"]
    cniInstallStarted, cniInstallFinished = "", ""
    for s in p["status"]["initContainerStatuses"]:
        if s["name"] == "init-cni-dropgz":
            cniInstallStarted = s["state"]["terminated"]["startedAt"]
            cniInstallFinished = s["state"]["terminated"]["finishedAt"]

    if not cniInstallStarted or not cniInstallFinished:
        continue

    createdToCNIInstallStart = duration(created, cniInstallStarted)
    cniInstallStartToFinish = duration(cniInstallStarted, cniInstallFinished)
    output.append([
        nodeName,
        created,
        cniInstallStarted,
        cniInstallFinished,
        createdToCNIInstallStart,
        cniInstallStartToFinish,
    ])

for node in sorted(output):
    print(",".join(str(x) for x in node))
