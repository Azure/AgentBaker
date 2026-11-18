"""Bounded boot diagnostics; never read workload file contents."""

import argparse
import datetime
import json
import math
from pathlib import Path


CSE_EVENTS = Path("/var/log/azure/Microsoft.Azure.Extensions.CustomScript/events")
MAX_CSE_EVENT_FILES = 2048
MAX_CSE_EVENT_FILE_BYTES = 16 * 1024
MAX_CSE_TASKS = 512


def instrument_cloud(path):
    """Instrument only the experimental image's copy, preserving command status."""
    text = path.read_text()
    start = '    local startTime\n'
    end = '    local endTime\n'
    if "aks_boot_trace_start" in text or text.count(start) != 1 or text.count(end) != 1:
        raise ValueError("Unexpected init-aks-cloud.sh timing wrapper; refusing to instrument")
    update = '[ $rc -eq 0 ] && { update-ca-certificates || rc=$?; }'
    if text.count(update) != 2 or text.count("function install_certs_to_trust_store {") != 1:
        raise ValueError("Unexpected CA update calls; refusing to instrument")
    text = text.replace(update, '[ $rc -eq 0 ] && { aks_boot_trace_update_ca || rc=$?; }')
    text = text.replace("function install_certs_to_trust_store {", """aks_boot_trace_skip_ca_update() {
    echo "WARNING: EXPERIMENT ONLY: update-ca-certificates skipped; downloaded CAs are NOT activated" >&2
}

aks_boot_trace_update_ca() {
    local mode="enabled"
    if [ -f /etc/aks-node-boot-trace-ca-update-mode ]; then
        read -r mode < /etc/aks-node-boot-trace-ca-update-mode || {
            echo "ERROR: could not read boot-trace CA experiment mode" >&2
            return 1
        }
    fi
    case "$mode" in
        enabled)
            logs_to_events "AKS.CSE.bootTrace.updateCaCertificates" update-ca-certificates
            ;;
        rust)
            logs_to_events "AKS.CSE.bootTrace.updateCaCertificates" /opt/aks-node-boot-trace/update-ca-certificates-rs
            ;;
        skip-update-ca-certificates)
            logs_to_events "AKS.CSE.bootTrace.skippedUpdateCaCertificates" aks_boot_trace_skip_ca_update
            ;;
        *)
            echo "ERROR: invalid boot-trace CA experiment mode: $mode" >&2
            return 1
            ;;
    esac
}

function install_certs_to_trust_store {""", 1)
    text = text.replace(start, """    local aks_boot_trace_start="" aks_boot_trace_idle=""
    read -r aks_boot_trace_start aks_boot_trace_idle < /proc/uptime ||
        echo "WARNING: boot-trace could not sample certificate start time" >&2
""" + start)
    text = text.replace(end, """    local aks_boot_trace_end="" aks_boot_trace_boot="" aks_boot_trace_directory=""
    read -r aks_boot_trace_end aks_boot_trace_idle < /proc/uptime ||
        echo "WARNING: boot-trace could not sample certificate end time" >&2
    case "$task" in
        AKS.CSE.rcv1p.isOptedIn|AKS.CSE.rcv1p.retrieveLegacyCerts|AKS.CSE.rcv1p.retrieveCerts|AKS.CSE.rcv1p.installCertsToTrustStore|AKS.CSE.bootTrace.updateCaCertificates|AKS.CSE.bootTrace.skippedUpdateCaCertificates)
            if [ -n "$aks_boot_trace_start" ] && [ -n "$aks_boot_trace_end" ]; then
                if read -r aks_boot_trace_boot < /proc/sys/kernel/random/boot_id; then
                    aks_boot_trace_directory="/var/log/aks-node-boot-trace/$aks_boot_trace_boot"
                    if [ -d "$aks_boot_trace_directory" ] ||
                        install -d -m 0700 /var/log/aks-node-boot-trace "$aks_boot_trace_directory"; then
                        printf '{"task":"%s","start":%s,"end":%s,"exit_status":%s}\\n' \\
                            "$task" "$aks_boot_trace_start" "$aks_boot_trace_end" "$ret" \\
                            >> "$aks_boot_trace_directory/cloud-timings.jsonl" ||
                            echo "WARNING: boot-trace could not save certificate timing" >&2
                    else
                        echo "WARNING: boot-trace could not create certificate timing directory" >&2
                    fi
                else
                    echo "WARNING: boot-trace could not read certificate boot identity" >&2
                fi
            fi
            ;;
    esac
""" + end)
    path.write_text(text)


