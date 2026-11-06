#!/usr/bin/env python3
"""Make WALinuxAgent download extension artifacts through the HostGAPlugin channel first.

By default WALinuxAgent downloads extension manifests and packages directly from storage
(e.g. umsa*.blob.core.windows.net) and only falls back to the HostGAPlugin channel
(168.63.129.16:32526) after the direct download fails. In network isolated clusters direct
storage access is blocked, so every agent start pays for one failed direct download before
the fallback succeeds - and that happens before the CSE extension can even be downloaded.

WALinuxAgent has no configuration option to change the default channel, so this script flips
the initial value of HostPluginProtocol.is_default_channel in the installed agent sources.

The VHD is shared between network isolated and regular clusters, so this script is a no-op
unless the node was bootstrapped with the PREFER_HOSTGAPLUGIN feature, delivered through
custom data. It runs as ExecStartPre of the agent service and must never fail the agent start.
"""

import glob
import os
import re
import sys
import tempfile
import zipfile

ENABLED_FEATURES_FILE = "/opt/azure/containers/enabled_features.sh"
MARKER_FILE = "/opt/azure/containers/prefer_hostgaplugin"
FEATURE_LINE = "PREFER_HOSTGAPLUGIN=true"

MODULE_PATH = "azurelinuxagent/common/protocol/hostplugin.py"
EGG_GLOB = "/var/lib/waagent/WALinuxAgent-*/bin/WALinuxAgent-*.egg"
# Newer agents use "is_default_channel", older ones "_is_default_channel".
PATTERN = re.compile(rb"^([ \t]*_?is_default_channel)[ \t]*=[ \t]*False[ \t]*$", re.MULTILINE)


def log(msg):
    print("aks-waagent-prefer-hostgaplugin: " + msg, flush=True)


def feature_enabled():
    if os.path.isfile(MARKER_FILE):
        return True
    try:
        with open(ENABLED_FEATURES_FILE, encoding="utf-8") as f:
            return any(line.strip() == FEATURE_LINE for line in f)
    except OSError:
        return False


def patch_source(content):
    return PATTERN.subn(rb"\1 = True", content, count=1)


def patch_installed_package():
    try:
        import azurelinuxagent.common.protocol.hostplugin as hostplugin  # pylint: disable=import-outside-toplevel
    except ImportError:
        return
    path = hostplugin.__file__
    if not path.endswith(".py"):
        return
    try:
        with open(path, "rb") as f:
            content = f.read()
        patched, count = patch_source(content)
        if count == 0:
            return
        with open(path, "wb") as f:
            f.write(patched)
        for pyc in glob.glob(os.path.join(os.path.dirname(path), "__pycache__", "hostplugin.*.pyc")):
            os.remove(pyc)
        log("patched " + path)
    except OSError as e:
        # e.g. read-only /usr on OSGuard images
        log("skipped {0}: {1}".format(path, e))


def patch_egg(egg):
    try:
        with zipfile.ZipFile(egg) as src:
            if MODULE_PATH not in src.namelist():
                return
            patched, count = patch_source(src.read(MODULE_PATH))
            if count == 0:
                return
            fd, tmp = tempfile.mkstemp(dir=os.path.dirname(egg), suffix=".tmp")
            os.close(fd)
            with zipfile.ZipFile(tmp, "w", zipfile.ZIP_DEFLATED) as dst:
                for item in src.infolist():
                    name = item.filename
                    # drop stale bytecode for the patched module so the patched source is used
                    if "hostplugin" in name and name.endswith((".pyc", ".pyo")):
                        continue
                    data = patched if name == MODULE_PATH else src.read(item)
                    dst.writestr(item, data)
        os.chmod(tmp, os.stat(egg).st_mode)
        os.replace(tmp, egg)
        log("patched " + egg)
    except (OSError, zipfile.BadZipFile) as e:
        log("skipped {0}: {1}".format(egg, e))


def main():
    if not feature_enabled():
        return 0
    patch_installed_package()
    for egg in glob.glob(EGG_GLOB):
        patch_egg(egg)
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:  # pylint: disable=broad-except
        log("unexpected error: {0}".format(e))
        sys.exit(0)
