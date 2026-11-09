"""Adapters for ftrace text/trace.dat and LTTng CTF. Times are boot seconds."""

from contextlib import ExitStack
import heapq
import json
import re
import subprocess
import tempfile


LINE = re.compile(
    r"^\s*(?P<comm>.+)-(?P<pid>\d+)\s+(?:\(\s*[^)]*\)\s+)?"
    r"\[(?P<cpu>\d+)\]\s+(?:\S+\s+)?"
    r"(?P<time>\d+(?:\.\d+)?):\s+(?P<name>[\w:]+):\s*(?P<data>.*)$"
)

NETWORK_EVENT_ALIASES = {
    "net_if_receive_skb_entry": "netif_receive_skb_entry",
    "net_if_receive_skb_list_entry": "netif_receive_skb_list_entry",
    "net_if_rx_entry": "netif_rx_entry",
}


def ftrace_lines(source, diagnostics, *, raw_timestamps=False):
    unmatched = 0
    gap = False
    for line in source:
        if not line.strip() or line.lstrip().startswith("#") or re.fullmatch(r"cpus=\d+\s*", line):
            continue
        if "LOST" in line or "CPU:" in line and "EVENTS" in line:
            diagnostics.append("ftrace reports lost events: " + line.strip())
            gap = True
            continue
        match = LINE.match(line)
        if not match:
            unmatched += 1
            gap = True
            continue
        value = match.groupdict()
        # --raw-ts uses integer nanoseconds for nano-second file clocks, but
        # other trace.dat clocks can still produce decimal seconds.
        timestamp = (
            int(value["time"]) / 1_000_000_000
            if raw_timestamps and "." not in value["time"]
            else float(value["time"])
        )
        yield {
            "t": timestamp,
            "cpu": int(value["cpu"]),
            "name": value["name"].rsplit(":", 1)[-1],
            "comm": value["comm"].strip(),
            "pid": int(value["pid"]),
            "data": value["data"],
            "gap": gap,
        }
        gap = False
    if unmatched:
        diagnostics.append(f"{unmatched} non-header ftrace lines could not be decoded.")


def merged_ftrace_text(path, diagnostics):
    # trace_pipe can interleave CPUs out of timestamp order. Spool each CPU so
    # merging large legacy captures does not retain all events in memory.
    with tempfile.TemporaryDirectory() as directory, ExitStack() as stack:
        source = stack.enter_context(path.open(encoding="utf-8", errors="strict"))
        streams, last_cpu = {}, {}
        previous = None
        inversions = 0
        worst = 0.0
        has_gaps = False
        diagnostic_count = len(diagnostics)
        for sequence, event in enumerate(ftrace_lines(source, diagnostics)):
            cpu, timestamp = event["cpu"], event["t"]
            if cpu in last_cpu and timestamp < last_cpu[cpu]:
                raise ValueError(
                    f"ftrace CPU {cpu} is not chronologically merged: "
                    "per-CPU timestamps moved backwards"
                )
            last_cpu[cpu] = timestamp
            if previous is not None and timestamp < previous:
                inversions += 1
                worst = max(worst, previous - timestamp)
            previous = timestamp
            has_gaps = has_gaps or event["gap"]
            if cpu not in streams:
                streams[cpu] = stack.enter_context(
                    open(f"{directory}/cpu{cpu}.jsonl", "w+", encoding="utf-8")
                )
            streams[cpu].write(json.dumps([sequence, event], allow_nan=False) + "\n")
        if inversions:
            if has_gaps or len(diagnostics) != diagnostic_count:
                raise ValueError(
                    "Cannot merge out-of-order legacy ftrace with lost or undecodable "
                    "records: their gap boundaries would be ambiguous"
                )
            diagnostics.append(
                f"Legacy ftrace merged per-CPU streams: {inversions} ordering inversions "
                f"(up to {worst:.9f}s); no events discarded."
            )
        for stream in streams.values():
            stream.seek(0)
        records = (map(json.loads, stream) for stream in streams.values())
        for _, event in heapq.merge(*records, key=lambda row: (row[1]["t"], row[0])):
            yield event


