"""Adapters for ftrace text/trace.dat. Times are boot seconds."""

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
