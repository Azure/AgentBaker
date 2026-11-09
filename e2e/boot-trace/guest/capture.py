#!/usr/bin/env python3
"""Root-only guest recorder for a dedicated, pre-instrumented test image."""

import argparse
import datetime
import json
import os
from pathlib import Path
import re
import signal
import socket
import subprocess
import sys
import time

if __package__:
    from .diagnostics import ContainerdWaitSampler
else:
    from diagnostics import ContainerdWaitSampler


ROOT = Path("/var/log/aks-node-boot-trace")
TRACEFS = Path("/sys/kernel/tracing")
JOURNAL_UNITS = (
    ("cloud-init-local.service",),
    ("cloud-init.service", "cloud-init-network.service"),
    ("cloud-config.service",),
    ("cloud-final.service",),
    ("walinuxagent.service", "waagent.service"),
    ("containerd.service",),
    ("kubelet.service",),
    ("cloud-init-main.service",),
    ("localdns.service",),
)
JOURNAL_UNIT_NAMES = tuple(unit for group in JOURNAL_UNITS for unit in group)
SYSTEMD_TIMESTAMP_PROPERTIES = (
    "ExecMainStartTimestampMonotonic", "ActiveEnterTimestampMonotonic",
    "InactiveEnterTimestampMonotonic", "ExecMainExitTimestampMonotonic",
)
SYSTEMD_PROPERTIES = (
    "Id", "LoadState", "ActiveState", "SubState", "Type", "RemainAfterExit",
    *SYSTEMD_TIMESTAMP_PROPERTIES,
)


def run(*args, timeout=30):
    try:
        return subprocess.run(
            args, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            text=True, timeout=timeout, env={**os.environ, "LC_ALL": "C"},
        ).stdout
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f"{args[0]} failed: {error.stderr.strip()}") from error


def save(path, value):
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, indent=2) + "\n")
    temporary.replace(path)


def clock_sample():
    before = time.monotonic()
    realtime = time.time()
    after = time.monotonic()
    return {
        "monotonic": (before + after) / 2, "realtime": realtime,
        "sample_span_seconds": after - before,
    }


def disk_size(path):
    return sum(item.stat().st_size for item in path.rglob("*") if item.is_file())


def systemd_services(diagnostics):
    rows = {}
    listed = run(
        "systemctl", "list-units", "--all", "--type=service",
        "--no-legend", "--plain", "--full",
    )
    units = sorted({
        line.split(maxsplit=1)[0]
        for line in listed.splitlines()
        if line.split() and line.split()[0].endswith(".service")
    })
    for unit in units:
        try:
            result = subprocess.run(
                ["systemctl", "show", unit, *(f"--property={name}" for name in SYSTEMD_PROPERTIES)],
                check=False, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                text=True, timeout=30, env={**os.environ, "LC_ALL": "C"},
            )
        except subprocess.TimeoutExpired:
            diagnostics.append(f"Timed out reading systemd service metadata for {unit}.")
            continue
        values = dict(
            line.split("=", 1) for line in result.stdout.splitlines() if "=" in line
        )
        if result.returncode != 0:
            message = result.stderr.strip() or f"exit status {result.returncode}"
            diagnostics.append(f"Could not read systemd service metadata for {unit}: {message}")
            continue
        missing = [name for name in SYSTEMD_PROPERTIES if name not in values]
        if missing:
            diagnostics.append(
                f"Systemd metadata for {unit} omitted properties: {', '.join(missing)}."
            )
            continue
        if values.get("LoadState") == "not-found":
            continue
        timestamps = {}
        for name in SYSTEMD_TIMESTAMP_PROPERTIES:
            value = values[name]
            if not value.isdecimal():
                raise ValueError(f"Invalid {name} value for systemd service {unit}: {value!r}")
            timestamps[name] = value
        if not any(value != "0" for value in timestamps.values()) and values.get("ActiveState") != "active":
            continue
        unit_id = values["Id"] or unit
        rows[unit_id] = {
            "name": unit_id,
            "active_state": values["ActiveState"],
            "sub_state": values["SubState"],
            "type": values["Type"],
            "remain_after_exit": values["RemainAfterExit"],
            **timestamps,
        }
    return [rows[name] for name in sorted(rows)]