def ftrace_events(path, diagnostics):
    if path.suffix != ".dat":
        yield from merged_ftrace_text(path, diagnostics)
        return
    # Keep kernel field names/flags, merge CPUs and retain the monotonic clock.
    with tempfile.TemporaryFile(mode="w+t", encoding="utf-8") as errors:
        try:
            process = subprocess.Popen(
                ["trace-cmd", "report", "-N", "-t", "--raw-ts", "-i", str(path)],
                stdout=subprocess.PIPE, stderr=errors, text=True, encoding="utf-8",
            )
        except FileNotFoundError as error:
            raise RuntimeError("Binary ftrace import requires trace-cmd on the report host.") from error
        try:
            yield from ftrace_lines(process.stdout, diagnostics, raw_timestamps=True)
            status = process.wait()
            errors.seek(0)
            message = errors.read(8192).strip()
            if status != 0:
                raise RuntimeError(f"trace-cmd report failed ({status}): {message}")
            if message:
                diagnostics.append("trace-cmd report: " + message)
        finally:
            process.stdout.close()
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)


def lttng_events(path, diagnostics, offset=None, realtime_origin=0.0):
    """Use distro-provided Babeltrace 2 bindings; importing reports needs no bt2."""
    try:
        import bt2
    except ImportError as error:
        raise RuntimeError(
            "LTTng import needs Babeltrace 2 Python bindings (python3-bt2)."
        ) from error

    def fields(field):
        if field is None:
            return {}
        numeric = {"pid", "tid", "vtid", "cpu_id", "prev_tid", "next_tid",
                   "parent_tid", "child_tid", "prev_state"}
        return {
            str(key): str(int(field[key])) if key in numeric else str(field[key])
            for key in field
        }

    first = True
    gap = False
    for message in bt2.TraceCollectionMessageIterator(str(path)):
        if isinstance(message, (bt2._DiscardedEventsMessageConst, bt2._DiscardedPacketsMessageConst)):
            diagnostics.append("LTTng reports discarded events or packets.")
            gap = True
        if not isinstance(message, bt2._EventMessageConst):
            continue
        if first:
            clock = message.default_clock_snapshot.clock_class
            if not clock.origin_is_unix_epoch and offset is None:
                raise ValueError(
                    "CTF clock has no Unix origin; supply --ctf-offset-seconds "
                    "(boot seconds minus CTF seconds)."
                )
            diagnostics.append(
                "CTF time converted to boot time using the supplied clock mapping; "
                "wall-clock changes during capture require additional review."
            )
            first = False
        mapping = offset if offset is not None else -realtime_origin
        event = message.event
        data = fields(event.payload_field)
        context = fields(event.common_context_field)
        packet = fields(event.packet.context_field) if event.packet else {}
        name = event.name.rsplit(":", 1)[-1]
        name = NETWORK_EVENT_ALIASES.get(name, name)
        if name == "sched_switch":
            payload = (
                f"prev_comm={data['prev_comm']} prev_pid={data['prev_tid']} "
                f"prev_state={data['prev_state']} ==> "
                f"next_comm={data['next_comm']} next_pid={data['next_tid']}"
            )
        elif name in ("sched_waking", "sched_wakeup", "sched_wakeup_new"):
            payload = f"comm={data['comm']} pid={data['tid']}"
        elif name == "sched_process_exit":
            payload = f"comm={data['comm']} pid={data['tid']}"
        elif name == "sched_process_fork":
            payload = (
                f"comm={data['parent_comm']} pid={data['parent_tid']} "
                f"child_comm={data['child_comm']} child_pid={data['child_tid']}"
            )
        else:
            payload = " ".join(f"{key}={value}" for key, value in data.items())
        if name.startswith("sched_") and "cpu_id" not in packet:
            raise ValueError("CTF scheduler event has no packet cpu_id; cannot account CPU time")
        yield {
            "t": message.default_clock_snapshot.ns_from_origin / 1e9 + mapping,
            "cpu": int(packet.get("cpu_id", -1)),
            "name": name,
            "comm": context.get("procname", ""),
            "pid": int(context.get("vtid", context.get("tid", 0))),
            "data": payload,
            "gap": gap,
        }
        gap = False
