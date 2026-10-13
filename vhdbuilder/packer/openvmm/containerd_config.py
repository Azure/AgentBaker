#!/usr/bin/env python3
"""Reconcile only the evaluation image's import; verify the effective configuration."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import tomllib

PREFIX = Path("/opt/kata-openvmm")
CONFIG = Path("/etc/containerd/config.toml")
RECEIPT = Path("/run/kata-openvmm/containerd-config.sha256")


def replace_imports(text, imports):
    """Locate the complete root assignment using TOML parsing, not line regexes."""
    parsed = tomllib.loads(text)
    assignment = "imports = " + json.dumps(imports, ensure_ascii=False) + "\n"
    if "imports" not in parsed:
        return assignment + text
    lines = text.splitlines(keepends=True)
    previous = 0
    for end in range(1, len(lines) + 1):
        try:
            prefix = tomllib.loads("".join(lines[:end]))
        except tomllib.TOMLDecodeError:
            # A multiline TOML value is incomplete until its closing delimiter.
            continue
        if "imports" in prefix:
            return "".join(lines[:previous]) + assignment + "".join(lines[end:])
        previous = end
    raise ValueError("cannot locate the root imports assignment")


def candidate_config(text, prefix=PREFIX):
    config = tomllib.loads(text)
    version = config.get("version")
    if version not in (2, 3, 4):
        raise ValueError("OpenVMM requires an explicit containerd config version 2, 3 or 4")
    imports = config.get("imports", [])
    if not isinstance(imports, list) or not all(isinstance(item, str) for item in imports):
        raise ValueError("containerd imports must be an array of paths")
    directory = prefix / "containerd" / ("v1" if version == 2 else "v2")
    fragment = directory / "openvmm.toml"
    if not fragment.is_file():
        raise ValueError(f"missing image-owned fragment: {fragment}")
    owned = {str(prefix / "containerd" / variant / name)
             for variant in ("v1", "v2") for name in ("*.toml", "openvmm.toml")}
    desired = [item for item in imports if item not in owned] + [str(fragment)]
    return text if imports == desired else replace_imports(text, desired)


def dump_config(binary, config):
    # Capture output: a generated config can contain registry credentials.
    result = subprocess.run([str(binary), "--config", str(config), "config", "dump"],
                            capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise ValueError("containerd could not load/merge the configuration; "
                         "inspect it on the node (output withheld to protect credentials)")
    return tomllib.loads(result.stdout)


def require_subset(actual, expected, path="config"):
    if isinstance(expected, dict):
        if not isinstance(actual, dict):
            raise ValueError(f"{path} must be a table")
        for key, value in expected.items():
            if key not in actual:
                raise ValueError(f"missing {path}.{key}")
            require_subset(actual[key], value, f"{path}.{key}")
    elif isinstance(expected, list) and expected and isinstance(expected[0], dict):
        if not isinstance(actual, list) or len(actual) != len(expected):
            raise ValueError(f"unexpected {path} entries")
        for index, (item, wanted) in enumerate(zip(actual, expected)):
            require_subset(item, wanted, f"{path}[{index}]")
    elif type(actual) is not type(expected) or actual != expected:
        # Never include values here: surrounding config can contain credentials.
        raise ValueError(f"unexpected {path}")


def validate_effective(config, prefix=PREFIX):
    expected = tomllib.loads((prefix / "containerd/v2/openvmm.toml").read_text())
    plugins = config.get("plugins", {})
    require_subset(plugins, expected["plugins"], "plugins")
    required = set(expected["required_plugins"])
    if not required <= set(config.get("required_plugins", [])):
        raise ValueError("missing required EROFS plugins")
    needed = required | {"io.containerd.cri.v1.images", "io.containerd.cri.v1.runtime",
                         "io.containerd.grpc.v1.cri"}
    if needed & set(config.get("disabled_plugins", [])) or "cri" in config.get("disabled_plugins", []):
        raise ValueError("a required EROFS/CRI plugin is disabled")


def fingerprint(config):
    return hashlib.sha256(json.dumps(config, sort_keys=True).encode()).hexdigest() + "\n"


def atomic_write(path, text, mode=0o600, owner=None):
    with tempfile.NamedTemporaryFile(mode="w", dir=path.parent,
                                     prefix=".openvmm-", delete=False) as stream:
        temporary = Path(stream.name)
        try:
            stream.write(text)
            stream.flush()
            if owner is not None:
                os.fchown(stream.fileno(), *owner)
            os.fchmod(stream.fileno(), mode)
            os.fsync(stream.fileno())
            os.replace(temporary, path)
        finally:
            temporary.unlink(missing_ok=True)


def reconcile(config=CONFIG, prefix=PREFIX, receipt=RECEIPT, check=False):
    if config.is_symlink() or not config.is_file():
        raise ValueError("containerd config must be a regular, non-symlink file")
    original = config.read_text()
    candidate = candidate_config(original, prefix)
    if check and candidate != original:
        raise ValueError("OpenVMM import is missing, stale or not last; startup reconciliation required")
    binary = prefix / "bin/containerd"
    # Temporary workaround: live RP CSE can replace the config without this branch's
    # imports. Remove reconciliation once all supported RP/ANC paths own the same
    # OpenVMM contract. Preserve other settings/comments; never substitute a dev-node config.
    if check:
        effective = dump_config(binary, config)
    else:
        with tempfile.NamedTemporaryFile(mode="w", dir=config.parent,
                                         prefix=".openvmm-", suffix=".toml") as stream:
            stream.write(candidate)
            stream.flush()
            effective = dump_config(binary, Path(stream.name))
    validate_effective(effective, prefix)
    if config.read_text() != original:
        raise ValueError("containerd config changed during validation; retry after CSE finishes")
    if not check:
        if candidate != original:
            metadata = config.stat()
            atomic_write(config, candidate, stat.S_IMODE(metadata.st_mode),
                         (metadata.st_uid, metadata.st_gid))
            print("OpenVMM: restored image-owned containerd import after configuration generation")
        receipt.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        atomic_write(receipt, fingerprint(effective))
    return effective


def check_running(config=CONFIG, prefix=PREFIX, receipt=RECEIPT):
    effective = reconcile(config, prefix, receipt, check=True)
    if receipt.read_text() != fingerprint(effective):
        raise ValueError("effective containerd configuration changed since startup")
    pid = subprocess.check_output(
        ["systemctl", "show", "containerd", "--property=MainPID", "--value"], text=True).strip()
    if not pid.isdecimal() or int(pid) <= 0:
        raise ValueError("containerd service has no running main process")
    proc = Path("/proc") / pid
    if (proc / "exe").resolve() != (prefix / "bin/containerd").resolve():
        raise ValueError("containerd service is not running the image-owned binary")
    argv = (proc / "cmdline").read_bytes().rstrip(b"\0").split(b"\0")
    if argv[1:] != [b"--config", os.fsencode(config)]:
        raise ValueError("containerd service is not using the validated config path")
    # Config v4 moves the listener into a server plugin; config dump migrates
    # older inputs, so checking only the original [grpc] table misses it.
    server = effective.get("plugins", {}).get("io.containerd.server.v1.grpc")
    address = (server if server is not None else effective.get("grpc", {})).get("address")
    if not address:
        raise ValueError("effective containerd gRPC address is missing")
    output = subprocess.check_output(
        [str(prefix / "bin/ctr"), "--address", address, "plugins", "ls"],
        text=True, timeout=30)
    statuses = {f"{parts[0]}.{parts[1]}": parts[-1]
                for line in output.splitlines()[1:] if len(parts := line.split()) >= 4}
    for plugin in ("io.containerd.differ.v1.erofs", "io.containerd.snapshotter.v1.erofs",
                   "io.containerd.cri.v1.images", "io.containerd.cri.v1.runtime",
                   "io.containerd.grpc.v1.cri"):
        if statuses.get(plugin) != "ok":
            raise ValueError(f"running containerd plugin is not healthy: {plugin}")
    print("OpenVMM: running containerd binary, startup configuration and CRI/EROFS plugins verified")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group()
    mode.add_argument("--check", action="store_true")
    mode.add_argument("--check-running", action="store_true")
    mode.add_argument("--initialize", action="store_true")
    args = parser.parse_args()
    if args.initialize and not CONFIG.exists():
        # Build-only bootstrap, never a startup fallback for missing CSE config.
        CONFIG.parent.mkdir(parents=True, exist_ok=True)
        default = subprocess.check_output([str(PREFIX / "bin/containerd"), "config", "default"],
                                          text=True, timeout=30)
        with CONFIG.open("x") as stream:
            os.chmod(CONFIG, 0o600)
            stream.write(default)
    if args.check_running:
        check_running()
    else:
        reconcile(check=args.check)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.SubprocessError) as error:
        sys.exit(f"OpenVMM containerd validation failed: {error}")
