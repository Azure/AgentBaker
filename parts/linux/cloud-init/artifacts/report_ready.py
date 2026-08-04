#!/usr/bin/env python3
"""Standalone script to report provisioning status to Azure fabric.

When cloud-init's built-in ready report is skipped via
experimental_skip_ready_report, this script can be invoked to send the
health status signal to Azure wireserver at the appropriate time during
node provisioning.

Protocol overview:
  * Ready/failure reports use the legacy GoalState health-report endpoint.
  * In-progress reports POST JSON to /provisioning/health.

Usage:
  report_ready.py [--endpoint ENDPOINT] [--retries N] [--retry-delay SECS]
  report_ready.py --failure --description "CSE failed with exit code 42"
  report_ready.py --in-progress --interval 60

The wireserver endpoint defaults to 168.63.129.16.
"""

import argparse
import csv
import fcntl
import io
import json
import logging
import os
import struct
import subprocess
import sys
import time
import xml.etree.ElementTree as ET
from datetime import datetime, timezone
from html import escape
from http.client import HTTPConnection, HTTPException
from textwrap import dedent

LOG = logging.getLogger("report_ready")

DEFAULT_WIRESERVER_ENDPOINT = "168.63.129.16"
DEFAULT_RETRIES = 3
DEFAULT_RETRY_DELAY = 5
DEFAULT_PROGRESS_INTERVAL = 60
DESCRIPTION_TRIM_LEN = 512

WIRESERVER_HEADERS = {
    "x-ms-agent-name": "WALinuxAgent",
    "x-ms-version": "2012-11-30",
}

HEALTH_REPORT_XML = dedent(
    """\
    <?xml version="1.0" encoding="utf-8"?>
    <Health xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
     xmlns:xsd="http://www.w3.org/2001/XMLSchema">
      <GoalStateIncarnation>{incarnation}</GoalStateIncarnation>
      <Container>
        <ContainerId>{container_id}</ContainerId>
        <RoleInstanceList>
          <Role>
            <InstanceId>{instance_id}</InstanceId>
            <Health>
              <State>{status}</State>
              {detail_subsection}
            </Health>
          </Role>
        </RoleInstanceList>
      </Container>
    </Health>
    """
)

HEALTH_DETAIL_XML = dedent(
    """\
    <Details>
              <SubStatus>{substatus}</SubStatus>
              <Description>{description}</Description>
            </Details>"""
)

# Hyper-V KVP constants matching cloud-init's protocol
KVP_POOL_FILE_GUEST = "/var/lib/hyperv/.kvp_pool_1"
KVP_KEY_SIZE = 512
KVP_VALUE_SIZE = 2048
KVP_RECORD_SIZE = KVP_KEY_SIZE + KVP_VALUE_SIZE
KVP_AZURE_MAX_VALUE_SIZE = 1024
KVP_PROVISIONING_KEY = "PROVISIONING_REPORT"
AGENT_NAME = "AKS-CSE"

PROVISIONING_HEALTH_HEADERS = {
    "User-Agent": AGENT_NAME,
    "x-ms-guest-agent-name": AGENT_NAME,
}

# When this file exists, cloud-init has handed ready reporting to this script.
REPORT_MARKER = "/var/lib/waagent/experimental_skip_ready_report"


# ---------------------------------------------------------------------------
# Hyper-V KVP helpers
# ---------------------------------------------------------------------------


def _encode_kvp_record(key: str, value: str) -> bytes:
    """Encode a key-value pair into a fixed-size KVP binary record."""
    return struct.pack(
        "%ds%ds" % (KVP_KEY_SIZE, KVP_VALUE_SIZE),
        key.encode("utf-8"),
        value.encode("utf-8"),
    )


def _append_kvp_record(record: bytes, kvp_file: str = KVP_POOL_FILE_GUEST) -> None:
    """Append a binary KVP record to the pool file with file locking."""
    try:
        with open(kvp_file, "ab") as f:
            fcntl.flock(f, fcntl.LOCK_EX)
            f.write(record)
            f.flush()
            fcntl.flock(f, fcntl.LOCK_UN)
    except OSError as e:
        LOG.warning("Failed to write KVP record: %s", e)


def _encode_report(fields: list) -> str:
    """Encode report fields as pipe-delimited CSV, matching cloud-init format."""
    buf = io.StringIO()
    csv.writer(buf, delimiter="|", quotechar="'", quoting=csv.QUOTE_MINIMAL).writerow(fields)
    return buf.getvalue().rstrip()


