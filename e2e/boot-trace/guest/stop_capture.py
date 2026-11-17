#!/usr/bin/env python3
"""Systemd safety stop for this boot's private trace; never touches other sessions."""

import json
import os
from pathlib import Path


def main():
    if os.geteuid() != 0:
        raise PermissionError("Safety stop requires root")
    boot_id = Path("/proc/sys/kernel/random/boot_id").read_text().strip()
    path = Path("/var/log/aks-node-boot-trace") / boot_id / "metadata.json"
    if not path.exists():
        print("No capture metadata exists; stopping boot ftrace if configured.")
        if os.environ.get("BACKEND") == "ftrace":
            Path("/sys/kernel/tracing/tracing_on").write_text("0")
        return
    metadata = json.loads(path.read_text())
    if metadata["backend"] != "ftrace":
        raise ValueError("Safety stop only supports ftrace")
    Path("/sys/kernel/tracing/tracing_on").write_text("0")


if __name__ == "__main__":
    main()
