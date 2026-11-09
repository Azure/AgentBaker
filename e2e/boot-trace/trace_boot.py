#!/usr/bin/env python3
"""Collect a first-boot trial or render local captures. Never builds an image."""

import argparse
import json
import os
from pathlib import Path
import subprocess
import sys

from orchestrate import scale_and_collect
from milestones import import_cse
from report import write_html
from timeline import analyze


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="action", required=True)
    collect = commands.add_parser(
        "collect", help="Read-only preflight by default; --execute explicitly scales +1",
    )
    collect.add_argument("--subscription", required=True)
    collect.add_argument("--resource-group", required=True)
    collect.add_argument("--cluster", required=True)
    collect.add_argument("--pool", required=True)
    collect.add_argument("--kubeconfig", required=True, type=Path)
    collect.add_argument("--context", required=True)
    collect.add_argument("--ssh-user", required=True)
    collect.add_argument("--ssh-config", type=Path)
    collect.add_argument("--backend", choices=("ftrace", "lttng"), required=True)
    collect.add_argument("--output", type=Path, required=True)
    collect.add_argument("--timeout", type=int, default=900)
    collect.add_argument("--max-mib", type=int, default=512)
    collect.add_argument("--execute", action="store_true")
    collect.add_argument("--instrumentation-confirmed", action="store_true")
    render = commands.add_parser("report", help="Decode one local capture into HTML and JSON")
    render.add_argument("bundle", type=Path)
    render.add_argument("--output", type=Path, required=True)
    render.add_argument("--ctf-offset-seconds", type=float)
    cse = commands.add_parser("import-cse", help="Convert an exported CSE BootDatapoints object")
    cse.add_argument("input", type=Path)
    cse.add_argument("--output", type=Path, required=True, help="Typically BUNDLE/milestones.json")
    cse.add_argument("--timezone-offset", help="Explicit +/-HH:MM for ambiguous source timezone names")
    compare = commands.add_parser("compare", help="Compare two normalized report JSON files")
    compare.add_argument("baseline", type=Path)
    compare.add_argument("candidate", type=Path)
    compare.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        os.umask(0o077)
        if args.action == "collect":
            scale_and_collect(args)
        elif args.action == "report":
            result = analyze(args.bundle, args.ctf_offset_seconds)
            args.output.mkdir(parents=True, mode=0o700, exist_ok=False)
            (args.output / "report.json").write_text(
                json.dumps(result, indent=2, allow_nan=False) + "\n", encoding="utf-8",
            )
            write_html(args.output / "report.html", [result], ["Capture"])
            print(args.output / "report.html")
        elif args.action == "import-cse":
            result = import_cse(
                json.loads(args.input.read_text(encoding="utf-8")), args.timezone_offset,
            )
            with args.output.open("x", encoding="utf-8") as output:
                output.write(json.dumps(result, indent=2, allow_nan=False) + "\n")
            print(args.output)
        else:
            results = [
                json.loads(path.read_text(encoding="utf-8"))
                for path in (args.baseline, args.candidate)
            ]
            if any(result.get("schema") != 1 for result in results):
                raise ValueError("Unsupported normalized report schema")
            args.output.mkdir(parents=True, mode=0o700, exist_ok=False)
            write_html(args.output / "comparison.html", results, ["Baseline", "Candidate"])
            print(args.output / "comparison.html")
    except (OSError, ValueError, KeyError, RuntimeError, subprocess.SubprocessError) as error:
        print(f"ERROR: {error}", file=sys.stderr)
        if isinstance(error, subprocess.CalledProcessError) and error.stderr:
            print(error.stderr, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