def _get_vm_id() -> str:
    """Read the VM ID from DMI system-uuid."""
    try:
        with open("/sys/class/dmi/id/product_uuid", "r") as f:
            return f.read().strip().lower()
    except OSError:
        pass
    try:
        result = subprocess.run(
            ["dmidecode", "-s", "system-uuid"],
            capture_output=True, text=True, timeout=5,
        )
        if result.returncode == 0:
            return result.stdout.strip().lower()
    except Exception:
        pass
    return "00000000-0000-0000-0000-000000000000"


def write_provisioning_kvp(report: str) -> None:
    """Write a provisioning report to the KVP pool file."""
    if len(report) >= KVP_AZURE_MAX_VALUE_SIZE:
        report = report[:KVP_AZURE_MAX_VALUE_SIZE - 1]
    record = _encode_kvp_record(KVP_PROVISIONING_KEY, report)
    _append_kvp_record(record)
    LOG.info("Wrote PROVISIONING_REPORT to KVP.")


def report_dmesg_to_kvp() -> None:
    """Capture dmesg output and write it to KVP, matching cloud-init behavior."""
    try:
        result = subprocess.run(
            ["dmesg"], capture_output=True, timeout=10,
        )
        dmesg = result.stdout[-KVP_AZURE_MAX_VALUE_SIZE:] if result.stdout else b""
        record = _encode_kvp_record("dmesg", dmesg.decode("utf-8", errors="replace"))
        _append_kvp_record(record)
        LOG.debug("Wrote dmesg to KVP.")
    except Exception as e:
        LOG.warning("Failed to dump dmesg to KVP: %s", e)


def kvp_report_success(vm_id: str) -> None:
    """Write success provisioning report to KVP, matching cloud-init format."""
    report = _encode_report([
        "result=success",
        f"agent={AGENT_NAME}",
        f"timestamp={datetime.now(timezone.utc).isoformat()}",
        f"vm_id={vm_id}",
    ])
    write_provisioning_kvp(report)


def kvp_report_failure(description: str, vm_id: str) -> None:
    """Write failure provisioning report to KVP, matching cloud-init format."""
    report = _encode_report([
        "result=failure",
        f"agent={AGENT_NAME}",
        f"timestamp={datetime.now(timezone.utc).isoformat()}",
        f"vm_id={vm_id}",
        f"description={description[:DESCRIPTION_TRIM_LEN]}",
    ])
    write_provisioning_kvp(report)


# ---------------------------------------------------------------------------
# Wireserver HTTP helpers
# ---------------------------------------------------------------------------


def http_get(endpoint: str, path: str) -> bytes:
    """Perform an HTTP GET against the wireserver."""
    conn = HTTPConnection(endpoint, timeout=30)
    try:
        conn.request("GET", path, headers=WIRESERVER_HEADERS)
        resp = conn.getresponse()
        if resp.status != 200:
            raise RuntimeError(
                f"GET {path} returned HTTP {resp.status}: {resp.reason}"
            )
        return resp.read()
    finally:
        conn.close()


def http_post(
    endpoint: str,
    path: str,
    body: bytes,
    headers: dict = None,
    content_type: str = "text/xml; charset=utf-8",
) -> None:
    """Perform an HTTP POST against the wireserver."""
    request_headers = {
        **(headers or WIRESERVER_HEADERS),
        "Content-Type": content_type,
    }
    conn = HTTPConnection(endpoint, timeout=30)
    try:
        conn.request("POST", path, body=body, headers=request_headers)
        resp = conn.getresponse()
        if resp.status not in (200, 201, 202):
            raise RuntimeError(
                f"POST {path} returned HTTP {resp.status}: {resp.reason}"
            )
    finally:
        conn.close()


def _xml_text(root: ET.Element, xpath: str) -> str:
    """Extract text from an XML element, raising if missing."""
    el = root.find(xpath)
    if el is None or el.text is None:
        raise ValueError(f"Missing element at xpath: {xpath}")
    return el.text


def fetch_goalstate(endpoint: str) -> dict:
    """Fetch and parse the GoalState from wireserver.

    Returns dict with keys: container_id, instance_id, incarnation.
    """
    raw = http_get(endpoint, "/machine/?comp=goalstate")
    root = ET.fromstring(raw)  # nosec B314

    return {
        "container_id": _xml_text(root, "./Container/ContainerId"),
        "instance_id": _xml_text(
            root,
            "./Container/RoleInstanceList/RoleInstance/InstanceId",
        ),
        "incarnation": _xml_text(root, "./Incarnation"),
    }


