#!/usr/bin/env python3
"""Systemd safety stop for this boot's private trace; never touches other sessions."""

import json
import os
from pathlib import Path
import subprocess


def main():
    if os.geteuid() != 0:
        raise PermissionError("Safety stop requires root")
    boot_id = Path("/proc/sys/kernel/random/boot_id").read_text().strip()
    path = Path("/var/log/aks-node-boot-trace") / boot_id / "metadata.json"
    if not path.exists():
        print("No capture metadata exists; no owned LTTng session to stop.")
        if os.environ.get("BACKEND") == "ftrace":
            Path("/sys/kernel/tracing/tracing_on").write_text("0")
        return
    metadata = json.loads(path.read_text())
    if metadata["backend"] == "ftrace":
        Path("/sys/kernel/tracing/tracing_on").write_text("0")
    elif metadata.get("session_name"):
        session = "aks-boot-" + boot_id
        if metadata["session_name"] != session:
            raise ValueError("Unexpected LTTng session identity")
        if "finalized_at" not in metadata:
            subprocess.run(["lttng", "stop", session], check=True, timeout=30)
            subprocess.run(["lttng", "destroy", session], check=True, timeout=30)
            print("Safety-stopped unfinalized capture; inspect metadata before import.")


if __name__ == "__main__":
    main()
