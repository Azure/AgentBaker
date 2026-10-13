"""Opt-in remote/root regression using a separate daemon, socket and image store."""

import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile
import time
import tomllib
import unittest

IMAGE = "example.invalid/openvmm-differ:test"
PAYLOAD = b"ordinary prefetch and confidential unpack must coexist\n"


def add_file(archive, name, data):
    entry = tarfile.TarInfo(name)
    entry.size = len(data)
    entry.mode = 0o644
    archive.addfile(entry, io.BytesIO(data))


def create_image(path):
    layer = io.BytesIO()
    with tarfile.open(fileobj=layer, mode="w") as archive:
        add_file(archive, "probe", PAYLOAD)
    blobs = {}

    def blob(data, media_type):
        digest = hashlib.sha256(data).hexdigest()
        blobs[f"blobs/sha256/{digest}"] = data
        return {"mediaType": media_type, "digest": f"sha256:{digest}", "size": len(data)}

    layer_descriptor = blob(layer.getvalue(), "application/vnd.oci.image.layer.v1.tar")
    config = blob(json.dumps({
        "architecture": "amd64", "os": "linux", "config": {},
        "rootfs": {"type": "layers", "diff_ids": [layer_descriptor["digest"]]},
    }).encode(), "application/vnd.oci.image.config.v1+json")
    manifest = blob(json.dumps({
        "schemaVersion": 2, "mediaType": "application/vnd.oci.image.manifest.v1+json",
        "config": config, "layers": [layer_descriptor],
    }).encode(), "application/vnd.oci.image.manifest.v1+json")
    manifest["annotations"] = {"org.opencontainers.image.ref.name": IMAGE}
    with tarfile.open(path, "w") as archive:
        add_file(archive, "oci-layout", b'{"imageLayoutVersion":"1.0.0"}')
        add_file(archive, "index.json", json.dumps({"schemaVersion": 2, "manifests": [manifest]}).encode())
        for name, data in blobs.items():
            add_file(archive, name, data)


@unittest.skipUnless(os.environ.get("OPENVMM_DIFFER_INTEGRATION") == "1",
                     "opt in on a remote root-capable validation machine")
class DifferIntegrationTests(unittest.TestCase):
    def test_prefetch_and_explicit_erofs_unpack(self):
        self.assertEqual(os.geteuid(), 0, "isolated snapshotter mounts require root")
        self.assertNotEqual(os.readlink("/proc/self/ns/mnt"), os.readlink("/proc/1/ns/mnt"),
                            "run via unshare --mount --propagation private, never in the host namespace")
        # Defense in depth against future listener schema changes using defaults.
        subprocess.run(["mount", "-t", "tmpfs", "tmpfs", "/run"], check=True)
        binary = Path(os.environ["CONTAINERD"])
        ctr = binary.with_name("ctr")
        fragment = Path("vhdbuilder/packer/openvmm/containerd-v2.toml").read_text()
        self.assertIn('default = ["erofs", "walking"]', fragment)
        for baseline in (True, False):
            with self.subTest(erofs_only_baseline=baseline), tempfile.TemporaryDirectory(
                    prefix="openvmm-differ-") as directory:
                root = Path(directory)
                socket = root / "containerd.sock"
                config = root / "config.toml"
                text = fragment.replace("version = 2", f'''version = 3
root = "{root}/data"
state = "{root}/state"
disabled_plugins = ["io.containerd.grpc.v1.cri", "io.containerd.cri.v1.images", "io.containerd.cri.v1.runtime", "io.containerd.nri.v1.nri", "io.containerd.internal.v1.opt"]
''')
                if baseline:
                    text = text.replace('default = ["erofs", "walking"]', 'default = ["erofs"]')
                text += f'\n[grpc]\n  address = "{socket}"\n'
                text += f'\n[ttrpc]\n  address = "{root}/containerd.ttrpc"\n'
                config.write_text(text)
                effective = tomllib.loads(subprocess.check_output(
                    [str(binary), "--config", str(config), "config", "dump"], text=True))
                servers = effective["plugins"]
                self.assertEqual(servers["io.containerd.server.v1.grpc"]["address"], str(socket))
                self.assertEqual(servers["io.containerd.server.v1.ttrpc"]["address"],
                                 str(root / "containerd.ttrpc"))
                self.assertEqual(effective["root"], str(root / "data"))
                self.assertEqual(effective["state"], str(root / "state"))
                image = root / "image.tar"
                create_image(image)
                mount = root / "mount"
                mount.mkdir()
                command = [str(ctr), "--address", str(socket), "--namespace", "differ-regression"]
                daemon = subprocess.Popen([str(binary), "--config", str(config), "--log-level", "error"])
                try:
                    for _ in range(100):
                        self.assertIsNone(daemon.poll(), "isolated containerd exited during startup")
                        if socket.exists():
                            ready = subprocess.run(command + ["version"], capture_output=True, timeout=5)
                            if ready.returncode == 0:
                                break
                        time.sleep(0.1)
                    else:
                        self.fail("isolated containerd did not become responsive")
                    subprocess.run(command + ["images", "import", "--no-unpack", str(image)], check=True)
                    # Match AgentBaker: no --snapshotter override, and no pre-unpacked image.
                    result = subprocess.run(command + ["images", "mount", IMAGE, str(mount)],
                                            capture_output=True, text=True, timeout=60)
                    if baseline:
                        self.assertNotEqual(result.returncode, 0)
                        self.assertIn("mount layer type must be erofs-layer", result.stderr)
                        print("Reproduced EROFS-only prefetch failure", flush=True)
                    else:
                        self.assertEqual(result.returncode, 0, result.stderr)
                        self.assertEqual((mount / "probe").read_bytes(), PAYLOAD)
                        subprocess.run(command + ["images", "unmount", str(mount)], check=True)
                        # Use the transfer service with the explicit EROFS unpack configuration.
                        subprocess.run(command + ["images", "import", "--snapshotter", "erofs", str(image)],
                                       check=True, timeout=60)
                        layers = list((root / "data/io.containerd.snapshotter.v1.erofs").rglob("*.erofs"))
                        self.assertEqual(len(layers), 1, "explicit EROFS unpack must produce an EROFS layer")
                        # Inspect without activating globally named dm-verity devices
                        # that could collide with the shared daemon's snapshot IDs.
                        extracted = root / "extracted"
                        subprocess.run(["fsck.erofs", f"--extract={extracted}", str(layers[0])],
                                       check=True, timeout=30)
                        self.assertEqual((extracted / "probe").read_bytes(), PAYLOAD)
                        print("Ordinary prefetch and explicit EROFS transfer unpack both passed", flush=True)
                finally:
                    try:
                        if subprocess.run(["mountpoint", "-q", str(mount)]).returncode == 0:
                            subprocess.run(command + ["images", "unmount", str(mount)], check=True, timeout=30)
                    finally:
                        daemon.terminate()
                        try:
                            daemon.wait(timeout=15)
                        except subprocess.TimeoutExpired:
                            daemon.kill()
                            daemon.wait(timeout=15)
                        # The mount manager caches backing mounts beyond ctr unmount.
                        # Reclaim only this fixture's mounts in the private namespace.
                        mounts = [line.split()[4] for line in Path("/proc/self/mountinfo").read_text().splitlines()
                                  if line.split()[4].startswith(str(root) + "/")]
                        for target in sorted(mounts, key=len, reverse=True):
                            subprocess.run(["umount", target], check=True, timeout=30)


if __name__ == "__main__":
    unittest.main()