def build_health_report(
    goalstate: dict,
    status: str = "Ready",
    substatus: str = None,
    description: str = None,
) -> bytes:
    """Build the health report XML for ready or failure."""
    detail_subsection = ""
    if substatus is not None:
        detail_subsection = HEALTH_DETAIL_XML.format(
            substatus=escape(substatus),
            description=escape(
                (description or "")[:DESCRIPTION_TRIM_LEN]
            ),
        )

    return HEALTH_REPORT_XML.format(
        incarnation=escape(str(goalstate["incarnation"])),
        container_id=escape(goalstate["container_id"]),
        instance_id=escape(goalstate["instance_id"]),
        status=escape(status),
        detail_subsection=detail_subsection,
    ).encode("utf-8")


def build_provisioning_health_report(
    state: str,
    substatus: str = None,
    description: str = None,
) -> bytes:
    """Build a JSON report for the provisioning-health endpoint."""
    report = {"state": state}
    if substatus is not None:
        report["details"] = {
            "subStatus": substatus,
            "description": (description or "")[:DESCRIPTION_TRIM_LEN],
        }
    return json.dumps(report, separators=(",", ":")).encode("utf-8")


def _reporting_enabled(operation: str) -> bool:
    """Return whether this script owns provisioning status reporting."""
    if os.path.exists(REPORT_MARKER):
        return True
    LOG.info(
        "Skipping %s: marker file %s does not exist, "
        "cloud-init is handling the ready report.",
        operation,
        REPORT_MARKER,
    )
    return False


def _send_report(
    endpoint: str,
    status: str,
    substatus: str = None,
    description: str = None,
    retries: int = DEFAULT_RETRIES,
    retry_delay: float = DEFAULT_RETRY_DELAY,
) -> None:
    """Fetch GoalState and send a health report to Azure fabric."""
    last_err = None
    for attempt in range(1, retries + 1):
        try:
            LOG.info(
                "Fetching GoalState from %s (attempt %d/%d)",
                endpoint,
                attempt,
                retries,
            )
            goalstate = fetch_goalstate(endpoint)
            LOG.info(
                "GoalState: container=%s instance=%s incarnation=%s",
                goalstate["container_id"],
                goalstate["instance_id"],
                goalstate["incarnation"],
            )

            document = build_health_report(
                goalstate,
                status=status,
                substatus=substatus,
                description=description,
            )
            LOG.info("Sending %s health report to %s", status, endpoint)
            http_post(endpoint, "/machine?comp=health", document)
            LOG.info("Successfully reported %s to Azure fabric.", status)
            return
        except (HTTPException, OSError, RuntimeError, ValueError) as exc:
            last_err = exc
            LOG.warning(
                "Attempt %d/%d failed: %s", attempt, retries, exc
            )
            if attempt < retries:
                time.sleep(retry_delay)

    LOG.error("Failed to report %s after %d attempts.", status, retries)
    raise RuntimeError(
        f"Failed to report {status} after {retries} attempts"
    ) from last_err


def _send_provisioning_health_report(
    endpoint: str,
    state: str,
    substatus: str = None,
    description: str = None,
    retries: int = DEFAULT_RETRIES,
    retry_delay: float = DEFAULT_RETRY_DELAY,
) -> None:
    """Send a JSON report to the provisioning-health endpoint."""
    document = build_provisioning_health_report(
        state=state,
        substatus=substatus,
        description=description,
    )
    last_err = None
    for attempt in range(1, retries + 1):
        try:
            LOG.info(
                "Sending %s/%s provisioning-health report to %s "
                "(attempt %d/%d)",
                state,
                substatus,
                endpoint,
                attempt,
                retries,
            )
            http_post(
                endpoint,
                "/provisioning/health",
                document,
                headers=PROVISIONING_HEALTH_HEADERS,
                content_type="application/json",
            )
            LOG.info(
                "Successfully reported %s/%s to Azure fabric.",
                state,
                substatus,
            )
            return
        except (HTTPException, OSError, RuntimeError) as exc:
            last_err = exc
            LOG.warning(
                "Attempt %d/%d failed: %s", attempt, retries, exc
            )
            if attempt < retries:
                time.sleep(retry_delay)

    raise RuntimeError(
        f"Failed to report {state}/{substatus} after {retries} attempts"
    ) from last_err


def report_ready(
    endpoint: str = DEFAULT_WIRESERVER_ENDPOINT,
    retries: int = DEFAULT_RETRIES,
    retry_delay: float = DEFAULT_RETRY_DELAY,
) -> None:
    """Send a provisioning success (Ready) report to Azure fabric.

    Matches cloud-init's _report_ready flow:
      1. Write dmesg to KVP
      2. Write success PROVISIONING_REPORT to KVP
      3. Fetch GoalState and POST Ready health report to wireserver

    Skipped unless the experimental_skip_ready_report marker exists, which
    indicates cloud-init has handed reporting to this script.
    """
    if not _reporting_enabled("report_ready"):
        return
    vm_id = _get_vm_id()
    report_dmesg_to_kvp()
    kvp_report_success(vm_id)
    _send_report(
        endpoint=endpoint,
        status="Ready",
        retries=retries,
        retry_delay=retry_delay,
    )


