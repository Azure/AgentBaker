import json
import pathlib
import re
import unittest


ROOT = pathlib.Path(__file__).resolve().parents[2]
REPOSITORIES = {
    "oss/v2/kubernetes/kubelet-sysext",
    "oss/v2/kubernetes/kubectl-sysext",
    "oss/v2/kubernetes/azure-acr-credential-provider-sysext",
    "aks-secure-tls-bootstrap/v2/aks-secure-tls-bootstrap-client-sysext",
}


class DalecSysextVersionTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.config = json.loads((ROOT / ".github/renovate.json").read_text())
        cls.components_text = (ROOT / "parts/common/components.json").read_text()
        cls.components = json.loads(cls.components_text)
        cls.rule = next(
            rule
            for rule in cls.config["packageRules"]
            if set(rule.get("matchPackageNames", [])) == REPOSITORIES
        )
        cls.extract = re.compile(
            cls.rule["extractVersion"].replace("(?<version>", "(?P<version>")
        )

    def normalize(self, tag):
        match = self.extract.fullmatch(tag)
        return match["version"] if match else None

    def test_revision_only_publications_have_the_same_managed_version(self):
        self.assertEqual(self.normalize("v1.34.11-9-azlinux3-x86-64"), "v1.34.11")
        self.assertEqual(self.normalize("v1.34.11-10-azlinux3-x86-64"), "v1.34.11")
        self.assertEqual(self.rule["versioning"], "semver")
        self.assertNotIn("matchCurrentVersion", self.rule)

    def test_patch_updates_change_the_managed_version_without_the_revision(self):
        self.assertEqual(self.normalize("v1.34.12-1-azlinux3-x86-64"), "v1.34.12")
        self.assertNotEqual(
            self.normalize("v1.34.11-99-azlinux3-x86-64"),
            self.normalize("v1.34.12-1-azlinux3-x86-64"),
        )

    def test_extraction_excludes_other_os_architecture_and_non_numeric_revisions(self):
        for tag in (
            "v1.34.11-1-azlinux2-x86-64",
            "v1.34.11-1-azlinux3-arm64",
            "v1.34.11-preview-azlinux3-x86-64",
            "v1.34.11-1-azlinux3-x86-64-extra",
            "v1.34.11",
        ):
            with self.subTest(tag=tag):
                self.assertIsNone(self.normalize(tag))

    def test_only_the_four_dalec_components_are_normalized(self):
        found = set()
        for package in self.components["Packages"]:
            for distro, releases in package.get("downloadURIs", {}).items():
                for release in releases.values():
                    for entry in release.get("versionsV2", []):
                        renovate_tag = entry.get("renovateTag", "")
                        repository = renovate_tag.split(", name=")[-1]
                        if repository not in REPOSITORIES:
                            continue
                        found.add(package["name"])
                        self.assertEqual(distro, "flatcar")
                        for field in ("latestVersion", "previousLatestVersion"):
                            if field in entry:
                                self.assertRegex(entry[field], r"^v\d+\.\d+\.\d+$")
        self.assertEqual(
            found,
            {
                "kubelet",
                "kubectl",
                "azure-acr-credential-provider-pmc",
                "aks-secure-tls-bootstrap-client",
            },
        )

    def test_oci_manager_extracts_normalized_current_and_previous_versions(self):
        manager = next(
            manager
            for manager in self.config["customManagers"]
            if manager["description"] == "auto update OCI artifacts in components.json"
        )
        pattern = re.sub(r"\(\?<(\w+)>", r"(?P<\1>", manager["matchStrings"][0])
        matches = [
            match
            for match in re.finditer(pattern, self.components_text)
            if match["packageName"] in REPOSITORIES
        ]
        self.assertEqual({match["packageName"] for match in matches}, REPOSITORIES)
        for match in matches:
            self.assertRegex(match["currentValue"], r"^v\d+\.\d+\.\d+$")
            if match["depType"]:
                self.assertRegex(match["depType"], r"^v\d+\.\d+\.\d+$")


if __name__ == "__main__":
    unittest.main()
