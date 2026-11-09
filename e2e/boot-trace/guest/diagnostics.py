"""Bounded boot diagnostics; never read workload file contents."""

import argparse
from pathlib import Path


def instrument_cloud(path):
    """Instrument only the experimental image's copy, preserving command status."""
    text = path.read_text()
    start = '    local startTime\n'
    end = '    local endTime\n'
    if "aks_boot_trace_start" in text or text.count(start) != 1 or text.count(end) != 1:
        raise ValueError("Unexpected init-aks-cloud.sh timing wrapper; refusing to instrument")
    text = text.replace(start, """    local aks_boot_trace_start="" aks_boot_trace_idle=""
    read -r aks_boot_trace_start aks_boot_trace_idle < /proc/uptime ||
        echo "WARNING: boot-trace could not sample certificate start time" >&2
""" + start)
    text = text.replace(end, """    local aks_boot_trace_end="" aks_boot_trace_boot="" aks_boot_trace_directory=""
    read -r aks_boot_trace_end aks_boot_trace_idle < /proc/uptime ||
        echo "WARNING: boot-trace could not sample certificate end time" >&2
    case "$task" in
        AKS.CSE.rcv1p.isOptedIn|AKS.CSE.rcv1p.retrieveLegacyCerts|AKS.CSE.rcv1p.retrieveCerts|AKS.CSE.rcv1p.installCertsToTrustStore)
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
