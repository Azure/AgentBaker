import copy
import importlib.util
import io
import json
import pathlib
import sys
import tempfile
import unittest
from contextlib import redirect_stdout
from unittest.mock import patch


SCRIPT = pathlib.Path(__file__).with_name("validate_previous_version.py")
SPEC = importlib.util.spec_from_file_location("validate_previous_version", SCRIPT)
validator = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = validator
SPEC.loader.exec_module(validator)


class VersionParserTests(unittest.TestCase):
    def test_parses_ubuntu_revision(self):
        parsed = validator.parse_version("1.35.7-ubuntu24.04u10")

        self.assertIsNotNone(parsed)
        self.assertEqual((1, 35, 7), parsed.release)
        self.assertEqual((24, 4), parsed.ubuntu_release)
        self.assertEqual(10, parsed.build)

    def test_ubuntu_revisions_are_numeric(self):
        versions = ["1.35.7-ubuntu24.04u1", "1.35.7-ubuntu24.04u10"]
        highest = validator.find_highest_build(
            versions, validator.parse_version(versions[0]).release_key
        )

        self.assertEqual(10, highest.build)

    def test_formats_ubuntu_recommendation(self):
        parsed = validator.parse_version("1.35.7-ubuntu24.04u10")
        like = validator.parse_version("1.35.6-ubuntu24.04u2")

        self.assertEqual(
            "1.35.7-ubuntu24.04u10",
            validator.format_recommendation(parsed, like),
        )

    def test_preserves_numeric_revision_behavior(self):
        parsed = validator.parse_version("v0.1.16-16.azl3")

        self.assertEqual((0, 1, 16), parsed.release)
        self.assertEqual(16, parsed.build)
        self.assertEqual("v0.1.16-16.azl3", validator.format_recommendation(parsed, parsed))

    def test_skips_ambiguous_formats(self):
        self.assertIsNone(validator.parse_version("10.0.20348.5622"))
        self.assertIsNone(validator.parse_version("1.35.7"))


