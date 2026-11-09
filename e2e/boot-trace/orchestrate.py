"""Explicit AKS +1 scale-out. No image installation or automatic scale-down."""

import codecs
import datetime
import ipaddress
import json
import os
from pathlib import Path, PurePosixPath
import queue
import re
import selectors
import subprocess
import tarfile
import threading
import time
import uuid
from urllib.parse import urlparse


PROVIDER = re.compile(
    r"(?:azure://)?/subscriptions/([^/]+)/resourcegroups/([^/]+)/providers/"
    r"microsoft.compute/virtualmachinescalesets/([^/]+)/virtualmachines/(\d+)",
    re.IGNORECASE,
)


def command(args, timeout=90):
    result = subprocess.run(
        args, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        text=True, timeout=timeout,
    )
    return result.stdout


def save(path, data):
    path.write_text(json.dumps(data, indent=2, allow_nan=False) + "\n", encoding="utf-8")


def terminate(process):
    if process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)


class NodeWatch:
    """Keep JSON parsing separate from orchestration; fail visibly on watch gaps."""

    def __init__(self, args, stderr_path):
        self.errors = stderr_path.open("wb")
        self.process = subprocess.Popen(
            args, stdout=subprocess.PIPE, stderr=self.errors,
        )
        self.events = queue.Queue(maxsize=256)
        self.failure = None
        self.closed = threading.Event()
        self.thread = threading.Thread(target=self.read, daemon=True)
        self.thread.start()

    def read(self):
        decoder = json.JSONDecoder()
        utf8 = codecs.getincrementaldecoder("utf-8")()
        pending = ""
        try:
            while not self.closed.is_set():
                chunk = self.process.stdout.read1(65536)
                pending += utf8.decode(chunk, final=not chunk)
                while pending.strip():
                    pending = pending.lstrip()
                    try:
                        event, end = decoder.raw_decode(pending)
                    except json.JSONDecodeError:
                        break
                    self.events.put(event, timeout=1)
                    pending = pending[end:]
                if len(pending) > 8 * 1024 * 1024:
                    raise ValueError("Kubernetes watch message exceeds 8 MiB")
                if not chunk:
                    raise RuntimeError(
                        "Kubernetes watch ended; coverage is incomplete. See watch.stderr."
                    )
        except (OSError, ValueError, RuntimeError, queue.Full) as error:
            self.failure = error

    def next(self, timeout):
        try:
            return self.events.get(timeout=timeout)
        except queue.Empty:
            if self.failure:
                raise RuntimeError(f"Node watch failed: {self.failure}") from self.failure
            return None

    def close(self):
        self.closed.set()
        terminate(self.process)
        self.thread.join(timeout=6)
        self.process.stdout.close()
        self.errors.close()


def provider(node):
    match = PROVIDER.fullmatch(node.get("spec", {}).get("providerID", ""))
    if not match:
        raise ValueError(f"Unsupported providerID on node {node['metadata']['name']}")
    return match.groups()


def unpack(archive, destination, boot_id, maximum):
    """Extract regular files only; never trust tar paths, symlinks or devices."""
    total = 0
    with tarfile.open(archive, mode="r:") as source:
        for member in source:
            path = PurePosixPath(member.name)
            if not path.parts or path.is_absolute() or ".." in path.parts or path.parts[0] != boot_id:
                raise ValueError("Unsafe path in guest capture archive")
            if member.isdir():
                continue
            if not member.isfile() or member.size < 0:
                raise ValueError("Capture archive must contain only regular files")
            total += member.size
            if total > maximum:
                raise ValueError("Expanded capture exceeds the configured byte limit")
            target = destination.joinpath(*path.parts[1:])
            if target == destination:
                raise ValueError("Invalid archive file path")
            target.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
            with source.extractfile(member) as content, target.open("xb") as output:
                while chunk := content.read(65536):
                    output.write(chunk)
            target.chmod(0o600)


def fetch(ssh, remote, destination, maximum):
    """Bound remote tar transfer in bytes and wall time, preserving stderr."""
    with (destination.parent / "ssh.stderr").open("wb") as errors:
        process = subprocess.Popen(
            [*ssh, remote], stdout=subprocess.PIPE, stderr=errors,
        )
        selector = selectors.DefaultSelector()
        selector.register(process.stdout, selectors.EVENT_READ)
        size = 0
        deadline = time.monotonic() + 180
        try:
            with destination.open("xb") as output:
                while True:
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        raise TimeoutError("Guest archive transfer timed out")
                    if not selector.select(timeout=min(1, remaining)):
                        continue
                    chunk = os.read(process.stdout.fileno(), 65536)
                    if not chunk:
                        break
                    size += len(chunk)
                    if size > maximum:
                        raise ValueError("Guest archive transfer exceeds byte limit")
                    output.write(chunk)
            if process.wait(timeout=10) != 0:
                raise RuntimeError("SSH archive transfer failed; see ssh.stderr")
        finally:
            selector.close()
            terminate(process)
            process.stdout.close()


