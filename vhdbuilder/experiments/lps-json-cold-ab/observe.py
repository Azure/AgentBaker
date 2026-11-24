#!/usr/bin/env python3
"""Read metadata only; missing/partial evidence is never a cold-cache pass."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import time
import tomllib

BASE = Path("/opt/azure/containers/lps-json-cold-ab")
ROOT = Path("/var/lib/containerd")
SNAPSHOTTER = "overlayfs"
SOCKET = "/run/containerd/containerd.sock"
CONFIG = Path("/etc/containerd/config.toml")
UNIT = "aks-lps-json-prefetch.service"
UNIT_PROPERTIES = (
    "Id", "Type", "RemainAfterExit", "RuntimeMaxUSec", "TimeoutStartUSec",
    "TimeoutStopUSec", "KillMode", "UMask", "Before", "After", "Requires", "Wants",
    "BindsTo", "PartOf", "ControlGroup", "InvocationID", "ExecMainPID", "ExecMainCode",
    "ExecMainStatus", "ExecMainStartTimestampMonotonic", "ExecMainExitTimestampMonotonic",
    "Result",
)
UNIT_EMPTY_ALLOWED = {"Before", "After", "Requires", "Wants", "BindsTo", "PartOf"}


def stamp():
    return {
        "observed_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "monotonic_ns": time.monotonic_ns(),
        "boottime_ns": time.clock_gettime_ns(time.CLOCK_BOOTTIME),
        "boot_id": Path("/proc/sys/kernel/random/boot_id").read_text().strip(),
    }


def capture_effective_unit(invocation_id):
    result = dict(schemaVersion=1, **stamp(), invocation_id=invocation_id, unit=UNIT,
                  source="systemctl-show-inside-ExecStopPost", timeout_seconds=3,
                  properties={}, query_exit_code=None, errors=[])
    try:
        query = subprocess.run(
            ["systemctl", "show", "--no-pager", "--all",
             "--property=" + ",".join(UNIT_PROPERTIES), UNIT],
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True, timeout=3,
            check=False)
        result["query_exit_code"] = query.returncode
        if query.returncode != 0:
            result["errors"].append("query-failed")
        for line in query.stdout.splitlines():
            key, separator, value = line.partition("=")
            # Never persist raw output, unexpected properties, or their values.
            if not separator or key not in UNIT_PROPERTIES:
                result["errors"].append("unexpected-query-output")
            elif key in result["properties"]:
                result["errors"].append("duplicate-property")
            else:
                result["properties"][key] = value
    except subprocess.TimeoutExpired:
        result["errors"].append("query-timeout")
    except Exception as exc:
        result["errors"].append(type(exc).__name__)
    properties = result["properties"]
    result["missing_properties"] = sorted(set(UNIT_PROPERTIES) - properties.keys())
    result["empty_properties"] = sorted(
        key for key, value in properties.items() if not value and key not in UNIT_EMPTY_ALLOWED)
    if result["missing_properties"] or result["empty_properties"]:
        result["errors"].append("incomplete-properties")
    if properties.get("Id") != UNIT:
        result["errors"].append("unit-identity-mismatch")
    if not invocation_id or properties.get("InvocationID") != invocation_id:
        result["errors"].append("invocation-identity-mismatch")
    result["complete"] = not result["missing_properties"] and not result["empty_properties"]
    result["finished"] = stamp()
    result["valid"] = not result["errors"]
    return result


def capture():
    result = dict(schemaVersion=1, **stamp(), containerd_root=str(ROOT),
                  snapshotter=SNAPSHOTTER, containerd_socket=SOCKET,
                  content=[], namespaces=[], errors=[])

    def command(*args):
        return subprocess.check_output(
            ["ctr", "--address", SOCKET, "--timeout", "3s", *args], text=True, timeout=4,
            stderr=subprocess.DEVNULL).strip()

    try:
        config_bytes = CONFIG.read_bytes()
        config = tomllib.loads(config_bytes.decode())
        result["containerd_config_sha256"] = hashlib.sha256(config_bytes).hexdigest()
        plugins = config.get("plugins", {})
        cri = plugins.get("io.containerd.cri.v1.images",
                          plugins.get("io.containerd.grpc.v1.cri", {}))
        if (config.get("root", str(ROOT)) != str(ROOT) or
                cri.get("containerd", {}).get("snapshotter", SNAPSHOTTER) != SNAPSHOTTER or
                config.get("grpc", {}).get("address", SOCKET) != SOCKET):
            raise ValueError("unqualified containerd root, socket or snapshotter")
        inventory = (BASE / "expected-inventory.json").read_bytes()
        result["expected_inventory_sha256"] = hashlib.sha256(inventory).hexdigest()
        blobs = ROOT / "io.containerd.content.v1.content" / "blobs" / "sha256"
        # stat only: do not open, hash, fetch, mount or unpack layer blobs.
        for blob in sorted(blobs.iterdir()):
            result["content"].append(dict(digest="sha256:" + blob.name, size=blob.stat().st_size))
        namespaces = command("namespaces", "list", "--quiet").splitlines()
        if "k8s.io" not in namespaces:
            raise ValueError("missing CRI namespace")
        for namespace in namespaces:
            entry = dict(name=namespace, images=command("-n", namespace, "images", "list"),
                         snapshots=[], snapshot_listing=command(
                             "-n", namespace, "snapshots", "--snapshotter", SNAPSHOTTER, "list"))
            result["namespaces"].append(entry)
            for line in entry["snapshot_listing"].splitlines()[1:]:
                key = line.split()[0]
                info = json.loads(command("-n", namespace, "snapshots", "--snapshotter",
                                          SNAPSHOTTER, "info", key))
                entry["snapshots"].append(info)
        result["physical_snapshot_ids"] = sorted(
            item.name for item in (ROOT / "io.containerd.snapshotter.v1.overlayfs" / "snapshots").iterdir()
            if item.is_dir())
    except Exception as exc:
        result["errors"].append(type(exc).__name__)
    result["finished"] = stamp()
    result["valid"] = not result["errors"]
    return result


def main():
    phase = sys.argv[1]
    if phase not in {"before-prefetch", "before-kubelet", "worker-start", "worker-end",
                     "unit-exit", "launch-result"}:
        raise ValueError("invalid observation phase")
    BASE.mkdir(mode=0o700, parents=True, exist_ok=True)
    if phase.startswith("before-"):
        result = capture()
    else:
        result = dict(schemaVersion=1, **stamp())
        result.update(mode=os.environ.get("LPS_PREFETCH_MODE"),
                      worker_pid=os.environ.get("LPS_PREFETCH_WORKER_PID"),
                      invocation_id=os.environ.get("INVOCATION_ID"))
        if phase == "launch-result":
            result.update(exit_code=int(sys.argv[2]), unit="aks-lps-json-prefetch.service")
        if phase == "worker-end":
            result["exit_code"] = int(sys.argv[2])
            status = Path("/opt/azure/containers/image-cache-status.json")
            try:
                # The ANC command deliberately returns zero on failure; preserve its real outcome.
                data = json.loads(status.read_text())
                result["prefetch_status"] = {key: data.get(key) for key in
                    ("schemaVersion", "source", "outcome", "reason", "startedAt", "finishedAt")}
            except Exception:
                result["prefetch_status_missing"] = True
            result["worker_exit_code"] = result["exit_code"] or (
                0 if result.get("prefetch_status", {}).get("outcome") == "complete" else 1)
        if phase == "unit-exit":
            result.update({key: os.environ.get(key) for key in
                           ("SERVICE_RESULT", "EXIT_CODE", "EXIT_STATUS")})
            result["effective_unit"] = capture_effective_unit(result["invocation_id"])
            result["valid"] = result["effective_unit"]["valid"] and all(
                result[key] for key in ("SERVICE_RESULT", "EXIT_CODE", "EXIT_STATUS"))
    result["phase"] = phase
    target = BASE / (phase + ".json")
    # Never overwrite an earlier observation on CSE retries.
    with target.open("x") as stream:
        json.dump(result, stream, sort_keys=True)
        stream.write("\n")
    if phase == "worker-end":
        sys.exit(result["worker_exit_code"])


if __name__ == "__main__":
    main()
