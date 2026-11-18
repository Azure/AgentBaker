"""Offline matrix preparation, validation and complete standalone report tests."""

import copy
import hashlib
from html.parser import HTMLParser
import json
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest
import xml.etree.ElementTree as ET
import zipfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from prepare_matrix import IMAGES, prepare, scenario_names
from render_matrix import aggregate, identity, load, render, validate


ROOT = Path(__file__).resolve().parents[3]
SOURCE = ROOT / "parts/linux/cloud-init/artifacts/init-aks-cloud.sh"
TOOL = ROOT / "e2e/boot-trace"


class MatrixTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.fixtures = self.root / "fixtures"
        binary = self.root / "binary"
        binary.write_bytes(b"synthetic CI binary")
        self.images = {name: {"resourceId": f"/images/{name}/versions/1.2.3"}
                       for name in IMAGES.values()}
        metadata = self.root / "metadata.json"
        metadata.write_text(json.dumps(self.images))
        prepare(SOURCE, binary, metadata, self.fixtures)
        self.manifest = load(self.fixtures / "fixture.json")
        self.artifacts = self.root / "artifacts"
        self.artifacts.mkdir()

    def bundle(self, name):
        os_name, variant, _ = identity(name)
        mode = "rust" if "rust" in variant else (
            "skip-update-ca-certificates" if variant == "skip_ca" else "enabled")
        retrieve = "parallel" if variant.startswith("parallel") else "original"
        experiment = {
            "variant": variant, "retrieve_mode": retrieve, "ca_update_mode": mode,
            "rust_sha256": self.manifest["rust_sha256"],
            "init_sha256": hashlib.sha256((self.fixtures / f"init-{retrieve}.sh").read_bytes()).hexdigest(),
            "certificate_count": 3, "certificate_set_sha256": "a" * 64, "bundle_sha256": "b" * 64,
        }
        boot = f"boot-{name}"
        cfg = {
            "vm_size": "test-vm", "os_sku": "Ubuntu" if os_name == "Ubuntu2604Minimal" else "AzureLinux",
            "kubernetes_version": "test", "network_plugin": "test", "cluster_resource_id": "test",
            "provisioning": "test", "base_image_version": "1.2.3",
            "image_resource_id": self.images[IMAGES[os_name]]["resourceId"],
            "experiment_variant": variant,
            **{key: experiment[key] for key in ("retrieve_mode", "ca_update_mode", "rust_sha256", "init_sha256")},
        }
        metadata = {
            "schema": 1, "backend": "ftrace", "boot_id": boot, "kernel": os_name,
            "architecture": "x86_64", "status": "stopped", "finalized_at": "test",
            "clock_start": {"realtime": 1010, "monotonic": 10, "sample_span_seconds": 0.001},
            "image_provenance": {"kernel": os_name, "backend": "ftrace", "base_version": "1.2.3"},
        }
        bundle = self.artifacts / "scenario-logs" / name / "boot-trace"
        bundle.mkdir(parents=True)
        documents = {
            "result.json": {"status": "collected"}, "metadata.json": metadata,
            "experiment.json": experiment,
            "readiness.json": {"boot_id": boot, "node_uid": "uid-" + name, "provider_id": "provider-" + name,
                               "transition_time": "1970-01-01T00:16:44Z", "configuration": cfg},
            "cse-command-start.json": {"boot_id": boot, "start": 0.5},
            "cse-boot-datapoints.json": {"BootDatapoints": {"CSEStartTime": "1970-01-01T00:16:43Z"}},
        }
        for filename, value in documents.items():
            (bundle / filename).write_text(json.dumps(value))
        tasks = [
            {"task": "AKS.CSE.rcv1p.retrieveCerts", "start": 1, "end": 2, "exit_status": 0},
            {"task": "AKS.CSE.rcv1p.installCertsToTrustStore", "start": 2, "end": 3, "exit_status": 0},
        ]
        if os_name == "Ubuntu2604Minimal":
            task = "skippedUpdateCaCertificates" if variant == "skip_ca" else "updateCaCertificates"
            tasks.append({"task": "AKS.CSE.bootTrace." + task, "start": 2.1, "end": 2.8, "exit_status": 0})
        (bundle / "cloud-timings.jsonl").write_text("".join(json.dumps(task) + "\n" for task in tasks))
        (bundle / "cse-task-timings.jsonl").write_text(
            '{"task":"AKS.CSE.network","start":0.6,"end":0.8}\n')
        (bundle / "ftrace.txt").write_text(
            " worker-0 [000] d..2 1.000000: sched_switch: prev_comm=idle prev_pid=0 prev_prio=120 prev_state=S ==> next_comm=worker next_pid=42 next_prio=120\n"
            " worker-42 [000] d..2 4.200000: sched_switch: prev_comm=worker prev_pid=42 prev_prio=120 prev_state=S ==> next_comm=idle next_pid=0 next_prio=120\n")
        return bundle

    def junit(self):
        suite = ET.Element("testsuite")
        for name in scenario_names():
            ET.SubElement(suite, "testcase", name=name, classname="e2e")
        filtered = ET.SubElement(suite, "testcase", name="Unrelated")
        ET.SubElement(filtered, "skipped", message="filtered: does not match tags")
        ET.SubElement(suite, "testcase", name=scenario_names()[0] + "/AKS.CSE.network", classname="CSE")
        ET.ElementTree(suite).write(self.fixtures / "junit.xml")

    def test_prepares_two_instrumented_scripts_and_all_variant_inputs(self):
        original = (self.fixtures / "init-original.sh").read_text()
        parallel = (self.fixtures / "init-parallel.sh").read_text()
        self.assertNotIn("download_cert_batch", original)
        self.assertIn("--parallel-max 4", parallel)
        for script in ("init-original.sh", "init-parallel.sh", "prepare-fixture.sh"):
            subprocess.run(["bash", "-n", str(self.fixtures / script)], check=True)
        self.assertIn("aks_boot_trace_start", original)
        self.assertIn("/opt/aks-node-boot-trace/prepare-fixture.sh", parallel)
        self.assertIn(self.manifest["rust_sha256"], (self.fixtures / "prepare-fixture.sh").read_text())
        self.assertEqual(SOURCE.read_text().count("aks_boot_trace_start"), 0)

    def test_requires_both_exact_vhd_metadata_entries(self):
        metadata = self.root / "missing.json"
        metadata.write_text(json.dumps({next(iter(self.images)): next(iter(self.images.values()))}))
        with self.assertRaisesRegex(ValueError, "Missing exact tracing VHD"):
            prepare(SOURCE, self.root / "binary", metadata, self.root / "missing-fixtures")

    def test_input_drift_and_cross_variant_identity_reuse_are_rejected(self):
        name = scenario_names()[0]
        bundle = self.bundle(name)
        first = validate(bundle, name, self.manifest, self.fixtures)
        first.update(ready_seconds=4, thread_cpu_seconds={"worker": 1})
        with self.assertRaisesRegex(ValueError, "Repeated node"):
            aggregate([first, copy.deepcopy(first)])
        ready = load(bundle / "readiness.json")
        ready["configuration"]["ca_update_mode"] = "rust"
        (bundle / "readiness.json").write_text(json.dumps(ready))
        with self.assertRaisesRegex(ValueError, "Readiness input mismatch"):
            validate(bundle, name, self.manifest, self.fixtures)

    def test_full_report_embeds_all_21_timelines_tasks_and_command_markers(self):
        for name in scenario_names():
            self.bundle(name)
        self.junit()
        output = self.root / "report"
        summary = render(self.artifacts, output, self.fixtures, "test-build")
        self.assertEqual(summary["campaign_state"], "completed")
        self.assertEqual(summary["valid_captures"], 21)
        self.assertEqual(len(summary["groups"]), 7)
        self.assertTrue(all(group["n"] == 3 for group in summary["groups"]))
        self.assertEqual(len(summary["junit_outcomes"]), 21)
        class Frames(HTMLParser):
            def __init__(self):
                super().__init__()
                self.frames = []

            def handle_starttag(self, tag, attrs):
                if tag == "iframe":
                    self.frames.append(dict(attrs))
        frames = Frames()
        page = (output / "index.html").read_text()
        frames.feed(page)
        self.assertEqual(len(frames.frames), 1)
        self.assertNotIn("src", frames.frames[0])
        timeline = frames.frames[0]["srcdoc"]
        payload = json.loads(timeline.split('<script id="payload" type="application/json">')[1].split("</script>")[0])
        self.assertEqual(len(payload["labels"]), 21)
        self.assertEqual([identity(label)[2] for label in payload["labels"]], [1] * 7 + [2] * 7 + [3] * 7)
        self.assertIn("CSE command start (cse_cmd.sh)", timeline)
        self.assertIn("cse_start.sh entry (not first AKS.CSE task)", timeline)
        self.assertIn("AKS.CSE.network", timeline)
        self.assertIn("worker", timeline)
        links = re.findall(r'data-capture="(\d+)">([^<]+)</a>', page)
        self.assertEqual(links, [(str(i), label) for i, label in enumerate(payload["labels"])])
        with zipfile.ZipFile(output / "ca-comparison-report.zip") as archive:
            self.assertIn("index.html", archive.namelist())
            self.assertTrue(all(Path(name).suffix in (".html", ".json") for name in archive.namelist()))

    def test_missing_capture_keeps_partial_report_and_suppresses_incomplete_deltas(self):
        self.bundle(scenario_names()[0])
        self.junit()
        summary = render(self.artifacts, self.root / "partial", self.fixtures, "test")
        self.assertEqual(summary["campaign_state"], "failed")
        self.assertEqual(len(summary["missing_or_incomplete"]), 20)
        self.assertIsNone(summary["groups"][0]["delta_from_original"])
        self.assertTrue((self.root / "partial/index.html").is_file())

    def test_failed_child_check_cannot_be_reported_as_passed(self):
        self.bundle(scenario_names()[0])
        self.junit()
        path = self.fixtures / "junit.xml"
        tree = ET.parse(path)
        ET.SubElement(tree.getroot()[-1], "failure", message="failed CSE task")
        tree.write(path)
        summary = render(self.artifacts, self.root / "child-failed", self.fixtures, "test")
        self.assertEqual(summary["junit_outcomes"][0]["status"], "failed")

    def test_duplicate_artifacts_and_wrong_matrix_are_rejected(self):
        name = scenario_names()[0]
        self.bundle(name)
        duplicate = self.artifacts / "another-logs" / name / "boot-trace"
        duplicate.mkdir(parents=True)
        (duplicate / "result.json").write_text('{"status":"collected"}')
        with self.assertRaisesRegex(ValueError, "Duplicate capture"):
            render(self.artifacts, self.root / "duplicate", self.fixtures, "test")
        self.manifest["scenario_names"].pop()
        (self.fixtures / "fixture.json").write_text(json.dumps(self.manifest))
        with self.assertRaisesRegex(ValueError, "21 CA matrix"):
            render(self.artifacts, self.root / "wrong", self.fixtures, "test")


if __name__ == "__main__":
    unittest.main()
