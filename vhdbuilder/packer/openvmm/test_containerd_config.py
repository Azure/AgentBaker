"""Replay CSE configurations without touching the host service or /etc."""

import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import tomllib
import unittest
from unittest.mock import patch

from containerd_config import check_running, fingerprint, reconcile, replace_imports, validate_effective


class ImportTests(unittest.TestCase):
    def test_preserves_unrelated_text_and_multiline_strings(self):
        for assignment in (
            'imports = ["a.toml"] # old import',
            '"imports" = [\n "a.toml", # closing ] is only a comment\n "b.toml",\n]',
        ):
            before = 'version = 3\n# preserve this\n'
            after = '\n[debug]\nlevel = "debug"\n[plugins.example]\nnote = """\nimports = []\n"""\n'
            result = replace_imports(before + assignment + after, ["managed.toml"])
            self.assertTrue(result.startswith(before))
            self.assertTrue(result.endswith(after.lstrip("\n")))
            self.assertEqual(tomllib.loads(result)["imports"], ["managed.toml"])
            self.assertIn('note = """\nimports = []\n"""', result)

    def test_missing_imports_added_before_tables(self):
        text = 'version = 3\n[debug]\nlevel = "info"\n'
        self.assertEqual(replace_imports(text, ["a"]), 'imports = ["a"]\n' + text)

    def test_running_gate_checks_receipt_process_and_plugin_health(self):
        effective = {"grpc": {"address": "/run/containerd/containerd.sock"}}
        plugins = "\n".join(["TYPE ID PLATFORMS STATUS"] + [
            f"{kind} {name} - ok" for kind, name in (
                ("io.containerd.differ.v1", "erofs"),
                ("io.containerd.snapshotter.v1", "erofs"),
                ("io.containerd.cri.v1", "images"),
                ("io.containerd.cri.v1", "runtime"),
                ("io.containerd.grpc.v1", "cri"),
            )])
        with tempfile.TemporaryDirectory() as directory:
            prefix = Path(directory) / "prefix"
            binary = prefix / "bin/containerd"
            config = Path(directory) / "config.toml"
            receipt = Path(directory) / "receipt"
            receipt.write_text(fingerprint(effective))
            argv = os.fsencode(binary) + b"\0--config\0" + os.fsencode(config) + b"\0"
            for failure in (None, "receipt", "binary", "argv", "plugin", "pid"):
                with self.subTest(failure=failure):
                    receipt.write_text("stale" if failure == "receipt" else fingerprint(effective))
                    def resolve_path(path):
                        if str(path).startswith("/proc/") and failure == "binary":
                            return Path("/usr/bin/containerd")
                        return binary
                    with patch("containerd_config.reconcile", return_value=effective) as reconcile_mock, \
                            patch("containerd_config.subprocess.check_output",
                                  side_effect=["0" if failure == "pid" else "123",
                                               plugins.replace("erofs - ok", "erofs - error")
                                               if failure == "plugin" else plugins]), \
                            patch.object(Path, "resolve", autospec=True, side_effect=resolve_path), \
                            patch.object(Path, "read_bytes",
                                         return_value=b"containerd\0" if failure == "argv" else argv):
                        if failure:
                            with self.assertRaises(ValueError):
                                check_running(config, prefix, receipt)
                        else:
                            check_running(config, prefix, receipt)
                        reconcile_mock.assert_called_once_with(config, prefix, receipt, check=True)