def scale_and_collect(args):
    if args.timeout <= 0 or args.max_mib < 16:
        raise ValueError("Positive timeout and at least 16 MiB are required")
    if not re.fullmatch(r"[a-z_][a-z0-9_-]*", args.ssh_user):
        raise ValueError("Invalid SSH user")
    subscription = str(uuid.UUID(args.subscription))
    def az(*arguments):
        return [
            "az", *arguments, "--subscription", subscription,
            "--output", "json", "--only-show-errors",
        ]

    kube = ["kubectl", "--kubeconfig", str(args.kubeconfig), "--context", args.context]
    cluster = json.loads(command(az(
        "aks", "show", "--resource-group", args.resource_group, "--name", args.cluster,
    )))
    pool = json.loads(command(az(
        "aks", "nodepool", "show", "--resource-group", args.resource_group,
        "--cluster-name", args.cluster, "--name", args.pool,
    )))
    if cluster.get("provisioningState") != "Succeeded" or pool.get("provisioningState") != "Succeeded":
        raise ValueError("Cluster and pool must be in Succeeded state")
    if pool.get("type") != "VirtualMachineScaleSets" or pool.get("osType") != "Linux":
        raise ValueError("Only Linux VMSS pools are supported")
    if pool.get("enableAutoScaling") or pool.get("scaleDownMode") == "Deallocate":
        raise ValueError("Use a dedicated pool without autoscaler or deallocated instances")
    # Check that the selected kube context points at this cluster's API endpoint.
    config = json.loads(command([*kube, "config", "view", "--minify", "--output=json"]))
    server = urlparse(config["clusters"][0]["cluster"]["server"]).hostname
    endpoints = {cluster.get("fqdn"), cluster.get("privateFqdn")}
    if not server or server.lower() not in {x.lower() for x in endpoints if x}:
        raise ValueError("Kubernetes context does not match the Azure cluster endpoint")
    baseline = json.loads(command([
        *kube, "get", "nodes", "-l", "agentpool=" + args.pool, "--output=json",
    ]))
    if not baseline["items"] or len(baseline["items"]) != pool["count"]:
        raise ValueError("Pool must have an existing registered node and no count discrepancy")
    identities = [provider(node) for node in baseline["items"]]
    target_vmss = {identity[:3] for identity in identities}
    if len(target_vmss) != 1:
        raise ValueError("Expected a single backing VMSS")
    vm_subscription, node_rg, vmss = identities[0][:3]
    if vm_subscription.lower() != subscription.lower() or node_rg.lower() != cluster["nodeResourceGroup"].lower():
        raise ValueError("Node providerID disagrees with Azure cluster identity")
    instances = json.loads(command(az(
        "vmss", "list-instances", "--resource-group", node_rg, "--name", vmss,
        "--query", "[].instanceId",
    )))
    initial_instances = {str(instance) for instance in instances}
    if len(initial_instances) != pool["count"]:
        raise ValueError("VMSS instance count differs from the AKS pool count")
    scale = az(
        "aks", "nodepool", "scale", "--resource-group", args.resource_group,
        "--cluster-name", args.cluster, "--name", args.pool,
        "--node-count", str(pool["count"] + 1), "--no-wait",
    )
    description = {
        "operation": "AKS nodepool scale +1", "cluster": cluster["id"],
        "pool": args.pool, "from_count": pool["count"], "to_count": pool["count"] + 1,
        "backend": args.backend, "scale_command": scale,
        "note": "No automatic scale-down. This creates a billable node.",
    }
    if not args.execute:
        print(json.dumps(description, indent=2))
        return
    if not args.instrumentation_confirmed:
        raise ValueError(
            "First-boot instrumentation must be prepared and verified separately. "
            "Pass --instrumentation-confirmed only for that dedicated test pool."
        )
    os.umask(0o077)
    args.output.mkdir(parents=True, mode=0o700, exist_ok=False)
    save(args.output / "operation.json", description)
    watch = NodeWatch([
        *kube, "get", "nodes", "-l", "agentpool=" + args.pool, "--watch-only",
        "--resource-version=" + baseline["metadata"]["resourceVersion"],
        "--output-watch-events", "--output=json",
    ], args.output / "watch.stderr")
    seen = {node["metadata"]["uid"] for node in baseline["items"]}
    new_uid = None
    try:
        command(scale)
        deadline = time.monotonic() + args.timeout
        node = None
        while time.monotonic() < deadline:
            event = watch.next(timeout=min(1, max(0.01, deadline - time.monotonic())))
            if event is None:
                continue
            if event.get("type") == "ERROR":
                raise RuntimeError("Kubernetes watch error: " + json.dumps(event["object"]))
            if event.get("type") == "BOOKMARK":
                continue
            if event.get("type") not in ("ADDED", "MODIFIED", "DELETED"):
                raise ValueError("Unexpected Kubernetes watch event type")
            candidate = event["object"]
            uid = candidate["metadata"]["uid"]
            if uid in seen:
                continue
            if not candidate.get("spec", {}).get("providerID"):
                continue
            identity = provider(candidate)
            if tuple(part.lower() for part in identity[:3]) != tuple(
                part.lower() for part in identities[0][:3]
            ) or identity[3] in initial_instances:
                raise ValueError("New node does not identify a newly created target VMSS instance")
            if new_uid and new_uid != uid:
                raise RuntimeError("Multiple new nodes observed; concurrent activity or replacement")
            new_uid = uid
            if event["type"] == "DELETED":
                raise RuntimeError("The new node was deleted before Ready")
            condition = next((
                c for c in candidate.get("status", {}).get("conditions", [])
                if c["type"] == "Ready" and c["status"] == "True"
            ), None)
            if condition:
                node = candidate
                observed = datetime.datetime.now(datetime.timezone.utc).isoformat()
                break
        if node is None:
            raise TimeoutError("New node did not become Ready; guest watchdog remains responsible for stopping tracing")
        boot_id = str(uuid.UUID(node["status"]["nodeInfo"]["bootID"]))
        addresses = [
            item["address"] for item in node["status"]["addresses"]
            if item["type"] == "InternalIP"
        ]
        if not addresses:
            raise ValueError("New node has no InternalIP for SSH collection")
        ip = str(ipaddress.ip_address(addresses[0]))
        profile = cluster.get("networkProfile", {})
        readiness = {
            "node": node["metadata"]["name"], "node_uid": node["metadata"]["uid"],
            "provider_id": node["spec"]["providerID"], "boot_id": boot_id,
            "transition_time": condition["lastTransitionTime"], "observed_at": observed,
            "configuration": {
                "vm_size": pool.get("vmSize"),
                "node_image_version": node["metadata"].get("labels", {}).get(
                    "kubernetes.azure.com/node-image-version", pool.get("nodeImageVersion"),
                ),
                "os_sku": pool.get("osSku"),
                "kubernetes_version": node["status"]["nodeInfo"].get("kubeletVersion"),
                "network_plugin": profile.get("networkPlugin"),
                "network_policy": profile.get("networkPolicy"),
                "network_dataplane": profile.get("networkDataplane"),
            },
        }
        save(args.output / "readiness.json", readiness)
        ssh = ["ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=yes", "-o", "ConnectTimeout=15"]
        if args.ssh_config:
            ssh += ["-F", str(args.ssh_config)]
        ssh += ["--", args.ssh_user + "@" + ip]
        command([*ssh, "sudo -n systemctl stop aks-node-boot-trace.service"], timeout=100)
        archive = args.output / "capture.tar"
        fetch(
            ssh, "sudo -n tar -C /var/log/aks-node-boot-trace -cf - " + boot_id,
            archive, args.max_mib * 1024 * 1024,
        )
        unpack(archive, args.output, boot_id, args.max_mib * 1024 * 1024)
        metadata = json.loads((args.output / "metadata.json").read_text())
        if metadata["boot_id"] != boot_id or metadata["backend"] != args.backend:
            raise ValueError("Guest capture boot/backend does not match the requested trial")
        if metadata.get("status") != "stopped" or "finalized_at" not in metadata:
            raise RuntimeError("Guest capture did not finalize successfully; inspect metadata.json")
        archive.unlink()
        save(args.output / "result.json", {"status": "collected", "readiness": readiness})
        print(f"Capture collected in {args.output}. The node remains allocated.")
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        save(args.output / "result.json", {
            "status": "incomplete", "error": str(error),
            "note": "Scale-out may have started; no automatic rollback. Check Azure and the guest watchdog.",
        })
        raise
    finally:
        watch.close()