def start_ftrace(destination, metadata):
    command = [
        "trace-cmd", "record", "-K", "-a", "--no-filter", "--poll",
        "-s", "10000", "--compression", "none", "--file-version", "6",
        "-o", str(destination / "trace.dat"),
    ]
    # -K preserves boot events; -k alone would erase them before recording.
    with (destination / "trace-cmd.log").open("wb") as log:
        process = subprocess.Popen(
            ["stdbuf", "-oL", *command], stdout=log, stderr=subprocess.STDOUT,
            env={**os.environ, "LC_ALL": "C"}, start_new_session=True,
        )
    metadata.update({
        "ftrace_format": "trace-cmd", "recorder_pid": process.pid,
        "supervisor_pid": os.getpid(), "recorder_command": command,
    })
    return process


def wait_ftrace_ready(process, destination):
    cpus = [path.name for path in (TRACEFS / "per_cpu").glob("cpu[0-9]*")]
    if not cpus:
        raise RuntimeError("No per-CPU ftrace buffers found")
    deadline = time.monotonic() + 30
    while time.monotonic() < deadline:
        if process.poll() is not None:
            raise RuntimeError("trace-cmd exited during setup; inspect trace-cmd.log")
        log = (destination / "trace-cmd.log").read_bytes()
        if (
            b"Hit Ctrl^C to stop recording" in log
            and all((destination / f"trace.dat.{cpu}").is_file() for cpu in cpus)
        ):
            return
        time.sleep(0.05)
    raise RuntimeError("trace-cmd readers were not ready within 30 seconds; inspect trace-cmd.log")


def stop_ftrace(process):
    try:
        if process.poll() is None:
            process.send_signal(signal.SIGINT)
        status = process.wait(timeout=60)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        process.wait(timeout=5)
        raise RuntimeError("trace-cmd did not finish draining within 60 seconds") from None
    if status != 0:
        # A failed parent can leave readers that ignore SIGTERM/SIGINT.
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        raise RuntimeError(f"trace-cmd exited with status {status}; inspect trace-cmd.log")


