"""Offline tests for image lifecycle guards and collected E2E bundle selection."""

import copy
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from prepare_vhd import IMAGE_NAME, instrument_template, validate_environment
from render_e2e import render, select_trials


class VHDIntegrationTests(unittest.TestCase):
    def setUp(self):
        self.template = {
            "provisioners": [
                {"type": "shell", "inline": "sudo reboot"},
                {"type": "shell", "inline": ["/home/packer/post-install-dependencies.sh"]},
                {"type": "shell", "inline": [
                    "/opt/azure/containers/cleanup-vhd.sh",
                    "waagent -force -deprovision+user",
                ]},
            ],
        }
        self.environment = {
            "BOOT_TRACE_BACKEND": "ftrace", "MODE": "linuxVhdMode", "OS_SKU": "Ubuntu",
            "OS_VERSION": "26.04", "ARCHITECTURE": "X86_64", "HYPERV_GENERATION": "V2",
            "FEATURE_FLAGS": "minimal", "IMG_PUBLISHER": "Canonical",
            "IMG_OFFER": "ubuntu-26_04-lts", "IMG_SKU": "minimal", "IMG_VERSION": "26.04.20260701",
            "SIG_IMAGE_NAME": IMAGE_NAME + "ftrace",
            "GENERATE_PUBLISHING_INFO": "False", "RUN_PREFETCH_OPTIMIZATION": "False",
            "ENABLE_FIPS": "False", "ENABLE_TRUSTED_LAUNCH": "False",
        }
        self.settings = {"img_version": self.environment["IMG_VERSION"],
                         "sig_image_name": self.environment["SIG_IMAGE_NAME"]}

    def test_inserts_install_after_reboot_and_arms_after_last_cleanup(self):
        original = copy.deepcopy(self.template)
        for backend in ("ftrace", "lttng"):
            result = instrument_template(self.template, backend, "26.04.20260701", "test-image")
            commands = result["provisioners"]
            post = next(i for i, step in enumerate(commands)
                        if "/home/packer/post-install-dependencies.sh" in step.get("inline", []))
            self.assertIn("install.sh prepare " + backend, commands[post - 1]["inline"][0])
            self.assertEqual(commands[-1]["inline"][0], "/opt/azure/containers/cleanup-vhd.sh")
            self.assertIn("install.sh arm " + backend, commands[-1]["inline"][1])
            self.assertIn("deprovision", commands[-1]["inline"][2])
            self.assertEqual(self.template, original)

    def test_rejects_changed_lifecycle_or_later_reboot(self):
        for broken in (
            {"provisioners": []},
            {"provisioners": self.template["provisioners"] + [{"type": "shell", "inline": "sudo reboot"}]},
        ):
            with self.subTest(template=broken), self.assertRaises(ValueError):
                instrument_template(broken, "ftrace", "26.04.20260701", "test")

    def test_rejects_production_definition_publishing_and_base_drift(self):
        self.assertEqual(validate_environment(self.environment, self.settings), ("ftrace", "26.04.20260701"))
        changes = {
            "SIG_IMAGE_NAME": "2604minimalgen2containerd", "GENERATE_PUBLISHING_INFO": "True",
            "RUN_PREFETCH_OPTIMIZATION": "True", "IMG_VERSION": "latest",
            "OS_VERSION": "22.04", "ARCHITECTURE": "ARM64", "FEATURE_FLAGS": "minimal,cvm",
            "BOOT_TRACE_BACKEND": "both",
        }
        for key, value in changes.items():
            with self.subTest(key=key), self.assertRaises(ValueError):
                validate_environment(dict(self.environment, **{key: value}), self.settings)
        with self.assertRaises(ValueError):
            validate_environment(self.environment, dict(self.settings, img_version="26.04.20260702"))

    def test_actual_standard_template_has_supported_final_lifecycle(self):
        root = Path(__file__).resolve().parents[3]
        template = json.loads((root / "vhdbuilder/packer/vhd-image-builder-base.json").read_text())
        result = instrument_template(template, "ftrace", "26.04.20260701", "test")
        self.assertIn("install.sh arm ftrace", result["provisioners"][-1]["inline"][-2])