class CseTaskSampler:
    """Retain bounded CSE task names and intervals, never event messages."""

    def __init__(self, warnings, realtime_origin, directory=CSE_EVENTS):
        self.warnings = warnings
        self.realtime_origin = realtime_origin
        self.directory = directory
        self.processed = set()
        self.read_failures = {}
        self.tasks = []
        self.reported = set()

    def warn(self, message):
        if message not in self.reported:
            self.reported.add(message)
            self.warnings.append(message)

    def timestamp(self, value):
        if not isinstance(value, str):
            raise ValueError("CSE event timestamp is not a string")
        try:
            return datetime.datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()
        except ValueError as error:
            raise ValueError("CSE event timestamp is invalid") from error

    def sample(self):
        try:
            paths = sorted(self.directory.glob("*.json"))
        except OSError:
            self.warn("CSE event files are not readable.")
            return
        pending = [path for path in paths if path.name not in self.processed]
        if len(self.processed) >= MAX_CSE_EVENT_FILES:
            if pending:
                self.warn(f"CSE event sampling was limited to {MAX_CSE_EVENT_FILES} files.")
            return
        if len(pending) + len(self.processed) > MAX_CSE_EVENT_FILES:
            pending = pending[:MAX_CSE_EVENT_FILES - len(self.processed)]
            self.warn(f"CSE event sampling was limited to {MAX_CSE_EVENT_FILES} files.")
        for path in pending:
            try:
                size = path.stat().st_size
                if size > MAX_CSE_EVENT_FILE_BYTES:
                    self.processed.add(path.name)
                    self.warn(
                        f"CSE event file {path.name} exceeded the "
                        f"{MAX_CSE_EVENT_FILE_BYTES}-byte limit."
                    )
                    continue
                event = json.loads(path.read_text(encoding="utf-8"))
            except json.JSONDecodeError:
                self.read_failures[path.name] = self.read_failures.get(path.name, 0) + 1
                if self.read_failures[path.name] >= 3:
                    self.processed.add(path.name)
                    self.warn(f"CSE event file {path.name} could not be parsed.")
                continue
            except (OSError, UnicodeDecodeError):
                self.read_failures[path.name] = self.read_failures.get(path.name, 0) + 1
                if self.read_failures[path.name] >= 3:
                    self.processed.add(path.name)
                    self.warn(f"CSE event file {path.name} could not be read.")
                continue

            self.processed.add(path.name)
            if not isinstance(event, dict):
                continue
            task = event.get("TaskName")
            if not isinstance(task, str) or not task.startswith("AKS.CSE."):
                continue
            if len(task) > 256:
                self.warn(f"CSE task name in {path.name} exceeded the 256-character limit.")
                continue
            if len(self.tasks) >= MAX_CSE_TASKS:
                self.warn(f"CSE task sampling was limited to {MAX_CSE_TASKS} intervals.")
                return
            try:
                start = self.timestamp(event.get("Timestamp")) - self.realtime_origin
                end = self.timestamp(event.get("OperationId")) - self.realtime_origin
            except (OverflowError, OSError, ValueError):
                self.warn(f"CSE task timestamps in {path.name} are invalid.")
                continue
            if not math.isfinite(start) or not math.isfinite(end) or end < start:
                self.warn(f"CSE task interval in {path.name} is invalid.")
                continue
            self.tasks.append({"task": task, "start": start, "end": end})

    def result(self):
        return sorted(self.tasks, key=lambda task: (task["start"], task["task"]))


