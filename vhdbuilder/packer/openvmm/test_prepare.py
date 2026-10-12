"""Unprivileged contract tests. All fixture files stay under the repository."""

import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tomllib
import unittest
from unittest import mock
import uuid

from prepare import prepare
from validate import (CONFIG, PREFIX, REQUIRED, validate_boot_evidence,
                      validate_bundle, validate_installed_host)

KERNEL_RELEASE = "6.18.34.mshv3-2.azl3"
HYPERV_RUNNING_VERSION = "10.0.26100.9606-9-0"
PACKAGES = {
    "kernel-mshv": "0:6.18.34.mshv3-2.azl3.x86_64",
    "mshv": "2:26100.9606.2609291929.1-1.azl3.x86_64",
    "mshv-bootloader-lx": "2:26100.9606.2609291929.1-1.azl3.x86_64",
    "mshv-kd": "2:26100.9606.2609291929.1-1.azl3.x86_64",
    "openvmm": "0:0.2.0-2.azl3.x86_64",
}


class BundleTests(unittest.TestCase):
    def setUp(self):
        self.root = Path("vhdbuilder/packer/openvmm") / (".test-" + uuid.uuid4().hex)
        self.bundle = self.root / "artifacts"
        self.bundle.mkdir(parents=True)
        self.addCleanup(shutil.rmtree, self.root)
        for name in REQUIRED:
            (self.bundle / name).write_text("test fixture, not an executable\n")
        (self.bundle / "measurements").mkdir()
        self.manifest = {
            "unsigned_test_only": True, "kernel_release": KERNEL_RELEASE,
            "hyperv_running_version": HYPERV_RUNNING_VERSION,
            "packages": dict(PACKAGES),
            "erofs_default_size_zero": "directory-backed-no-fsmerge",
            "measurements": {},
        }
        for key in ("package_source", "guest_kernel_package", "containerd_source", "containerd_version",
                    "erofs_utils_source", "kata_source"):
            self.manifest[key] = "fixture provenance"
        self.manifest["containerd_version"] = "containerd fixture v2.2.0 fixture"
        for variant in ("production", "debug"):
            self.manifest["measurements"][variant] = {}
            for kind in ("json", "cbor", "idblocks"):
                name = f"measurements/{variant}.{kind}"
                self.manifest["measurements"][variant][kind] = name
                (self.bundle / name).write_text("native measurement fixture")
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        for debug in (False, True):
            suffix = "-debug" if debug else ""
            (self.bundle / f"{CONFIG}{suffix}.toml").write_text(f'''
[hypervisor.openvmm]
path = "{PREFIX}/bin/openvmm"
image = "{PREFIX}/share/kata-containers/kata-containers.img"
igvm = "{PREFIX}/share/kata-containers/kata-openvmm-snp{suffix}.bin"
default_memory = 1024
default_vcpus = 1
overhead_memory = 0
overhead_vcpus = 0
confidential_guest = true
sev_snp_guest = true
enable_debug = {str(debug).lower()}
[runtime]
hypervisor_name = "openvmm"
static_sandbox_resource_mgmt = true
static_sandbox_default_workload_vcpus = 1
static_sandbox_default_workload_mem = 1024
enable_debug = {str(debug).lower()}
''')
        self.checksum()

    def checksum(self):
        paths = sorted(p for p in self.bundle.rglob("*") if p.is_file() and p.name != "SHA256SUMS")
        (self.bundle / "SHA256SUMS").write_text("".join(
            f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.relative_to(self.bundle)}\n" for p in paths))

    def test_normal_template_not_modified(self):
        original = Path("vhdbuilder/packer/vhd-image-builder-mariner.json").read_bytes()
        output = self.root / "packer.json"
        prepare(self.bundle, output)
        template = json.loads(output.read_text())
        self.assertEqual(Path("vhdbuilder/packer/vhd-image-builder-mariner.json").read_bytes(), original)
        self.assertEqual(template["variables"]["feature_flags"], "kata")
        self.assertEqual(template["variables"]["os_version"], "{{env `OS_VERSION`}}")
        self.assertEqual(template["variables"]["openvmm_unsigned_test_only"], "true")
        self.assertNotIn("openvmm_trusted_host_manifest_sha256", template["variables"])
        provisioners = template["provisioners"]
        install = next(i for i, p in enumerate(provisioners) if "/openvmm-integration/install.sh" in str(p))
        check = next(i for i, p in enumerate(provisioners) if "/integration/check-image.sh" in str(p))
        self.assertNotIn("/integration/check-host.sh", str(provisioners))
        self.assertTrue(any(p.get("expect_disconnect") for p in provisioners[install:check]))
        self.assertEqual(next(p["source"] for p in provisioners if p.get("destination") ==
                              "/home/packer/kata-containers.img"), "kata-containers.img")
        with self.assertRaises(FileExistsError):
            prepare(self.bundle, output)

    @unittest.skipUnless(os.environ.get("PACKER"), "set PACKER to validate generated template syntax")
    def test_packer_syntax(self):
        output = self.root / "packer.json"
        prepare(self.bundle, output)
        subprocess.run([os.environ["PACKER"], "validate", "-syntax-only", str(output)], check=True)

    def test_missing_package_pin_blocks_prepare(self):
        del self.manifest["packages"]["kernel-mshv"]
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "packages must pin"):
            prepare(self.bundle, self.root / "packer.json")
        self.assertFalse((self.root / "packer.json").exists())

    def test_mismatched_checksum(self):
        (self.bundle / "ctr").write_text("modified")
        with self.assertRaisesRegex(ValueError, "checksum mismatch"):
            validate_bundle(self.bundle)

    def test_missing_manifest_coverage(self):
        (self.bundle / "SHA256SUMS").write_text("")
        with self.assertRaisesRegex(ValueError, "cover every"):
            validate_bundle(self.bundle)

    def test_stock_kernel_rejected(self):
        self.manifest["kernel_release"] = "stock-al3"
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "kernel-mshv package"):
            validate_bundle(self.bundle)

    def test_unsupported_containerd_version_rejected(self):
        self.manifest["containerd_version"] = "containerd fixture v1.7.0 fixture"
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "containerd 2.x"):
            validate_bundle(self.bundle)

    def test_mixed_hyperv_build_rejected(self):
        self.manifest["packages"]["mshv-kd"] = "2:26100.1-1.azl3.x86_64"
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "same version"):
            validate_bundle(self.bundle)

    def test_package_architecture_rejected(self):
        self.manifest["packages"]["openvmm"] = "0:0.2.0-2.azl3.aarch64"
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "epoch:version-release.x86_64"):
            validate_bundle(self.bundle)

    def test_missing_package_provenance_rejected(self):
        del self.manifest["package_source"]
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "provenance: package_source"):
            validate_bundle(self.bundle)

    def test_raw_runtime_bundle_rejected(self):
        (self.bundle / "openvmm").write_text("obsolete raw runtime")
        self.checksum()
        with self.assertRaisesRegex(ValueError, "obsolete package-provided"):
            validate_bundle(self.bundle)

    def test_nested_boot_entry(self):
        source = Path("vhdbuilder/packer/openvmm/grub-menuentry.sh").read_text()
        source = source.replace("/etc/grub.d/50_mariner_mshv_menuentry", "cat")
        for flag in ("", " MSHV_FORCE_NESTED=FALSE", " MSHV_FORCE_NESTED=TRUE"):
            with self.subTest(flag=flag):
                entry = ('menuentry "Dom0" {\n'
                         'chainloader /HvLoader.efi lxhvloader.dll MSHV_ENABLE=TRUE'
                         + flag + ' MSHV_SEV_SNP=TRUE\n}\n')
                output = subprocess.check_output(["bash", "-c", source], input=entry, text=True)
                self.assertIn('menuentry "OpenVMM SNP"', output)
                self.assertEqual(output.count("MSHV_FORCE_NESTED=TRUE"), 1)
                self.assertNotIn("MSHV_FORCE_NESTED=FALSE", output)
                self.assertIn("MSHV_SEV_SNP=TRUE", output)

    def test_map_preserves_existing_builder_settings(self):
        settings = self.root / "settings.json"
        settings.write_text(json.dumps({
            "subscription_id": "035db282-f1c8-4ce7-b78f-2a7265d5398c",
            "gallery_subscription_id": "035db282-f1c8-4ce7-b78f-2a7265d5398c",
            "vm_size": "Standard_D4ads_v5", "resource_group_name": "MarinerAKSTest",
        }))
        output = self.root / "map.json"
        prepare(self.bundle, output, settings, "123456")
        template = json.loads(output.read_text())
        builder = template["builders"][0]
        self.assertEqual(builder.pop("temp_resource_group_name"), "pkr-openvmm-123456")
        self.assertEqual(builder.pop("temp_compute_name"), "openvmm-123456")
        original = json.loads(Path("vhdbuilder/packer/vhd-image-builder-mariner.json").read_text())
        self.assertEqual(builder, original["builders"][0])
        self.assertEqual(template["variables"]["openvmm_install_host"], "true")
        self.assertEqual(template["variables"]["openvmm_build_vm_name"], "openvmm-123456")
        if os.environ.get("PACKER"):
            subprocess.run([os.environ["PACKER"], "validate", "-syntax-only", str(output)], check=True)

    def test_map_settings_cannot_redirect_subscription(self):
        settings = self.root / "redirect.json"
        settings.write_text(json.dumps({"subscription_id": "another-subscription"}))
        with self.assertRaisesRegex(ValueError, "MAP test subscription"):
            prepare(self.bundle, self.root / "redirect-packer.json", settings, "123456")
        self.assertFalse((self.root / "redirect-packer.json").exists())

    def test_map_build_id_required(self):
        settings = self.root / "settings.json"
        settings.write_text(json.dumps({"subscription_id": "035db282-f1c8-4ce7-b78f-2a7265d5398c"}))
        for build_id in (None, "", "0", "123;echo unsafe"):
            with self.subTest(build_id=build_id), self.assertRaisesRegex(ValueError, "numeric MAP build ID"):
                prepare(self.bundle, self.root / "invalid.json", settings, build_id)

    def test_installer_requires_explicit_host_install_approval(self):
        environment = dict(os.environ, OPENVMM_UNSIGNED_TEST_ONLY="true")
        environment.pop("OPENVMM_INSTALL_HOST", None)
        result = subprocess.run(["bash", "vhdbuilder/packer/openvmm/install.sh", str(self.bundle)],
                                env=environment, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("disposable evaluation VM approval", result.stderr)

    def test_installed_host_gate_is_read_only(self):
        def query(args, **kwargs):
            self.assertEqual(args[0], "rpm")
            if "-qf" in args:
                return "openvmm"
            return next(version + "\n" for name, version in PACKAGES.items()
                        if args[-1] == f"{name}-{version}")
        with mock.patch("validate.subprocess.check_output", side_effect=query), \
                mock.patch("validate.subprocess.run",
                           return_value=subprocess.CompletedProcess(["rpm"], 0, "", "")) as verify:
            validate_installed_host(self.bundle / "bundle.json")
            self.assertEqual(verify.call_args.args[0][:2], ["rpm", "-V"])
        with mock.patch("validate.subprocess.check_output", return_value="wrong-version"):
            with self.assertRaisesRegex(ValueError, "does not match"):
                validate_installed_host(self.bundle / "bundle.json")

    def test_installed_payload_tampering_rejected(self):
        def query(args, **kwargs):
            if "-qf" in args:
                return "openvmm"
            return next(version + "\n" for name, version in PACKAGES.items()
                        if args[-1] == f"{name}-{version}")
        with mock.patch("validate.subprocess.check_output", side_effect=query), \
                mock.patch("validate.subprocess.run", return_value=subprocess.CompletedProcess(
                    ["rpm"], 1, "S.5...... /usr/bin/openvmm\n", "")):
            with self.assertRaisesRegex(ValueError, "payload verification failed"):
                validate_installed_host(self.bundle / "bundle.json")

    def test_post_reboot_evidence(self):
        log = (f"Hyper-V: Hypervisor Build {HYPERV_RUNNING_VERSION}\n"
               "Hyper-V: running as root partition\n"
               "misc mshv: SEV-SNP support status: available\n")
        validate_boot_evidence(self.manifest, KERNEL_RELEASE, "new-boot", "old-boot", log)
        cases = [
            (KERNEL_RELEASE, "old-boot", log, "changed boot ID"),
            ("6.18.34.mshv3", "new-boot", log, "kernel-mshv package"),
            (KERNEL_RELEASE, "new-boot", log.replace("root partition", "child partition"), "root partition"),
            (KERNEL_RELEASE, "new-boot", log.replace(HYPERV_RUNNING_VERSION, "10.0.1"), "pinned Hyper-V"),
            (KERNEL_RELEASE, "new-boot", log.replace("status: available", "status: unavailable"), "SEV-SNP"),
        ]
        for kernel, boot_id, evidence, message in cases:
            with self.subTest(message=message), self.assertRaisesRegex(ValueError, message):
                validate_boot_evidence(self.manifest, kernel, boot_id, "old-boot", evidence)

    def test_debug_igvm_required(self):
        config = self.bundle / f"{CONFIG}-debug.toml"
        config.write_text(config.read_text().replace("kata-openvmm-snp-debug.bin", "kata-openvmm-snp.bin"))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "must select"):
            validate_bundle(self.bundle)

    def test_zero_boot_memory_rejected(self):
        config = self.bundle / f"{CONFIG}.toml"
        config.write_text(config.read_text().replace("default_memory = 1024", "default_memory = 0"))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "default_memory=1024"):
            validate_bundle(self.bundle)

    def test_workload_defaults_must_be_in_runtime(self):
        config = self.bundle / f"{CONFIG}.toml"
        defaults = ("static_sandbox_default_workload_vcpus = 1\n"
                    "static_sandbox_default_workload_mem = 1024\n")
        text = config.read_text().replace(defaults, "")
        config.write_text(text.replace("[runtime]", defaults + "[runtime]"))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "runtime.static_sandbox_default"):
            validate_bundle(self.bundle)

    def test_snp_flags_required(self):
        config = self.bundle / f"{CONFIG}.toml"
        original = config.read_text()
        for key in ("confidential_guest", "sev_snp_guest"):
            with self.subTest(key=key):
                config.write_text(original.replace(f"{key} = true", f"{key} = false"))
                self.checksum()
                with self.assertRaisesRegex(ValueError, key):
                    validate_bundle(self.bundle)

    def test_external_symlink_rejected(self):
        (self.bundle / "ctr").unlink()
        (self.bundle / "ctr").symlink_to("../artifacts/containerd")
        self.checksum()
        with self.assertRaisesRegex(ValueError, "regular bundle"):
            validate_bundle(self.bundle)

    def test_unsigned_opt_in_required(self):
        self.manifest["unsigned_test_only"] = False
        (self.bundle / "bundle.json").write_text(json.dumps(self.manifest))
        self.checksum()
        with self.assertRaisesRegex(ValueError, "unsigned_test_only"):
            validate_bundle(self.bundle)

    def test_installer_rejects_without_optin_before_host_changes(self):
        environment = dict(os.environ)
        environment.pop("OPENVMM_UNSIGNED_TEST_ONLY", None)
        result = subprocess.run(["bash", "vhdbuilder/packer/openvmm/install.sh", str(self.bundle)],
                                env=environment, capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("without explicit test-only opt-in", result.stderr)

    def test_make_rejects_optin_without_artifacts(self):
        result = subprocess.run([
            "make", "-f", "packer.mk", "prepare-openvmm-test",
            "OPENVMM_UNSIGNED_TEST_ONLY=true", "OPENVMM_ARTIFACT_DIR=",
        ], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("OPENVMM_ARTIFACT_DIR is required", result.stdout)

    def test_make_accepts_real_pipeline_os_label(self):
        for os_version in ("V3", "V3kata", "3.0"):
            with self.subTest(os_version=os_version):
                result = subprocess.run([
                    "make", "-f", "packer.mk", "prepare-openvmm-test",
                    "OPENVMM_UNSIGNED_TEST_ONLY=true", f"OPENVMM_ARTIFACT_DIR={self.root}/missing",
                    f"OPENVMM_MAP_SETTINGS={self.root}/settings.json", "BUILD_ID=123456",
                    "OS_SKU=AzureLinux", f"OS_VERSION={os_version}", "ARCHITECTURE=X86_64",
                    "HYPERV_GENERATION=V2", "FEATURE_FLAGS=kata",
                ], capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("bundle.json", result.stderr)
                self.assertNotIn("requires Azure Linux 3", result.stdout)

    def test_make_rejects_wrong_os_or_non_kata_feature(self):
        for os_version, feature in (("V2", "kata"), ("V3", "None")):
            with self.subTest(os_version=os_version, feature=feature):
                result = subprocess.run([
                    "make", "-f", "packer.mk", "prepare-openvmm-test",
                    "OPENVMM_UNSIGNED_TEST_ONLY=true", f"OPENVMM_ARTIFACT_DIR={self.bundle}",
                    "BUILD_ID=123456", "OS_SKU=AzureLinux", f"OS_VERSION={os_version}",
                    "ARCHITECTURE=X86_64", "HYPERV_GENERATION=V2", f"FEATURE_FLAGS={feature}",
                ], capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("Traceback", result.stderr)
                self.assertIn("OpenVMM", result.stdout)

    def test_make_requires_build_id(self):
        result = subprocess.run([
            "make", "-f", "packer.mk", "prepare-openvmm-test",
            "OPENVMM_UNSIGNED_TEST_ONLY=true", f"OPENVMM_ARTIFACT_DIR={self.bundle}",
            "BUILD_ID=",
            "OS_SKU=AzureLinux", "OS_VERSION=V3kata", "ARCHITECTURE=X86_64",
            "HYPERV_GENERATION=V2", "FEATURE_FLAGS=kata",
        ], capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("BUILD_ID is required", result.stdout)

    def test_make_settings_alone_does_not_enable_host_install(self):
        result = subprocess.run([
            "make", "-f", "packer.mk", "prepare-openvmm-test",
            f"OPENVMM_MAP_SETTINGS={self.root}/settings.json",
            "OPENVMM_UNSIGNED_TEST_ONLY=", "OPENVMM_ARTIFACT_DIR=",
        ], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_make_passes_map_settings(self):
        settings = self.root / "settings.json"
        settings.write_text(json.dumps({
            "subscription_id": "035db282-f1c8-4ce7-b78f-2a7265d5398c",
            "vm_size": "Standard_D4ads_v5",
        }))
        output = Path("vhdbuilder/packer/openvmm-test.generated.json")
        self.assertFalse(output.exists(), "refusing to overwrite an existing generated template")
        self.addCleanup(output.unlink, missing_ok=True)
        result = subprocess.run([
            "make", "-f", "packer.mk", "prepare-openvmm-test",
            "OPENVMM_UNSIGNED_TEST_ONLY=true", f"OPENVMM_ARTIFACT_DIR={self.bundle}",
            f"OPENVMM_MAP_SETTINGS={settings}", "BUILD_ID=123456",
            "OS_SKU=AzureLinux", "OS_VERSION=V3", "ARCHITECTURE=X86_64",
            "HYPERV_GENERATION=V2", "FEATURE_FLAGS=kata",
        ], capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        template = json.loads(output.read_text())
        self.assertEqual(template["variables"]["openvmm_install_host"], "true")
        self.assertEqual(template["builders"][0]["temp_compute_name"], "openvmm-123456")
        self.assertIn("shared_image_gallery_destination", template["builders"][0])

    def test_image_check_does_not_require_snp_hardware(self):
        image = Path("vhdbuilder/packer/openvmm/check-image.sh").read_text()
        for command in ("modprobe ", "/dev/mshv", "--boot-evidence", "dmesg"):
            self.assertNotIn(command, image)
        self.assertIn('modinfo -k "$expected"', image)
        host = Path("vhdbuilder/packer/openvmm/check-host.sh").read_text()
        self.assertIn("--boot-evidence", host)
        self.assertIn("/dev/mshv", host)
        self.assertIn('integration/check-image.sh', host)

    def test_installer_uses_agentbaker_package_manager(self):
        script = Path("vhdbuilder/packer/openvmm/install.sh").read_text()
        self.assertIn('dnf install -y "${packages[@]}"', script)
        self.assertIn('dnf upgrade -y "${packages[@]}"', script)
        self.assertNotIn("tdnf ", script)

    def test_kernel_module_checks_use_packaged_driver_name(self):
        for name in ("install.sh", "check-image.sh", "check-host.sh"):
            with self.subTest(script=name):
                text = Path("vhdbuilder/packer/openvmm", name).read_text()
                self.assertIn("for module in mshv_root loop dm_mod dm_zero dm_verity erofs overlay;", text)
                self.assertNotIn("for module in mshv ", text)

    def test_containerd_contract(self):
        path = Path("vhdbuilder/packer/openvmm/containerd-v2.toml")
        variants = [path.read_text(), subprocess.check_output([
            "sed", "-e", 's/\\[plugins."io.containerd.cri.v1.images"\\]/[plugins."io.containerd.grpc.v1.cri".containerd]/',
            "-e", "s/io.containerd.cri.v1.runtime/io.containerd.grpc.v1.cri/g",
            "-e", "/^  use_local_image_pull = /d", str(path),
        ], text=True) + '[plugins."io.containerd.cri.v1.images"]\n  use_local_image_pull = false\n']
        for text in variants:
            plugins = tomllib.loads(text)["plugins"]
            runtime_plugin = ("io.containerd.cri.v1.runtime"
                              if "io.containerd.cri.v1.runtime" in plugins
                              else "io.containerd.grpc.v1.cri")
            runtimes = plugins[runtime_plugin]["containerd"]["runtimes"]
            self.assertEqual(runtimes["kata-cc"], runtimes["kata-openvmm"])
            self.assertIs(plugins["io.containerd.cri.v1.images"]["use_local_image_pull"], False)
            self.assertEqual(plugins["io.containerd.differ.v1.erofs"], {
                "enable_dmverity": True, "enable_tar_index": False,
                "mkfs_options": ["-T0", "--mkfs-time", "--sort=none"],
            })
            self.assertEqual(plugins["io.containerd.snapshotter.v1.erofs"]["default_size"], "0")
            self.assertEqual(plugins["io.containerd.snapshotter.v1.erofs"]["dmverity_mode"], "on")
            self.assertEqual(plugins["io.containerd.service.v1.diff-service"]["default"], ["erofs"])
            self.assertEqual(plugins["io.containerd.transfer.v1.local"]["unpack_config"], [{
                "platform": "linux/amd64", "snapshotter": "erofs", "differ": "erofs", "optional": False,
            }])

    def test_optional_snapshotter_mode_preserves_required_dmverity(self):
        text = subprocess.check_output([
            "sed", "/^  dmverity_mode = /d", "vhdbuilder/packer/openvmm/containerd-v2.toml",
        ], text=True)
        plugins = tomllib.loads(text)["plugins"]
        self.assertNotIn("dmverity_mode", plugins["io.containerd.snapshotter.v1.erofs"])
        self.assertEqual(plugins["io.containerd.snapshotter.v1.erofs"]["default_size"], "0")
        self.assertIs(plugins["io.containerd.differ.v1.erofs"]["enable_dmverity"], True)
        self.assertEqual(plugins["io.containerd.service.v1.diff-service"]["default"], ["erofs"])
        self.assertEqual(plugins["io.containerd.transfer.v1.local"]["unpack_config"][0]["differ"], "erofs")


if __name__ == "__main__":
    unittest.main()