def report_failure(
    description: str,
    endpoint: str = DEFAULT_WIRESERVER_ENDPOINT,
    retries: int = DEFAULT_RETRIES,
    retry_delay: float = DEFAULT_RETRY_DELAY,
) -> None:
    """Send a provisioning failure (NotReady) report to Azure fabric.

    Matches cloud-init's _report_failure flow:
      1. Write dmesg to KVP
      2. Write failure PROVISIONING_REPORT to KVP
      3. Fetch GoalState and POST NotReady health report to wireserver

    Skipped unless the experimental_skip_ready_report marker exists.
    """
    if not _reporting_enabled("report_failure"):
        return
    vm_id = _get_vm_id()
    report_dmesg_to_kvp()
    kvp_report_failure(description, vm_id)
    _send_report(
        endpoint=endpoint,
        status="NotReady",
        substatus="ProvisioningFailed",
        description=description,
        retries=retries,
        retry_delay=retry_delay,
    )


def report_in_progress(
    endpoint: str = DEFAULT_WIRESERVER_ENDPOINT,
    retries: int = DEFAULT_RETRIES,
    retry_delay: float = DEFAULT_RETRY_DELAY,
) -> None:
    """Report that provisioning is still running."""
    if not _reporting_enabled("report_in_progress"):
        return
    vm_id = _get_vm_id()
    _send_provisioning_health_report(
        endpoint=endpoint,
        state="NotReady",
        substatus="Provisioning",
        description=f"AKS CSE provisioning is still in progress for vm_id={vm_id}.",
        retries=retries,
        retry_delay=retry_delay,
    )


def report_in_progress_forever(
    endpoint: str = DEFAULT_WIRESERVER_ENDPOINT,
    retries: int = DEFAULT_RETRIES,
    retry_delay: float = DEFAULT_RETRY_DELAY,
    interval: float = DEFAULT_PROGRESS_INTERVAL,
) -> None:
    """Send in-progress reports periodically until the process is terminated."""
    while True:
        try:
            report_in_progress(
                endpoint=endpoint,
                retries=retries,
                retry_delay=retry_delay,
            )
        except RuntimeError as exc:
            LOG.warning("Failed to send in-progress report: %s", exc)
        time.sleep(interval)


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Report provisioning status to Azure wireserver."
    )
    parser.add_argument(
        "--endpoint",
        default=DEFAULT_WIRESERVER_ENDPOINT,
        help=f"Wireserver endpoint (default: {DEFAULT_WIRESERVER_ENDPOINT})",
    )
    parser.add_argument(
        "--retries",
        type=int,
        default=DEFAULT_RETRIES,
        help=f"Number of retry attempts (default: {DEFAULT_RETRIES})",
    )
    parser.add_argument(
        "--retry-delay",
        type=float,
        default=DEFAULT_RETRY_DELAY,
        help=f"Delay between retries in seconds (default: {DEFAULT_RETRY_DELAY})",
    )
    parser.add_argument(
        "--verbose", "-v", action="store_true", help="Enable debug logging"
    )
    report_type = parser.add_mutually_exclusive_group()
    report_type.add_argument(
        "--failure", action="store_true",
        help="Report provisioning failure instead of success",
    )
    report_type.add_argument(
        "--in-progress", action="store_true",
        help="Periodically report that provisioning is still running",
    )
    parser.add_argument(
        "--description", type=str, default="",
        help="Failure description (used with --failure)",
    )
    parser.add_argument(
        "--interval",
        type=float,
        default=DEFAULT_PROGRESS_INTERVAL,
        help=(
            "Seconds between in-progress reports "
            f"(default: {DEFAULT_PROGRESS_INTERVAL})"
        ),
    )
    args = parser.parse_args()
    if args.interval <= 0:
        parser.error("--interval must be greater than zero")

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s - %(name)s [%(levelname)s]: %(message)s",
    )

    try:
        if args.in_progress:
            if not _reporting_enabled("report_in_progress"):
                return 0
            report_in_progress_forever(
                endpoint=args.endpoint,
                retries=args.retries,
                retry_delay=args.retry_delay,
                interval=args.interval,
            )
        elif args.failure:
            report_failure(
                description=args.description,
                endpoint=args.endpoint,
                retries=args.retries,
                retry_delay=args.retry_delay,
            )
        else:
            report_ready(
                endpoint=args.endpoint,
                retries=args.retries,
                retry_delay=args.retry_delay,
            )
        return 0
    except RuntimeError:
        return 1


if __name__ == "__main__":
    sys.exit(main())
