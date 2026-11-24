"""Focused tests for the isolated image recipe, launch boundary and metadata observer."""
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
EXPERIMENT = ROOT / "vhdbuilder/experiments/lps-json-cold-ab"


def load(name):
    spec = importlib.util.spec_from_file_location(name, EXPERIMENT / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ColdABTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(dir=ROOT / ".test-tmp")
        self.addCleanup(self.directory.cleanup)
        self.base = Path(self.directory.name)

    def run_shell(self, script, **env):
        return subprocess.run(["bash", "-c", script], text=True, capture_output=True,
                              env={**os.environ, **env}, check=False)

    def prefetch(self, flag, enabled="true", mode="json"):
        source = (ROOT / "parts/linux/cloud-init/artifacts/cse_main.sh").read_text()
        body = source[source.index("prepullImages() {"):source.index("\nfunction nodePrep")]
        body = body.replace("/opt/azure/containers/lps-json-cold-ab", str(self.base))
        (self.base / "hooks.sh").write_text(
            'lpsColdObserve() { echo "observe:$1"; }\n'
            'lpsAsyncPrefetch() { echo "async:$1"; return "${LAUNCH_RESULT:-0}"; }\n')
        (self.base / "worker.sh").write_text('printf "worker:%s:%s\\n" "$1" "$2"\n')
        return self.run_shell(
            body + '\nprepullImages', ENABLE_NETWORKING_IMAGE_CACHE=enabled,
            ANC_IMAGE_CACHE_MODE=mode, ENABLE_NETWORKING_IMAGE_CACHE_ASYNC=flag,
            ANC_SELECTED_BIN_PATH="/selected path/anc")

    def test_feature_default_false_and_true(self):
        for flag in ("", "false"):
            result = self.prefetch(flag)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn("worker:sync:/selected path/anc", result.stdout)
            self.assertLess(result.stdout.index("observe:"), result.stdout.index("worker:"))
            self.assertNotIn("async:", result.stdout)
        result = self.prefetch("true")
        self.assertIn("async:/selected path/anc", result.stdout)
        self.assertNotIn("worker:", result.stdout)

    def test_disabled_or_unsupported_never_launches(self):
        for enabled, mode in (("false", "json"), ("true", "other")):
            result = self.prefetch("true", enabled, mode)
            self.assertEqual(result.stdout, "")
            self.assertEqual(result.returncode, 0)
        self.assertNotIn("async:", self.prefetch("true", mode="nbc").stdout)

    def test_launch_failure_is_fail_open_but_logged(self):
        with patch.dict(os.environ, {"LAUNCH_RESULT": "124"}):
            result = self.prefetch("true")
        self.assertEqual(result.returncode, 0)
        self.assertIn("failed or timed out", result.stdout)
        self.assertNotIn("launched;", result.stdout)

    def test_normal_feature_file_delivers_exact_async_key(self):
        flags = self.base / "features"
        flags.write_text("ENABLE_NETWORKING_IMAGE_CACHE_ASYNC=true\n")
        result = self.run_shell(
            '. parts/linux/cloud-init/artifacts/aks-node-controller-hotfix.sh\n'
            f'anc_hotfix_read_feature_flags "{flags}" >/dev/null\n'
            'printf "%s" "$ENABLE_NETWORKING_IMAGE_CACHE_ASYNC"')
        self.assertEqual(result.stdout, "true")

    def test_managed_launch_keeps_private_context_without_wait_or_ordering(self):
        hooks = (EXPERIMENT / "hooks.sh").read_text().replace(
            "/run/aks-lps-json-prefetch", str(self.base / "context"))
        result = self.run_shell(hooks + """
timeout() { shift 3; "$@"; }
systemd-run() {
    printf '%s\\n' "$@"
    bash -c 'source "$1"; printf "context:%s:%s" "$ANC_IMAGE_CACHE_MODE" "$ANC_IMAGE_CACHE_INPUT"' _ """ +
            f'"{self.base}/context/environment.sh"' + """
}
lpsAsyncPrefetch '/selected path/anc'
""", ANC_IMAGE_CACHE_MODE="json", ANC_IMAGE_CACHE_INPUT="/private config.json")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("context:json:/private config.json", result.stdout)
        self.assertIn("RuntimeMaxSec=330", result.stdout)
        self.assertIn("ExecStopPost=", result.stdout)
        self.assertIn("/selected path/anc", result.stdout)
        self.assertNotIn("--wait", result.stdout)
        self.assertNotIn("Before=", result.stdout)
        self.assertEqual((self.base / "context/environment.sh").stat().st_mode & 0o777, 0o600)

    def test_observer_reads_no_layers_and_captures_chain_metadata(self):
        observer = load("observe")
        observer.BASE = self.base
        observer.ROOT = self.base / "containerd"
        (self.base / "expected-inventory.json").write_text("{}")
        blobs = observer.ROOT / "io.containerd.content.v1.content/blobs/sha256"
        blobs.mkdir(parents=True)
        (blobs / ("a" * 64)).write_bytes(b"not read")
        snapshots = observer.ROOT / "io.containerd.snapshotter.v1.overlayfs/snapshots/42"
        snapshots.mkdir(parents=True)
        calls = []

        def metadata(argv, **kwargs):
            calls.append(argv)
            if "namespaces" in argv:
                return "k8s.io\n"
            if "info" in argv:
                return '{"Name":"sha256:chain","Parent":"sha256:parent","Kind":"Committed"}'
            if "snapshots" in argv:
                return "KEY PARENT KIND\nsha256:chain sha256:parent Committed\n"
            return "REF TYPE DIGEST SIZE\n"

        original_read = Path.read_bytes

        def read_bytes(path):
            if str(path) == "/etc/containerd/config.toml":
                return b""
            return original_read(path)

        with patch.object(observer.subprocess, "check_output", side_effect=metadata), \
                patch.object(Path, "read_bytes", read_bytes):
            result = observer.capture()
        self.assertTrue(result["valid"])
        self.assertEqual(result["content"], [{"digest": "sha256:" + "a" * 64, "size": 8}])
        self.assertEqual(result["physical_snapshot_ids"], ["42"])
        self.assertEqual(result["namespaces"][0]["snapshots"][0]["Kind"], "Committed")
        self.assertTrue(all(not {"pull", "fetch", "get", "mount", "unpack", "import"} & set(cmd)
                            for cmd in calls))
        with patch.object(observer.subprocess, "check_output", side_effect=RuntimeError), \
                patch.object(Path, "read_bytes", read_bytes):
            self.assertFalse(observer.capture()["valid"])

    def test_cache_input_excludes_aliases_and_preserves_every_other_dependency(self):
        prepare = load("prepare")
        record = json.loads((EXPERIMENT / "cache-input-record.json").read_text())
        components = json.loads((ROOT / "parts/common/components.json").read_text())
        excluded = set(record["excluded_repositories"])
        for category in ("ContainerImages", "GPUContainerImages", "OCIArtifacts"):
            self.assertFalse(any(item.get("downloadURL", "").rsplit(":", 1)[0] in excluded
                                 for item in components[category]))
        original = subprocess.check_output(
            ["git", "show", "39c49f59632fab867c8d12339db54fa075480586:parts/common/components.json"],
            cwd=ROOT)
        filtered, removed = prepare.filter_components(original, excluded)
        self.assertEqual(filtered, (ROOT / "parts/common/components.json").read_bytes())
        self.assertEqual(removed, record["removed"])
        baseline = json.loads(original)
        for category in baseline:
            expected = [item for item in baseline[category]
                        if item.get("downloadURL", "").rsplit(":", 1)[0] not in excluded]
            self.assertEqual(components[category], expected)

    def test_kubelet_observation_is_after_import_and_before_start(self):
        source = (ROOT / "parts/linux/cloud-init/artifacts/cse_config_kubelet.sh").read_text()
        self.assertLess(source.index("image import"), source.index("lpsColdObserve before-kubelet"))
        self.assertLess(source.index("lpsColdObserve before-kubelet"),
                        source.index("systemctlEnableAndStartNoBlock kubelet"))

    def test_worker_preserves_actual_failure_and_timeout(self):
        observer = (EXPERIMENT / "observe.py").read_text().replace(
            "/opt/azure/containers/lps-json-cold-ab", str(self.base)).replace(
            "/opt/azure/containers/image-cache-status.json", str(self.base / "status.json"))
        (self.base / "observe.py").write_text(observer)
        worker = (EXPERIMENT / "worker.sh").read_text().replace(
            "/opt/azure/containers/lps-json-cold-ab", str(self.base)).replace(
            "/run/aks-lps-json-prefetch/environment.sh", str(self.base / "environment.sh"))
        anc = self.base / "anc"
        anc.write_text("#!/bin/bash\nexit 0\n")
        anc.chmod(0o700)
        for outcome, want in (("complete", 0), ("failed", 1)):
            (self.base / "status.json").write_text(json.dumps({"outcome": outcome}))
            result = subprocess.run(["bash", "-c", worker, "_", "sync", str(anc)],
                                    capture_output=True, text=True, env=os.environ.copy())
            self.assertEqual(result.returncode, want, result.stderr)
            end = json.loads((self.base / "worker-end.json").read_text())
            self.assertEqual(end["prefetch_status"]["outcome"], outcome)
            self.assertEqual(end["worker_exit_code"], want)
            self.assertEqual(end["mode"], "sync")
            for name in ("worker-start.json", "worker-end.json"):
                (self.base / name).unlink()
        (self.base / "environment.sh").write_text("export INVOCATION_ID=parent-cse\n")
        (self.base / "status.json").write_text('{"outcome":"complete"}')
        result = subprocess.run(["bash", "-c", worker, "_", str(anc)],
                                capture_output=True, text=True,
                                env={**os.environ, "INVOCATION_ID": "native-worker-unit"})
        self.assertEqual(result.returncode, 0, result.stderr)
        end = json.loads((self.base / "worker-end.json").read_text())
        self.assertEqual(end["mode"], "async")
        self.assertEqual(end["invocation_id"], "native-worker-unit")
        for name in ("worker-start.json", "worker-end.json"):
            (self.base / name).unlink()
        timeout = self.base / "timeout"
        timeout.write_text("#!/bin/bash\nexit 124\n")
        timeout.chmod(0o700)
        (self.base / "status.json").unlink()
        result = subprocess.run(["bash", "-c", worker, "_", "sync", str(anc)],
                                capture_output=True, text=True,
                                env={**os.environ, "PATH": str(self.base) + ":" + os.environ["PATH"]})
        self.assertEqual(result.returncode, 124, result.stderr)
        end = json.loads((self.base / "worker-end.json").read_text())
        self.assertEqual(end["exit_code"], 124)
        self.assertTrue(end["prefetch_status_missing"])

    def test_unit_exit_uses_systemd_native_status_and_observation_never_overwrites(self):
        observer = load("observe")
        observer.BASE = self.base
        output = subprocess.CompletedProcess([], 0, stdout=self.unit_properties_output())
        with patch("sys.argv", ["observe.py", "unit-exit"]), patch.dict(
                os.environ, {"SERVICE_RESULT": "timeout", "EXIT_CODE": "killed",
                             "EXIT_STATUS": "15", "INVOCATION_ID": "native-invocation"}), \
                patch.object(observer.subprocess, "run", return_value=output):
            observer.main()
            result = json.loads((self.base / "unit-exit.json").read_text())
            self.assertEqual(result["SERVICE_RESULT"], "timeout")
            self.assertEqual(result["EXIT_STATUS"], "15")
            self.assertEqual(result["invocation_id"], "native-invocation")
            self.assertTrue(result["valid"])
            with self.assertRaises(FileExistsError):
                observer.main()

    @staticmethod
    def unit_properties_output():
        return "\n".join([
            "Id=aks-lps-json-prefetch.service", "Type=exec", "RemainAfterExit=no",
            "RuntimeMaxUSec=5min 30s", "TimeoutStartUSec=1min 30s", "TimeoutStopUSec=10s",
            "KillMode=control-group", "UMask=0077", "Before=shutdown.target",
            "After=basic.target", "Requires=sysinit.target", "Wants=", "BindsTo=", "PartOf=",
            "ControlGroup=/system.slice/aks-lps-json-prefetch.service",
            "InvocationID=native-invocation", "ExecMainPID=1234", "ExecMainCode=1",
            "ExecMainStatus=0", "ExecMainStartTimestampMonotonic=1000000",
            "ExecMainExitTimestampMonotonic=2000000", "Result=success", "",
        ])

    def test_unit_exit_retains_effective_properties_after_unit_disappears(self):
        observer = load("observe")
        observer.BASE = self.base
        output = subprocess.CompletedProcess([], 0, stdout=self.unit_properties_output())
        with patch("sys.argv", ["observe.py", "unit-exit"]), patch.dict(
                os.environ, {"SERVICE_RESULT": "success", "EXIT_CODE": "exited",
                             "EXIT_STATUS": "0", "INVOCATION_ID": "native-invocation"}), \
                patch.object(observer.subprocess, "run", return_value=output) as query:
            observer.main()
            argv = query.call_args.args[0]
            self.assertEqual(argv[:4], ["systemctl", "show", "--no-pager", "--all"])
            expected = {line.partition("=")[0] for line in self.unit_properties_output().splitlines()}
            self.assertEqual(set(argv[4].removeprefix("--property=").split(",")), expected)
            self.assertNotIn("Environment", argv[4])
            self.assertNotIn("ExecStart", argv[4])
            self.assertEqual(query.call_args.kwargs["timeout"], 3)
            self.assertEqual(query.call_args.kwargs["stderr"], subprocess.DEVNULL)
            query.return_value = subprocess.CompletedProcess([], 4, stdout="")
            self.assertFalse(observer.capture_effective_unit("native-invocation")["valid"])
            retained = json.loads((self.base / "unit-exit.json").read_text())
            effective = retained["effective_unit"]
            self.assertTrue(retained["valid"])
            self.assertTrue(effective["valid"])
            self.assertTrue(effective["complete"])
            self.assertEqual(effective["boot_id"], retained["boot_id"])
            self.assertEqual(effective["invocation_id"], retained["invocation_id"])
            self.assertEqual(effective["properties"]["InvocationID"], retained["invocation_id"])
            self.assertEqual(effective["properties"]["RemainAfterExit"], "no")
            self.assertEqual(effective["properties"]["ExecMainExitTimestampMonotonic"], "2000000")
            self.assertNotIn("kubelet", effective["properties"]["Before"])

    def test_unit_exit_missing_failed_or_unsafe_query_is_explicitly_invalid(self):
        observer = load("observe")
        good = self.unit_properties_output()
        cases = [
            subprocess.CompletedProcess([], 0, stdout=""),
            subprocess.CompletedProcess([], 0, stdout="Id=aks-lps-json-prefetch.service\n"),
            subprocess.CompletedProcess([], 4, stdout=good),
            subprocess.TimeoutExpired(["systemctl"], 3),
            FileNotFoundError("systemctl unavailable"),
            subprocess.CompletedProcess([], 0, stdout=good.replace("Type=exec", "Type=")),
            subprocess.CompletedProcess([], 0, stdout=good.replace(
                "InvocationID=native-invocation", "InvocationID=another-invocation")),
            subprocess.CompletedProcess([], 0, stdout=good + "Environment=private-value\n"),
        ]
        observer.BASE = self.base
        for response in cases:
            with self.subTest(response=type(response).__name__), \
                    patch("sys.argv", ["observe.py", "unit-exit"]), patch.dict(
                        os.environ, {"SERVICE_RESULT": "success", "EXIT_CODE": "exited",
                                     "EXIT_STATUS": "0", "INVOCATION_ID": "native-invocation"}), \
                    patch.object(observer.subprocess, "run") as query:
                if isinstance(response, Exception):
                    query.side_effect = response
                else:
                    query.return_value = response
                observer.main()
                path = self.base / "unit-exit.json"
                raw = path.read_text()
                retained = json.loads(raw)
                self.assertFalse(retained["valid"])
                self.assertFalse(retained["effective_unit"]["valid"])
                self.assertTrue(retained["effective_unit"]["errors"])
                self.assertNotIn("private-value", raw)
                self.assertNotIn("Environment", retained["effective_unit"]["properties"])
                if not retained["effective_unit"]["properties"]:
                    self.assertFalse(retained["effective_unit"]["complete"])
                    self.assertTrue(retained["effective_unit"]["missing_properties"])
                path.unlink()

    @unittest.skipUnless(shutil.which("containerd") and shutil.which("ctr"),
                         "requires existing local containerd and ctr")
    def test_observer_against_isolated_live_containerd_metadata(self):
        observer = load("observe")
        observer.BASE = self.base
        observer.ROOT = self.base / "root"
        # UNIX socket paths have a 108-byte kernel limit.
        observer.SOCKET = str(self.base / "sock")
        observer.CONFIG = self.base / "config.toml"
        observer.CONFIG.write_text(
            f'version = 2\nroot = "{observer.ROOT}"\nstate = "{self.base}/state"\n'
            'disabled_plugins = ["io.containerd.grpc.v1.cri"]\n'
            f'[grpc]\naddress = "{observer.SOCKET}"\nuid = {os.getuid()}\ngid = {os.getgid()}\n'
            f'[ttrpc]\nuid = {os.getuid()}\ngid = {os.getgid()}\n')
        (self.base / "expected-inventory.json").write_text("{}")
        with (self.base / "containerd.log").open("w") as log:
            process = subprocess.Popen(["containerd", "--config", str(observer.CONFIG)],
                                       stdout=log, stderr=log)
            try:
                deadline = time.monotonic() + 10
                while True:
                    if process.poll() is not None or time.monotonic() > deadline:
                        self.fail((self.base / "containerd.log").read_text())
                    if Path(observer.SOCKET).exists():
                        try:
                            ready = subprocess.run(
                                ["ctr", "--address", observer.SOCKET, "--timeout", "1s",
                                 "namespaces", "list"], capture_output=True, timeout=2, check=False)
                            if ready.returncode == 0:
                                break
                        except subprocess.TimeoutExpired:
                            pass
                    time.sleep(0.1)
                subprocess.run(["ctr", "--address", observer.SOCKET, "namespaces", "create", "k8s.io"],
                               capture_output=True, check=True, timeout=3)
                for args in (["prepare", "fixture-active"],
                             ["commit", "fixture-committed", "fixture-active"]):
                    # Metadata/directory fixture only: never mount, pull, import or read layers.
                    subprocess.run(
                        ["ctr", "--address", observer.SOCKET, "-n", "k8s.io",
                         "snapshots", "--snapshotter", "overlayfs", *args],
                        capture_output=True, check=True, timeout=3)
                # Empty stores do not create the blobs subdirectory until their first write.
                (observer.ROOT / "io.containerd.content.v1.content/blobs/sha256").mkdir(parents=True)
                result = observer.capture()
                self.assertTrue(result["valid"], json.dumps(result))
                self.assertEqual(result["namespaces"][0]["name"], "k8s.io")
                self.assertEqual(result["namespaces"][0]["snapshots"][0]["Name"], "fixture-committed")
                self.assertEqual(len(result["physical_snapshot_ids"]), 1)
            finally:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)


if __name__ == "__main__":
    unittest.main()