def record(args):
    if os.geteuid() != 0:
        raise PermissionError("Capture requires root on a dedicated test node")
    os.umask(0o077)
    if not 0 < args.max_seconds <= 600 or args.max_mib < 16:
        raise ValueError("Duration must be 1..600 seconds, with at least 16 MiB")
    boot_id = Path("/proc/sys/kernel/random/boot_id").read_text().strip()
    destination = ROOT / boot_id
    destination.mkdir(parents=True, mode=0o700, exist_ok=True)
    metadata = {
        "schema": 1, "backend": args.backend, "boot_id": boot_id,
        "kernel": os.uname().release, "architecture": os.uname().machine,
        "clock_start": clock_sample(), "status": "recording",
        "image_label": args.image_label,
        "warnings": [
            "Capture starts before selected userspace services, not at kernel time zero.",
            "Service activation timestamps are not full application initialization.",
        ],
    }
    image_manifest = Path("/opt/aks-node-boot-trace/image.txt")
    if image_manifest.is_file():
        metadata["image_provenance"] = dict(
            line.split("=", 1) for line in image_manifest.read_text().splitlines() if "=" in line
        )
    else:
        metadata["warnings"].append("No baked image provenance manifest is available.")
    metadata["warnings"].append(
        "Type=notify service ordering delays selected boot services until the recorder is ready. "
        "AgentBaker E2E also opens a Bastion/SSH connection during boot. "
        "No uninstrumented control is measured by this trial."
    )
    metadata["warnings"].append(
        "Containerd D-state threads are sampled every 200 ms, with a 1 s discovery interval. "
        "This adds /proc reads and CPU overhead; snapshots are saved only after tracing stops. "
        "Short waits can be missed; descriptor snapshots do not prove block-request attribution."
    )
    # Certificate timings can precede this service; a second recorder must not.
    with (destination / "metadata.json").open("x") as initial:
        initial.write(json.dumps(metadata, indent=2) + "\n")
    stopping = False

    def request_stop(_signum, _frame):
        nonlocal stopping
        stopping = True

    signal.signal(signal.SIGTERM, request_stop)
    signal.signal(signal.SIGINT, request_stop)
    recorder = None
    session_created = False
    failed = False
    wait_sampler = ContainerdWaitSampler(metadata["warnings"])
    try:
        if args.backend == "ftrace":
            if not (TRACEFS / "tracing_on").exists():
                raise RuntimeError("tracefs is not mounted at /sys/kernel/tracing")
            if (TRACEFS / "tracing_on").read_text().strip() != "1":
                raise RuntimeError("ftrace must already be enabled by boot configuration")
            trace_clock = (TRACEFS / "trace_clock").read_text()
            if "[mono]" not in trace_clock:
                raise RuntimeError("ftrace must use trace_clock=mono")
            metadata["ftrace_clock"] = trace_clock.strip()
            metadata["enabled_events"] = (TRACEFS / "set_event").read_text().splitlines()
            if not any("sched_switch" in event for event in metadata["enabled_events"]):
                raise RuntimeError("sched_switch was not enabled before this boot")
            if (TRACEFS / "current_tracer").read_text().strip() != "nop":
                raise RuntimeError("This experiment expects event tracing with current_tracer=nop")
            recorder = start_ftrace(destination, metadata)
            wait_ftrace_ready(recorder, destination)
            metadata["warnings"].append(
                "Binary ftrace collection uses trace-cmd -K without resetting boot buffers. "
                "The size watchdog is sampled; final draining may overshoot. "
                "Combining per-CPU files temporarily needs approximately twice their disk space."
            )
        else:
            if (
                (TRACEFS / "tracing_on").exists()
                and (TRACEFS / "tracing_on").read_text().strip() == "1"
                and (
                    (TRACEFS / "set_event").read_text().strip()
                    or (TRACEFS / "current_tracer").read_text().strip() != "nop"
                )
            ):
                raise RuntimeError("Disable boot ftrace before running the LTTng-only trial")
            # Only one private session; never stop or destroy unrelated sessions.
            session = "aks-boot-" + boot_id
            metadata["session_name"] = session
            save(destination / "metadata.json", metadata)
            run("lttng", "create", session, "--output=" + str(destination / "ctf"))
            session_created = True
            run(
                "lttng", "enable-channel", "--kernel", "--session", session,
                "--subbuf-size=4M", "--num-subbuf=16", "--switch-timer=0",
                "--read-timer=20000",
                "--discard", "--tracefile-size=0", "boot",
            )
            metadata["lttng_channel"] = {
                "subbuffer_bytes": 4 * 1024 * 1024, "subbuffer_count": 16,
                "cpu_count": os.cpu_count(),
                "buffer_bytes": 64 * 1024 * 1024 * (os.cpu_count() or 1),
                "switch_timer_us": 0, "read_timer_us": 20000,
                "mode": "discard", "tracefile_size": 0,
            }
            events = (
                "sched_switch,sched_wakeup,sched_wakeup_new,"
                "sched_process_fork,sched_process_exit,sched_process_exec,"
                "block_rq_issue,block_rq_complete,"
                "net_dev_xmit,net_if_receive_skb_entry,"
                "net_if_receive_skb_list_entry,net_if_rx_entry"
            )
            run(
                "lttng", "enable-event", "--kernel", "--session", session,
                "--channel=boot", events,
            )
            metadata["enabled_events"] = events.split(",")
            run(
                "lttng", "add-context", "--kernel", "--session", session,
                "--channel=boot", "--type=pid", "--type=tid", "--type=procname",
            )
            run("lttng", "start", session)
            metadata["warnings"].append(
                "LTTng starts after session setup; early kernel events are not covered. "
                "The size watchdog is sampled and may overshoot while buffers flush. "
                "Use a bounded dedicated filesystem for a hard disk-space cap. "
                "Larger buffers reduce burst loss but do not guarantee loss-free capture."
            )
        metadata["recorder_active_monotonic"] = time.monotonic()
        save(destination / "metadata.json", metadata)
        notify = os.environ.get("NOTIFY_SOCKET")
        if notify:
            address = "\0" + notify[1:] if notify.startswith("@") else notify
            with socket.socket(socket.AF_UNIX, socket.SOCK_DGRAM) as channel:
                channel.connect(address)
                channel.sendall(b"READY=1")
        deadline = time.monotonic() + args.max_seconds
        limit = args.max_mib * 1024 * 1024
        while not stopping:
            if recorder is not None and recorder.poll() is not None:
                raise RuntimeError("trace-cmd exited before stop was requested; inspect trace-cmd.log")
            if time.monotonic() >= deadline:
                metadata["warnings"].append("Capture stopped by duration watchdog.")
                break
            if disk_size(destination) >= limit:
                metadata["warnings"].append("Capture stopped by size watchdog.")
                break
            wait_sampler.sample(time.monotonic())
            time.sleep(0.2)
        metadata["status"] = "stopped"
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        failed = True
        metadata["status"] = "failed"
        metadata["error"] = str(error)
        raise
    finally:
        metadata["clock_end"] = clock_sample()
        save(destination / "metadata.json", metadata)
        try:
            if args.backend == "ftrace" and recorder is not None:
                (TRACEFS / "tracing_on").write_text("0")
                stop_ftrace(recorder)
                if not (destination / "trace.dat").is_file():
                    raise RuntimeError("trace-cmd did not produce trace.dat")
                if disk_size(destination) >= args.max_mib * 1024 * 1024:
                    metadata["warnings"].append("Final ftrace archive reached the sampled size limit.")
                stats = {
                    str(path.relative_to(TRACEFS)): path.read_text()
                    for path in TRACEFS.glob("per_cpu/cpu*/stats")
                }
                save(destination / "ftrace-stats.json", stats)
            elif session_created:
                run("lttng", "stop", session)
                try:
                    metadata["lttng_status"] = run("lttng", "list", session)
                    loss = re.findall(r"Discarded events:\s*(\d+)", metadata["lttng_status"])
                    if not loss:
                        raise RuntimeError("LTTng did not expose its discarded-event counters")
                    metadata["lttng_discarded_events"] = sum(map(int, loss))
                    if metadata["lttng_discarded_events"]:
                        metadata["warnings"].append(
                            f"LTTng discarded {metadata['lttng_discarded_events']} events; "
                            "scheduler accounting is partial, not a loss-free measurement."
                        )
                finally:
                    run("lttng", "destroy", session)
            if not failed:
                save(destination / "containerd-waits.json", wait_sampler.result())
                save(destination / "services.json", systemd_services(metadata["warnings"]))
                if args.include_journal:
                    journal = run(
                        "journalctl", "--boot=0", "--no-pager", "--output=json",
                        "--lines=10000", *("--unit=" + unit for unit in JOURNAL_UNIT_NAMES),
                    )
                    (destination / "journal.jsonl").write_text(journal)
            metadata["finalized_at"] = datetime.datetime.now(
                datetime.timezone.utc,
            ).isoformat()
        except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
            metadata["status"] = "failed"
            metadata["error"] = "Finalization failed: " + str(error)
            raise
        finally:
            save(destination / "metadata.json", metadata)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--backend", choices=("ftrace", "lttng"), required=True)
    parser.add_argument("--max-seconds", type=int, default=600)
    parser.add_argument("--max-mib", type=int, default=256)
    parser.add_argument("--image-label", required=True)
    parser.add_argument("--include-journal", action="store_true")
    args = parser.parse_args()
    try:
        record(args)
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"Capture failed: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
