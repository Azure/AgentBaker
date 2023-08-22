#!/usr/bin/env python3

from datetime import datetime
import json
import os
import subprocess
import sys
import time


MAX_RETRIES = 30
RETRY_INTERVAL_SECONDS = 10
CNI_CONFLIST_DIR = "/etc/cni/net.d"
EVENTS_LOGGING_DIR = "/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events/"
EVENT_TASK_NAME="AKS.Runtime.cni_conflist_create"
EVENT_LEVEL = "Microsoft.Azure.Extensions.CustomScript-1.23"


def retry(f, failure_val=None):
    count = 0
    while count < MAX_RETRIES:
        try:
            return f()
        except Exception as ex:
            print("Did not succeed, will retry:", ex)
            count += 1

        time.sleep(RETRY_INTERVAL_SECONDS)

    print("Max number of retries {} exceeded, will return {}".format(MAX_RETRIES, failure_val))
    return failure_val


def ensure_events_logging_dir_exists():
    if not os.path.exists(EVENTS_LOGGING_DIR):
        raise Exception("Directory {} does not yet exist".format(EVENTS_LOGGING_DIR))
    return True


def get_earliest_cni_conflist_create_ts():
    print("Checking CNI conflist...")
    earliest = None
    with os.scandir(CNI_CONFLIST_DIR) as iter:
        for entry in iter:
            _, ext = os.path.splitext(entry.name)
            if ext == ".conflist":
                # technically ctime is the last inode update,
                # which may be later than the actual (unknown) create time,
                # but for our purposes it's close enough.
                ctime = entry.stat().st_ctime
                print("Found CNI conflist {} with ctime {}".format(entry.name, ctime))
                if earliest is None or ctime < earliest:
                    earliest = ctime

    if earliest is None:
        raise Exception("CNI conflist not found")

    print("Earliest CNI conflist timestamp is", earliest)
    return earliest


def emit_event(msg):
    event_ts = datetime.now().isoformat(sep=" ", timespec="milliseconds")
    event = {
        "Timestamp": event_ts,
        "OperationId": event_ts,
        "Version": "1.23",
        "TaskName": EVENT_TASK_NAME,
        "EventLevel": EVENT_LEVEL,
        "Message": json.dumps(msg),
        "EventPid": "0",
        "EventTid": "0",
    }

    path = os.path.join(EVENTS_LOGGING_DIR, "{}.json".format(int(time.time_ns() / 1000000)))
    with open(path, "w") as f:
        json.dump(event, f)

    print("Wrote event", path)


def get_kubelet_start_ts():
    # Get kubelet.service ExecMainStartTime in seconds since the Unix epoch.
    stdout = subprocess.check_output("systemctl show kubelet.service -p ExecMainStartTimestamp | sed -e 's/ExecMainStartTimestamp=//g' | awk '{ printf(\"%s %s\", $2, $3) }' | xargs -I %% date --date=\"%%\" '+%s'", shell=True)
    return int(stdout.decode("utf8").strip())


def format_ts(ts):
    return datetime.utcfromtimestamp(ts).isoformat(sep=" ", timespec="milliseconds")


if __name__ == "__main__":
    # Wait for walinuxagent to create the events logging directory.
    if not retry(ensure_events_logging_dir_exists, failure_val=False):
        print("Directory {} was not created".format(EVENTS_LOGGING_DIR))
        sys.exit(1)

    kubelet_start_ts = retry(get_kubelet_start_ts)
    if kubelet_start_ts is None:
        print("Could not retrieve kubelet start time")
        sys.exit(1)

    ts = retry(get_earliest_cni_conflist_create_ts)
    if ts is None:
        # This is expected in a BYO CNI cluster where the customer
        # has not yet installed a CNI plugin.
        print("Emitting event that CNI conflist was not found")
        emit_event({
            "CNIConflistFound": False,
            "CNIConflistCreateTimestamp": None,
            "KubeletStartTimestamp": None,
            "DurationFromKubeletStartInSeconds": None,
        })
    elif ts < kubelet_start_ts:
        # CNI conflist should always be created after kubelet.
        # If not, then something unexpected happened, and we can't
        # trust this data.
        print("Kublet start_ts {} is after CNI create timestamp {}, abort", kubelet_start_ts, ts)
        sys.exit(1)
    else:
        print("Emitting event that CNI conflist found with timestamp")
        emit_event({
            "CNIConflistFound": True,
            "KubeletStartTimestamp": format_ts(kubelet_start_ts),
            "CNIConflistCreateTimestamp": format_ts(ts),
            "DurationFromKubeletStartInSeconds": int(ts - kubelet_start_ts),
        })