class ContainerdWaitSampler:
    """Sample D-state threads, not all syscalls or process arguments."""

    def __init__(self, warnings, proc=Path("/proc")):
        self.proc = proc
        self.warnings = warnings
        self.processes = {}
        self.active = {}
        self.waits = []
        self.files = []
        self.next_scan = 0
        self.reported = set()

    def warn(self, message):
        if message not in self.reported:
            self.reported.add(message)
            self.warnings.append(message)

    def read(self, path, limit=8192):
        try:
            with path.open() as source:
                text = source.read(limit + 1)
            if len(text) > limit:
                self.warn(f"Containerd diagnostic {path.name} was truncated.")
            return text[:limit]
        except (FileNotFoundError, ProcessLookupError):
            return None
        except PermissionError:
            self.warn(f"Containerd diagnostic {path.name} is not readable.")
            return None

    def file_table(self, process, now):
        rows = []
        try:
            descriptors = sorted((process / "fd").iterdir())
            for descriptor in descriptors[:128]:
                try:
                    info = descriptor.stat()
                    rows.append({
                        "fd": int(descriptor.name), "path": str(descriptor.readlink()),
                        "inode": info.st_ino, "device": info.st_dev,
                    })
                except (FileNotFoundError, ProcessLookupError):
                    continue
            if len(descriptors) > 128:
                self.warn("Containerd descriptor snapshot was limited to 128 entries.")
        except (FileNotFoundError, ProcessLookupError):
            return
        except PermissionError:
            self.warn("Containerd descriptors are not readable.")
        self.files.append({"pid": int(process.name), "sample_monotonic": now, "descriptors": rows})

    def sample(self, now):
        if len(self.waits) >= 128:
            self.warn("Containerd wait diagnostics reached their 128-interval limit.")
            return
        if not self.processes and now >= self.next_scan:
            self.next_scan = now + 1
            for process in self.proc.glob("[0-9]*"):
                comm = self.read(process / "comm", 64)
                if comm is not None and comm.strip() == "containerd":
                    stat = self.read(process / "stat")
                    if stat is not None:
                        self.processes[process] = stat.rsplit(")", 1)[1].split()[19]
        observed = set()
        for process, identity in list(self.processes.items()):
            stat = self.read(process / "stat")
            if stat is None or stat.rsplit(")", 1)[1].split()[19] != identity:
                del self.processes[process]
                continue
            try:
                tasks = list((process / "task").iterdir())
            except (FileNotFoundError, ProcessLookupError):
                continue
            except PermissionError:
                self.warn("Containerd threads are not readable.")
                continue
            sampled_files = False
            for task in tasks:
                stat = self.read(task / "stat")
                if stat is None or stat.rsplit(")", 1)[1].split()[0] != "D":
                    continue
                key = (process.name, task.name, stat.rsplit(")", 1)[1].split()[19])
                observed.add(key)
                if key in self.active:
                    self.active[key]["last_observed_monotonic"] = now
                    continue
                if len(self.waits) >= 128:
                    self.warn("Containerd wait diagnostics reached their 128-interval limit.")
                    break
                row = {
                    "pid": int(process.name), "tid": int(task.name),
                    "first_observed_monotonic": now, "last_observed_monotonic": now,
                    "kernel_stack": self.read(task / "stack"),
                    "wait_channel": self.read(task / "wchan"),
                }
                syscall = self.read(task / "syscall")
                if syscall:
                    fields = syscall.split()
                    # x86_64 read/pread64/readv/preadv/preadv2/fsync/fdatasync:
                    # export only the syscall number and fd, never buffers/arguments.
                    if len(fields) >= 2 and fields[0] in ("0", "17", "19", "295", "327", "74", "75"):
                        row["syscall"] = int(fields[0])
                        row["fd"] = int(fields[1], 0)
                self.waits.append(row)
                self.active[key] = row
                if not sampled_files:
                    self.file_table(process, now)
                    sampled_files = True
        self.active = {key: value for key, value in self.active.items() if key in observed}

    def result(self):
        return {
            "schema": 1, "sample_interval_seconds": 0.2,
            "note": "Sampled D-state intervals, not exact I/O latencies. Short waits can be missed. "
                    "File descriptors are snapshots, not proof of which file caused a block request.",
            "waits": self.waits, "file_tables": self.files,
        }


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--instrument-cloud", type=Path, required=True)
    instrument_cloud(parser.parse_args().instrument_cloud)