class ChangeDetectionTests(unittest.TestCase):
    def setUp(self):
        self.tag = "OCI_registry=https://mcr.microsoft.com, name=oss/v2/kubernetes/kubelet-sysext"
        self.versions = [
            {
                "k8sVersion": minor,
                "renovateTag": self.tag,
                "latestVersion": f"v{minor}.2-2-azlinux3",
                "previousLatestVersion": f"v{minor}.2-1-azlinux3",
            }
            for minor in ("1.34", "1.35")
        ]
        self.base = {
            "Packages": [
                {
                    "name": name,
                    "downloadURIs": {
                        "flatcar": {"current": {"versionsV2": copy.deepcopy(self.versions)}}
                    },
                }
                for name in ("kubelet", "kubectl")
            ]
        }

    def validate(self, head, available_versions=()):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "components.json"
            suggestions_path = pathlib.Path(directory) / "suggestions.json"
            path.write_text(json.dumps(head, indent=2) + "\n", encoding="utf-8")
            with (
                patch.object(sys, "argv", [
                    str(SCRIPT), "--base", "base", "--file", str(path),
                    "--suggestions", str(suggestions_path),
                ]),
                patch.object(validator, "load_json_at_ref", return_value=self.base),
                patch.object(validator, "lookup_available_versions", return_value=available_versions) as lookup,
                redirect_stdout(io.StringIO()),
            ):
                result = validator.main()
            self.suggestions = json.loads(suggestions_path.read_text(encoding="utf-8"))
            self.head_lines = path.read_text(encoding="utf-8").splitlines()
            return result, lookup.call_count

    def test_unrelated_package_insertion_and_reordering_are_unchanged(self):
        head = copy.deepcopy(self.base)
        head["Packages"].reverse()
        head["Packages"].insert(0, {
            "name": "azure-acr-credential-provider",
            "downloadURIs": {"flatcar": {"current": {"versionsV2": [
                {"latestVersion": "v1.35.2-1-azlinux3"}
            ]}}},
        })

        self.assertEqual((0, 0), self.validate(head))

    def test_version_insertion_and_reordering_with_shared_tag_are_unchanged(self):
        head = copy.deepcopy(self.base)
        versions = head["Packages"][0]["downloadURIs"]["flatcar"]["current"]["versionsV2"]
        versions.reverse()
        versions.insert(0, {
            "k8sVersion": "1.36",
            "renovateTag": self.tag,
            "latestVersion": "v1.36.2-1-azlinux3",
        })

        self.assertEqual((0, 0), self.validate(head))

    def test_version_changes_with_shared_tag_are_checked(self):
        for field in ("latestVersion", "previousLatestVersion"):
            with self.subTest(field=field):
                head = copy.deepcopy(self.base)
                versions = head["Packages"][0]["downloadURIs"]["flatcar"]["current"]["versionsV2"]
                versions.reverse()
                versions[1][field] = "v1.34.2-3-azlinux3"

                self.assertEqual((1, 1), self.validate(head))

    def test_new_component_and_new_minor_are_checked(self):
        for new_component in (False, True):
            with self.subTest(new_component=new_component):
                head = copy.deepcopy(self.base)
                if new_component:
                    package = copy.deepcopy(head["Packages"][0])
                    package["name"] = "new-component"
                    head["Packages"].insert(0, package)
                    expected = 2
                else:
                    versions = head["Packages"][0]["downloadURIs"]["flatcar"]["current"]["versionsV2"]
                    version = copy.deepcopy(versions[0])
                    version["k8sVersion"] = "1.36"
                    versions.insert(0, version)
                    expected = 1

                self.assertEqual((1, expected), self.validate(head))

    def test_identity_distinguishes_os_release_and_windows_selectors(self):
        entry = self.versions[0]
        data = {"Packages": [{
            "name": "kubelet",
            "downloadURIs": {
                os_name: {
                    release: {"versionsV2": [dict(entry, windowsSkuMatch=sku) for sku in ("2022*", "2025*")]}
                    for release in ("current", "r2404")
                }
                for os_name in ("flatcar", "ubuntu")
            },
        }]}
        keys = [validator.entry_key(e) for e in validator.collect_entries(data)]

        self.assertEqual(8, len(set(keys)))

    def test_image_reordering_preserves_identity_and_head_suggestion_path(self):
        self.base = {"ContainerImages": [
            {"downloadURL": url, "multiArchVersionsV2": [copy.deepcopy(self.versions[index])]}
            for index, url in enumerate(("mcr.microsoft.com/first:*", "mcr.microsoft.com/second:*"))
        ]}
        head = copy.deepcopy(self.base)
        head["ContainerImages"].reverse()
        self.assertEqual((0, 0), self.validate(head))

        head["ContainerImages"][0]["multiArchVersionsV2"][0]["latestVersion"] = "v1.35.2-3-azlinux3"
        self.assertEqual(
            (1, 1), self.validate(head, available_versions=["v1.35.1-3-azlinux3"]),
        )
        line_number, raw_line = next(
            (i, line) for i, line in enumerate(self.head_lines, start=1)
            if '"previousLatestVersion": "v1.35.2-1-azlinux3"' in line
        )
        self.assertEqual([{
            "line": line_number,
            "new_line": raw_line.replace("v1.35.2-1-azlinux3", "v1.35.1-3-azlinux3"),
        }], self.suggestions)

    def test_rows_without_explicit_minor_selectors_are_not_overwritten(self):
        versions = copy.deepcopy(self.versions)
        for version in versions:
            del version["k8sVersion"]
        self.base = {"ContainerImages": [{
            "downloadURL": "mcr.microsoft.com/example:*",
            "windowsVersions": versions,
        }]}
        head = copy.deepcopy(self.base)
        head_versions = head["ContainerImages"][0]["windowsVersions"]
        head_versions.reverse()
        self.assertEqual((0, 0), self.validate(head))

        head_versions[0]["latestVersion"] = "v1.35.2-3-azlinux3"
        self.assertEqual((1, 1), self.validate(head))
        matches = validator.match_base_entries(
            validator.collect_entries(self.base), validator.collect_entries(head),
        )
        self.assertEqual("v1.35.2-2-azlinux3", matches[0].latest_version)

        head_versions.append(copy.deepcopy(head_versions[1]))
        self.assertEqual((1, 2), self.validate(head))

    def test_multiple_changed_rows_without_selectors_do_not_guess_base_release(self):
        head = copy.deepcopy(self.base)
        for data in (self.base, head):
            for package in data["Packages"]:
                for version in package["downloadURIs"]["flatcar"]["current"]["versionsV2"]:
                    del version["k8sVersion"]
                    if data is head:
                        version["latestVersion"] = version["latestVersion"].replace("-2-", "-3-")

        self.assertEqual((1, 4), self.validate(head))
        self.assertEqual(
            [None] * 4,
            validator.match_base_entries(
                validator.collect_entries(self.base), validator.collect_entries(head),
            ),
        )


if __name__ == "__main__":
    unittest.main()