@unittest.skipUnless(os.environ.get("CONTAINERD"), "set CONTAINERD to the evaluated 2.x binary")
class ContainerdReplayTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.prefix = self.root / "prefix"
        (self.prefix / "bin").mkdir(parents=True)
        (self.prefix / "bin/containerd").symlink_to(Path(os.environ["CONTAINERD"]).resolve())
        for variant in ("v1", "v2"):
            (self.prefix / "containerd" / variant).mkdir(parents=True)
        modern = Path("vhdbuilder/packer/openvmm/containerd-v2.toml").read_text()
        (self.prefix / "containerd/v2/openvmm.toml").write_text(modern)
        legacy = modern.replace('[plugins."io.containerd.cri.v1.images"]',
                                '[plugins."io.containerd.grpc.v1.cri".containerd]')
        legacy = legacy.replace("io.containerd.cri.v1.runtime", "io.containerd.grpc.v1.cri")
        legacy = legacy.replace("  use_local_image_pull = false\n", "")
        legacy += '[plugins."io.containerd.cri.v1.images"]\n  use_local_image_pull = false\n'
        (self.prefix / "containerd/v1/openvmm.toml").write_text(legacy)
        self.config = self.root / "config.toml"
        self.receipt = self.root / "run/config.sha256"

    def configure(self, check=False):
        return reconcile(self.config, self.prefix, self.receipt, check=check)

    def fixture(self, version):
        runtime = "io.containerd.grpc.v1.cri" if version == 2 else "io.containerd.cri.v1.runtime"
        images = "io.containerd.grpc.v1.cri" if version == 2 else "io.containerd.cri.v1.images"
        return f'''# RP config: this comment must survive
version = {version}
root = "/var/lib/containerd"
[plugins."{runtime}".cni]
  bin_dir = "/opt/cni/bin"
  conf_dir = "/etc/cni/net.d"
[plugins."{runtime}".containerd]
  default_runtime_name = "runc"
[plugins."{runtime}".containerd.runtimes.runc]
  runtime_type = "io.containerd.runc.v2"
  [plugins."{runtime}".containerd.runtimes.runc.options]
    SystemdCgroup = true
[plugins."{runtime}".containerd.runtimes.kata-cc]
  runtime_type = "io.containerd.kata.v2"
  snapshotter = "tardev"
  [plugins."{runtime}".containerd.runtimes.kata-cc.options]
    ConfigPath = "/opt/kata/share/defaults/kata-containers/configuration-clh.toml"
[plugins."{images}".registry]
  config_path = "/etc/containerd/certs.d"
'''

    def test_legacy_and_anc_configs_survive_repeated_cse_and_reboot(self):
        for version in (2, 3, 4):
            with self.subTest(version=version):
                original = self.fixture(version)
                self.config.write_text(original)
                self.config.chmod(0o640)
                result = self.configure()
                self.assertEqual(self.config.stat().st_mode & 0o777, 0o640)
                self.assertIn(original, self.config.read_text())
                runtime = result["plugins"]["io.containerd.cri.v1.runtime"]
                self.assertEqual(runtime["containerd"]["default_runtime_name"], "runc")
                self.assertTrue(runtime["containerd"]["runtimes"]["runc"]["options"]["SystemdCgroup"])
                self.assertEqual(runtime["cni"]["conf_dir"], "/etc/cni/net.d")
                self.assertEqual(result["plugins"]["io.containerd.cri.v1.images"]["registry"]
                                 ["config_path"], "/etc/containerd/certs.d")
                reconciled = self.config.read_bytes()
                receipt = self.receipt.read_bytes()
                self.configure()
                self.assertEqual(self.config.read_bytes(), reconciled)
                self.assertEqual(self.receipt.read_bytes(), receipt)
                with patch("containerd_config.tempfile.NamedTemporaryFile",
                           side_effect=AssertionError("read-only acceptance must not create files")):
                    self.assertEqual(self.configure(check=True), result)
                # CSE replaces the file; acceptance must detect this without repairing it.
                self.config.write_text(original)
                with self.assertRaisesRegex(ValueError, "startup reconciliation required"):
                    self.configure(check=True)
                self.assertEqual(self.config.read_text(), original)
                shutil.rmtree(self.receipt.parent)  # simulated reboot clears /run
                self.assertEqual(self.configure(), result)

    def test_existing_relative_and_nested_imports_preserved(self):
        (self.root / "nested.toml").write_text('version = 3\n[debug]\nlevel = "debug"\n')
        (self.root / "site.toml").write_text('version = 3\nimports = ["nested.toml"]\n')
        self.config.write_text('imports = ["site.toml"]\n' + self.fixture(3))
        result = self.configure()
        self.assertEqual(result["debug"]["level"], "debug")
        self.assertEqual(tomllib.loads(self.config.read_text())["imports"][0], "site.toml")

    def test_nested_import_cannot_silently_override_owned_settings(self):
        (self.root / "nested.toml").write_text(
            'version = 3\n[plugins."io.containerd.differ.v1.erofs"]\n enable_dmverity = false\n')
        (self.root / "site.toml").write_text('version = 3\nimports = ["nested.toml"]\n')
        self.config.write_text('imports = ["site.toml"]\n' + self.fixture(3))
        # Import precedence differs between loaders: either the owned override wins
        # or startup must fail explicitly, never succeed with disabled integrity.
        try:
            result = self.configure()
        except ValueError as error:
            self.assertIn("enable_dmverity", str(error))
            self.assertFalse(self.receipt.exists())
        else:
            self.assertIs(result["plugins"]["io.containerd.differ.v1.erofs"]["enable_dmverity"], True)

    def test_containerd_failure_does_not_log_config_credentials(self):
        self.config.write_text(self.fixture(3))
        with patch("containerd_config.subprocess.run", return_value=subprocess.CompletedProcess(
                ["containerd"], 1, "", "credential-from-private-config")):
            with self.assertRaisesRegex(ValueError, "withheld to protect credentials") as error:
                self.configure()
        self.assertNotIn("credential-from-private-config", str(error.exception))
        self.assertEqual(self.config.read_text(), self.fixture(3))

    def test_anc_glob_normalized_and_wrong_schema_import_replaced(self):
        for variant in ("v1", "v2"):
            self.config.write_text(
                f'imports = ["{self.prefix}/containerd/{variant}/*.toml"]\n' + self.fixture(3))
            self.configure()
            imports = tomllib.loads(self.config.read_text())["imports"]
            self.assertEqual(imports, [str(self.prefix / "containerd/v2/openvmm.toml")])

    def test_build_bootstrap_defaults_and_explicit_conflicting_values(self):
        default = subprocess.check_output(
            [str(self.prefix / "bin/containerd"), "config", "default"], text=True)
        self.config.write_text(replace_imports(default, []))
        effective = self.configure()
        self.assertIs(effective["plugins"]["io.containerd.cri.v1.images"]["use_local_image_pull"], False)
        self.assertEqual(effective["plugins"]["io.containerd.cri.v1.images"]["snapshotter"], "erofs")
        conflicting = self.fixture(3) + '''
[plugins."io.containerd.differ.v1.erofs"]
  enable_dmverity = false
  enable_tar_index = true
  mkfs_options = ["--legacy"]
[plugins."io.containerd.snapshotter.v1.erofs"]
  default_size = "64MB"
'''
        self.config.write_text(conflicting)
        effective = self.configure()
        self.assertIs(effective["plugins"]["io.containerd.differ.v1.erofs"]["enable_dmverity"], True)
        self.assertIs(effective["plugins"]["io.containerd.differ.v1.erofs"]["enable_tar_index"], False)

    def test_disabled_plugin_and_invalid_input_fail_without_writes(self):
        for text in ('version = 1\n', 'version = 3\nimports = "not an array"\n',
                     'version = 3\n[invalid', 'version = 3\ndisabled_plugins = ["cri"]\n',
                     'version = 3\ndisabled_plugins = ["io.containerd.differ.v1.erofs"]\n'):
            with self.subTest(text=text):
                self.config.write_text(text)
                with self.assertRaises(ValueError):
                    self.configure()
                self.assertEqual(self.config.read_text(), text)
                self.assertFalse(self.receipt.exists())

    def test_missing_fragment_rejected_before_config_change(self):
        self.config.write_text(self.fixture(3))
        (self.prefix / "containerd/v2/openvmm.toml").unlink()
        with self.assertRaisesRegex(ValueError, "missing image-owned fragment"):
            self.configure()
        self.assertEqual(self.config.read_text(), self.fixture(3))

    def test_effective_configuration_not_just_import_presence(self):
        self.config.write_text(self.fixture(3))
        effective = self.configure()
        for key, value in (("enable_dmverity", False), ("enable_tar_index", True)):
            with self.subTest(key=key):
                damaged = tomllib.loads(
                    (self.prefix / "containerd/v2/openvmm.toml").read_text())
                damaged["plugins"]["io.containerd.differ.v1.erofs"][key] = value
                with self.assertRaisesRegex(ValueError, key):
                    validate_effective(damaged, self.prefix)
        effective["plugins"]["io.containerd.cri.v1.runtime"]["containerd"]["runtimes"] \
            ["kata-cc"]["runtime_type"] = "io.containerd.kata.v2"
        with self.assertRaisesRegex(ValueError, "runtime_type"):
            validate_effective(effective, self.prefix)

    def test_concurrent_cse_write_rejected(self):
        self.config.write_text(self.fixture(3))
        from containerd_config import dump_config

        def concurrent_write(*args):
            result = dump_config(*args)
            self.config.write_text("# CSE replacement\n" + self.fixture(3))
            return result

        with patch("containerd_config.dump_config", side_effect=concurrent_write):
            with self.assertRaisesRegex(ValueError, "changed during validation"):
                self.configure()
        self.assertTrue(self.config.read_text().startswith("# CSE replacement"))
        self.assertFalse(self.receipt.exists())

    def test_no_startup_fallback_for_missing_or_symlink_config(self):
        with self.assertRaisesRegex(ValueError, "regular, non-symlink"):
            self.configure()
        other = self.root / "other.toml"
        other.write_text(self.fixture(3))
        self.config.symlink_to(other)
        with self.assertRaisesRegex(ValueError, "regular, non-symlink"):
            self.configure()
        self.assertEqual(other.read_text(), self.fixture(3))

    def test_installer_wires_startup_and_read_only_acceptance(self):
        source = Path("vhdbuilder/packer/openvmm/install.sh").read_text()
        self.assertIn("ExecStartPre=/usr/bin/python3 /opt/kata-openvmm/integration/containerd_config.py", source)
        self.assertIn("ExecStart=/opt/kata-openvmm/bin/containerd --config /etc/containerd/config.toml", source)
        self.assertIn("/etc/modules-load.d/openvmm.conf", source)
        self.assertIn("RuntimeDirectory=kata-openvmm", source)
        host = Path("vhdbuilder/packer/openvmm/check-host.sh").read_text()
        self.assertNotIn('modprobe "$module"', host)
        self.assertIn("containerd_config.py\" --check-running", host)


if __name__ == "__main__":
    unittest.main()