class E2EReportTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.bundles = {}
        for index, label in enumerate(("ftrace", "ftrace_2", "ftrace_3", "lttng")):
            backend = "lttng" if label == "lttng" else "ftrace"
            bundle = self.root / ("Ubuntu2604Minimal_BootTrace_" + label) / "boot-trace"
            bundle.mkdir(parents=True)
            self.bundles[label] = bundle
            self.write(bundle, "result.json", {"status": "collected"})
            self.write(bundle, "metadata.json", {
                "backend": backend, "boot_id": f"boot-{index}", "status": "stopped",
                "finalized_at": "test", "kernel": "test-kernel", "architecture": "x86_64",
                "clock_start": {"realtime": 1000.5, "monotonic": 0},
                "image_provenance": {
                    "backend": backend, "kernel": "test-kernel", "base_version": "26.04.20260701",
                },
            })
            self.write(bundle, "readiness.json", {
                "boot_id": f"boot-{index}", "node_uid": f"uid-{index}", "provider_id": f"vm-{index}",
                "configuration": {
                    "vm_size": "same", "os_sku": "Ubuntu", "kubernetes_version": "same",
                    "network_plugin": "kubenet", "canonical_base_version": "26.04.20260701",
                    "cluster_resource_id": "cluster", "provisioning": "AgentBaker E2E VMSS",
                },
            })

    def write(self, bundle, name, value):
        (bundle / name).write_text(json.dumps(value))

    def update(self, label, filename, mutate):
        bundle = self.bundles[label]
        value = json.loads((bundle / filename).read_text())
        mutate(value)
        self.write(bundle, filename, value)

    def test_selects_three_distinct_comparable_ftrace_nodes_and_optional_lttng(self):
        self.assertEqual(
            select_trials(self.root),
            [(label, self.bundles[label]) for label in ("ftrace", "ftrace_2", "ftrace_3", "lttng")],
        )
        self.bundles["lttng"].joinpath("result.json").unlink()
        self.assertEqual(
            select_trials(self.root),
            [(label, self.bundles[label]) for label in ("ftrace", "ftrace_2", "ftrace_3")],
        )
        self.bundles["ftrace_2"].joinpath("result.json").unlink()
        with self.assertRaises(ValueError):
            select_trials(self.root)

    def test_rejects_missing_incomplete_duplicate_and_incomparable_trials(self):
        changes = [
            ("result.json", lambda value: value.update(status="incomplete")),
            ("metadata.json", lambda value: value.update(boot_id="different")),
            ("metadata.json", lambda value: value.update(kernel="other-kernel")),
            ("readiness.json", lambda value: value.update(provider_id="vm-0")),
            ("readiness.json", lambda value: value["configuration"].update(vm_size="different")),
            ("readiness.json", lambda value: value["configuration"].update(canonical_base_version="different")),
        ]
        for filename, mutate in changes:
            original = (self.bundles["ftrace_2"] / filename).read_text()
            with self.subTest(filename=filename):
                self.update("ftrace_2", filename, mutate)
                with self.assertRaises(ValueError):
                    select_trials(self.root)
                (self.bundles["ftrace_2"] / filename).write_text(original)
        self.write(self.bundles["ftrace_2"], "metadata.json", json.loads(
            (self.bundles["ftrace"] / "metadata.json").read_text(),
        ))
        with self.assertRaises(ValueError):
            select_trials(self.root)

    def test_render_uses_shared_milestone_schema_and_self_contained_html(self):
        for bundle in self.bundles.values():
            self.write(bundle, "cse-boot-datapoints.json", {
                "BootDatapoints": {
                    "KernelStartTime": "1970-01-01T00:16:40Z",
                    "KubeletStartTime": "1970-01-01T00:16:42Z",
                },
            })

        def fake_analyze(bundle):
            return {
                "schema": 1, "metadata": json.loads((bundle / "metadata.json").read_text()),
                "milestones": [], "warnings": [],
                "ready_seconds": float(bundle.parent.name.rsplit("_", 1)[-1])
                if bundle.parent.name.endswith(("_2", "_3")) else 20.0,
            }

        output = self.root / "reports"
        with patch("render_e2e.analyze", side_effect=fake_analyze):
            render(self.root, output)
        result = json.loads((output / "ftrace/report.json").read_text())
        self.assertEqual(result["milestones"][0]["t"], 1.5)
        self.assertIn("Kubelet", result["milestones"][0]["name"])
        self.assertEqual(len(result["milestones"]), 1)
        self.assertTrue(any("before boot and was excluded" in item for item in result["warnings"]))
        self.assertTrue((output / "comparison.html").is_file())
        comparison = (output / "comparison.html").read_text()
        self.assertIn("connect-src 'none'", comparison)
        self.assertIn("ftrace 3", comparison)
        self.assertIn("trial-summary", comparison)
        self.assertIn("repeat-patterns", comparison)


if __name__ == "__main__":
    unittest.main()
