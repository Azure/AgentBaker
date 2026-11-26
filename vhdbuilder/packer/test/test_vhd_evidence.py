import importlib.util
import json
import tempfile
import unittest
from pathlib import Path
from unittest import mock


SCRIPT = Path(__file__).parents[1] / "collect_vhd_evidence.py"
SPEC = importlib.util.spec_from_file_location("collect_vhd_evidence", SCRIPT)
evidence_module = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(evidence_module)


class PackageQueryTests(unittest.TestCase):
    def test_dpkg_query_keeps_installed_packages_and_full_versions(self):
        output = (
            "installed\taz-oras\t1.3.6-ubuntu22.04u2\tamd64\n"
            "config-files\told-package\t1.0\tall\n"
        )
        runner = mock.Mock(return_value=mock.Mock(stdout=output))

        packages = evidence_module.run_package_query("dpkg", runner)

        self.assertEqual(
            packages,
            [
                {
                    "name": "az-oras",
                    "version": "1.3.6-ubuntu22.04u2",
                    "architecture": "amd64",
                    "package_manager": "dpkg",
                }
            ],
        )
        self.assertIn("${Version}", runner.call_args.args[0][-1])

    def test_rpm_query_preserves_nonzero_epoch(self):
        runner = mock.Mock(
            return_value=mock.Mock(
                stdout=(
                    "kernel\t0\t6.6.1\t1.azl3\tx86_64\n"
                    "kernel\t0\t6.6.2\t1.azl3\tx86_64\n"
                    "containerd2\t2\t2.1.0\t3.azl3\tx86_64\n"
                )
            )
        )

        packages = evidence_module.run_package_query("rpm", runner)

        versions = {(package["name"], package["version"]) for package in packages}
        self.assertEqual(
            versions,
            {
                ("kernel", "6.6.1-1.azl3"),
                ("kernel", "6.6.2-1.azl3"),
                ("containerd2", "2:2.1.0-3.azl3"),
            },
        )
        self.assertTrue(all(package["architecture"] == "x86_64" for package in packages))

    def test_collect_marks_unimplemented_sections_as_not_collected(self):
        with tempfile.TemporaryDirectory() as temporary:
            os_release_path = Path(temporary) / "os-release"
            os_release_path.write_text('ID="ubuntu"\nVERSION_ID="24.04"\n', encoding="utf-8")
            runner = mock.Mock(
                return_value=mock.Mock(stdout="installed\tbase-files\t13ubuntu10\tamd64\n")
            )

            evidence = evidence_module.collect_evidence(
                {
                    "IMAGE_VERSION": "202611.25.0",
                    "SKU_NAME": "24.04",
                    "AGENTBAKER_COMMIT": "abc123",
                    "ADO_BUILD_ID": "42",
                    "ADO_BUILD_NUMBER": "build-42",
                },
                str(os_release_path),
                runner,
            )

        self.assertEqual(evidence["image"]["distribution_version"], "24.04")
        self.assertEqual(evidence["coverage"]["os_packages"], "complete")
        self.assertEqual(evidence["coverage"]["components"], "not_collected")
        self.assertEqual(evidence["source_artifacts"], [])
        evidence_module.validate_evidence(evidence)

    def test_validation_rejects_dangling_artifact_references(self):
        with mock.patch.object(
            evidence_module,
            "read_os_release",
            return_value={"ID": "ubuntu", "VERSION_ID": "24.04"},
        ), mock.patch.object(
            evidence_module,
            "run_package_query",
            return_value=[
                {
                    "name": "base-files",
                    "version": "13",
                    "architecture": "amd64",
                    "package_manager": "dpkg",
                }
            ],
        ):
            evidence = evidence_module.collect_evidence({})
        evidence["components"].append(
            {
                "id": "oras",
                "name": "oras",
                "requested_version": "1.3.6",
                "artifact_ids": ["missing"],
                "locations": [],
            }
        )
        evidence["coverage"]["components"] = "partial"

        with self.assertRaisesRegex(ValueError, "unknown source artifact"):
            evidence_module.validate_evidence(evidence)

    def test_schema_is_valid_json_and_versions_match_collector(self):
        schema_path = SCRIPT.with_name("vhd-evidence.schema.json")
        schema = json.loads(schema_path.read_text(encoding="utf-8"))

        self.assertEqual(schema["properties"]["schema_version"]["const"], evidence_module.SCHEMA_VERSION)
        self.assertEqual(schema["$schema"], "https://json-schema.org/draft/2020-12/schema")


if __name__ == "__main__":
    unittest.main()
