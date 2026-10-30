import copy
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("windows_vhd_impact.py")
SPEC = importlib.util.spec_from_file_location("windows_vhd_impact", SCRIPT)
impact = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(impact)
ROOT = SCRIPT.parents[2]
COMPONENTS = json.loads((ROOT / impact.COMPONENTS_PATH).read_text())


class WindowsInputsTests(unittest.TestCase):
    def setUp(self):
        self.before = copy.deepcopy(COMPONENTS)
        self.after = copy.deepcopy(self.before)

    def assertImpact(self, expected):
        self.assertEqual(
            impact.windows_inputs(json.dumps(self.before))
            != impact.windows_inputs(json.dumps(self.after)), expected
        )

    def package(self, name):
        return next(p for p in self.after["Packages"] if p["name"] == name)

    def test_grid_update_from_pr_9672(self):
        image = next(i for i in self.after["GPUContainerImages"]
                     if i["downloadURL"] == "mcr.microsoft.com/aks/aks-gpu-grid:*")
        image["gpuVersion"]["latestVersion"] = "595.91.07-20260929170538"
        self.before["GPUContainerImages"] = copy.deepcopy(self.after["GPUContainerImages"])
        next(i for i in self.before["GPUContainerImages"]
             if i["downloadURL"] == image["downloadURL"])["gpuVersion"]["latestVersion"] = (
            "570.237-20260817204535"
        )
        self.assertImpact(False)

    def test_linux_scopes_are_ignored(self):
        for distro in impact.LINUX_DISTROS:
            with self.subTest(distro=distro):
                self.package("containerd")["downloadURIs"][distro] = {
                    "current": {"versionsV2": [{"latestVersion": "linux-only"}]}
                }
                self.assertImpact(False)
        self.package("containerd")["downloadLocation"] = "/new/linux/path"
        self.after["ContainerImages"][0]["amd64OnlyVersions"] = ["linux-only"]
        self.assertImpact(False)

    def test_windows_and_shared_fields_are_retained(self):
        edits = {
            "windows versions": lambda d: d["ContainerImages"][0].update(
                windowsVersions=[{"latestVersion": "changed"}]),
            "multiarch fallback": lambda d: d["ContainerImages"][0].update(
                multiArchVersionsV2=[{"latestVersion": "changed"}]),
            "image download URL": lambda d: d["ContainerImages"][0].update(
                downloadURL="changed"),
            "Windows image URL": lambda d: d["ContainerImages"][0].update(
                windowsDownloadURL="changed"),
            "unknown root field": lambda d: d.update(NewWindowsFeature=True),
            "unknown image field": lambda d: d["ContainerImages"][0].update(
                NewWindowsFeature=True),
            "new package": lambda d: d["Packages"].append(
                {"name": "new", "downloadURIs": {}}),
            "removed package": lambda d: d["Packages"].pop(),
            "package ordering": lambda d: d["Packages"].reverse(),
            "image ordering": lambda d: d["ContainerImages"].reverse(),
            "OCI artifact": lambda d: d.setdefault("OCIArtifacts", []).append(
                {"name": "new", "registry": "new", "windowsVersions": []}),
        }
        for label, edit in edits.items():
            with self.subTest(label=label):
                self.after = copy.deepcopy(self.before)
                edit(self.after)
                self.assertImpact(True)

    def test_package_default_and_windows_inputs_are_retained(self):
        for name in ("containerd", "oras"):
            for scope in ("default", "windows"):
                for field, value in (
                    ("versionsV2", [{"latestVersion": "new", "previousLatestVersion": "old"}]),
                    ("downloadURL", "new"),
                    ("windowsDownloadURL", "new"),
                    ("windowsDownloadRequiresAzCopy", True),
                    ("unknown", True),
                ):
                    with self.subTest(name=name, scope=scope, field=field):
                        self.after = copy.deepcopy(self.before)
                        self.package(name)["downloadURIs"].setdefault(scope, {})[
                            "current" if scope == "default" else "default"
                        ] = {field: value}
                        self.assertImpact(True)

    def test_windows_package_destination_and_name_are_retained(self):
        for field in ("windowsDownloadLocation", "name"):
            with self.subTest(field=field):
                self.after = copy.deepcopy(self.before)
                self.package("oras")[field] = "changed"
                self.assertImpact(True)

    def test_windows_auth_and_version_order_are_not_just_url_set_comparisons(self):
        part = {
            "downloadURL": "https://example.invalid/${version}.zip",
            "versionsV2": [{"latestVersion": "v1"}, {"latestVersion": "v2"}],
        }
        for document in (self.before, self.after):
            next(p for p in document["Packages"] if p["name"] == "containerd")[
                "downloadURIs"
            ]["windows"] = {"default": copy.deepcopy(part)}
        current = self.package("containerd")["downloadURIs"]["windows"]["default"]
        current["windowsDownloadRequiresAzCopy"] = True
        self.assertImpact(True)
        del current["windowsDownloadRequiresAzCopy"]
        current["versionsV2"].reverse()
        self.assertImpact(True)

    def test_windows_fields_removed_or_future_distro_added_require_build(self):
        self.package("oras")["downloadURIs"].pop("windows", None)
        self.assertImpact(True)
        self.after = copy.deepcopy(self.before)
        self.package("oras")["downloadURIs"]["futureos"] = {}
        self.assertImpact(True)

    def test_all_windows_sku_and_oci_fields_are_retained(self):
        for scope in ("default", "ws2019", "ws2022", "ws2025", "wsfuture"):
            with self.subTest(scope=scope):
                self.after = copy.deepcopy(self.before)
                self.package("containerd")["downloadURIs"].setdefault("windows", {})[
                    scope
                ] = {"versionsV2": [{"latestVersion": "new"}]}
                self.assertImpact(True)
        for field, value in (
            ("registry", "changed"), ("windowsDownloadLocation", "changed"),
            ("windowsVersions", [{"latestVersion": "new", "previousLatestVersion": "old",
                                  "windowsSkuMatch": "2025*"}]),
        ):
            with self.subTest(field=field):
                self.after = copy.deepcopy(self.before)
                self.after["OCIArtifacts"][0][field] = value
                self.assertImpact(True)

    def test_whole_document_azcopy_flag_under_linux_fields_is_retained(self):
        # compute_msi_resource_strings scans every object, including Linux subtrees.
        ubuntu = self.package("containerd")["downloadURIs"].setdefault("ubuntu", {})
        for value, expected in ((True, True), (False, False), (1, False)):
            with self.subTest(value=value):
                ubuntu["windowsDownloadRequiresAzCopy"] = value
                self.assertImpact(expected)
        self.before, self.after = self.after, self.before
        ubuntu["windowsDownloadRequiresAzCopy"] = True
        self.assertImpact(True)

    def test_json_formatting_is_ignored_but_types_are_not(self):
        self.assertEqual(impact.windows_inputs(json.dumps(self.before)),
                         impact.windows_inputs(json.dumps(self.before, indent=4, sort_keys=True)))
        self.before["unknown"] = True
        self.after["unknown"] = 1
        self.assertImpact(True)

    def test_bad_json_fails_closed(self):
        for text in (
            "{", "[]", "null", '{"ContainerImages":[],"Packages":[],"unknown":NaN}',
            '{"ContainerImages":[],"Packages":[],"Packages":[]}',
            '{"ContainerImages":[],"Packages":[],"packages":[]}',
            '{"ContainerImages":null,"Packages":[]}',
            '{"ContainerImages":[],"Packages":[{"downloadURIs":null}]}',
            '{"ContainerImages":[],"Packages":[],"GPUContainerImages":null}',
        ):
            with self.subTest(text=text), self.assertRaises(ValueError):
                impact.windows_inputs(text)


class GitImpactTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.env = dict(os.environ, GIT_AUTHOR_NAME="Test", GIT_AUTHOR_EMAIL="test@example.com",
                        GIT_COMMITTER_NAME="Test", GIT_COMMITTER_EMAIL="test@example.com")
        self.git("init", "-q")
        self.git("config", "commit.gpgsign", "false")
        self.file = self.repo / impact.COMPONENTS_PATH
        self.file.parent.mkdir(parents=True)
        self.file.write_text(json.dumps(COMPONENTS))
        self.commit()
        self.base = self.git("rev-parse", "HEAD").strip()

    def git(self, *args):
        return subprocess.run(["git", "-C", str(self.repo), *args], check=True,
                              capture_output=True, text=True, env=self.env).stdout

    def commit(self):
        self.git("add", ".")
        self.git("commit", "-qm", "test")

    def linux_change(self):
        data = json.loads(self.file.read_text())
        data["GPUContainerImages"][0]["gpuVersion"]["latestVersion"] = "linux-only"
        self.file.write_text(json.dumps(data))
        self.commit()

    def merge(self, target=None):
        head = self.git("rev-parse", "HEAD").strip()
        tree = self.git("rev-parse", "HEAD^{tree}").strip()
        merge = self.git("commit-tree", tree, "-p", target or self.base,
                         "-p", head, "-m", "PR merge").strip()
        self.git("checkout", "-q", "--detach", merge)
        return merge

    def check(self):
        return impact.skip_windows_vhd(self.repo, "PullRequest", "refs/pull/9672/merge",
                                      self.git("rev-parse", "HEAD").strip())[0]

    def test_components_only_pr_skips(self):
        self.linux_change()
        self.merge()
        self.assertTrue(self.check())

    def test_windows_change_in_earlier_commit_is_not_missed(self):
        (self.repo / "windows.ps1").write_text("changed")
        self.commit()
        self.linux_change()
        self.merge()
        self.assertFalse(self.check())

    def test_linux_and_windows_components_in_same_pr_run_build(self):
        self.linux_change()
        data = json.loads(self.file.read_text())
        data["ContainerImages"][0]["windowsVersions"] = [{"latestVersion": "new"}]
        self.file.write_text(json.dumps(data))
        self.commit()
        self.merge()
        self.assertFalse(self.check())

    def test_any_other_path_including_rename_runs_build(self):
        for path in ("parts/windows/script.ps1", "schemas/components.cue",
                     ".pipelines/scripts/helper.py", "README.md"):
            with self.subTest(path=path):
                self.git("checkout", "-q", "--detach", self.base)
                changed = self.repo / path
                changed.parent.mkdir(parents=True, exist_ok=True)
                changed.write_text("changed")
                self.linux_change()
                self.merge()
                self.assertFalse(self.check())
        self.git("mv", impact.COMPONENTS_PATH, "renamed.json")
        self.commit()
        self.merge()
        self.assertFalse(self.check())

    def test_component_file_mode_changes_run_build(self):
        self.linux_change()
        self.git("update-index", "--chmod=+x", impact.COMPONENTS_PATH)
        self.git("commit", "-qm", "mode")
        self.merge()
        self.assertFalse(self.check())

    def test_non_pr_and_non_merge_refs_never_skip(self):
        self.linux_change()
        merge = self.merge()
        for reason, branch in (
            ("Manual", "refs/pull/9672/merge"),
            ("IndividualCI", "refs/heads/main"),
            ("PullRequest", "refs/pull/9672/head"),
        ):
            with self.subTest(reason=reason, branch=branch):
                self.assertFalse(impact.skip_windows_vhd(self.repo, reason, branch, merge)[0])

    def test_wrong_checkout_fails_closed(self):
        self.linux_change()
        self.merge()
        with self.assertRaises(ValueError):
            impact.skip_windows_vhd(self.repo, "PullRequest", "refs/pull/9672/merge", self.base)

    def test_non_merge_commit_does_not_skip(self):
        self.linux_change()
        self.assertFalse(self.check())

    def test_deleted_or_symlinked_components_never_skip(self):
        self.file.unlink()
        self.commit()
        self.merge()
        self.assertFalse(self.check())
        self.git("checkout", "-q", "--detach", self.base)
        self.file.unlink()
        self.file.symlink_to("elsewhere.json")
        self.commit()
        self.merge()
        self.assertFalse(self.check())

    def test_cli_falls_back_to_full_build_when_detection_fails(self):
        self.linux_change()
        merge = self.merge()
        env = dict(self.env, BUILD_REASON="PullRequest",
                   BUILD_SOURCEBRANCH="refs/pull/9672/merge", BUILD_SOURCEVERSION=merge)
        result = subprocess.run(
            [sys.executable, str(SCRIPT)], cwd=self.repo, env=env,
            capture_output=True, text=True,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("variable=skipWindowsVhd;isOutput=true]true", result.stdout)
        env["BUILD_SOURCEVERSION"] = self.base
        result = subprocess.run(
            [sys.executable, str(SCRIPT)], cwd=self.repo, env=env,
            capture_output=True, text=True,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("task.logissue type=warning", result.stdout)
        self.assertIn("variable=skipWindowsVhd;isOutput=true]false", result.stdout)
        self.assertNotIn("isOutput=true]true", result.stdout)

    def test_advanced_target_changes_are_not_attributed_to_pr(self):
        (self.repo / "windows.ps1").write_text("target-only")
        self.commit()
        target = self.git("rev-parse", "HEAD").strip()
        self.linux_change()
        self.merge(target)
        self.assertTrue(self.check())

    def test_depth_two_checkout_works_but_missing_parent_fails_closed(self):
        self.linux_change()
        merge = self.merge()
        for depth in (2, 1):
            with self.subTest(depth=depth), tempfile.TemporaryDirectory() as clone:
                subprocess.run(["git", "clone", "-q", "--depth", str(depth),
                                self.repo.as_uri(), clone], check=True, capture_output=True)
                if depth == 2:
                    self.assertTrue(impact.skip_windows_vhd(
                        clone, "PullRequest", "refs/pull/9672/merge", merge)[0])
                else:
                    with self.assertRaises(subprocess.CalledProcessError):
                        impact.skip_windows_vhd(
                            clone, "PullRequest", "refs/pull/9672/merge", merge)


class WindowsConsumerDriftTests(unittest.TestCase):
    def test_windows_scripts_do_not_read_ignored_linux_fields(self):
        # If a Windows script starts reading an ignored field, the projection
        # in windows_inputs() must be updated before this test is.
        ignored = "|".join(
            ("GPUContainerImages", "amd64OnlyVersions", "downloadLocation", *impact.LINUX_DISTROS)
        )
        pattern = re.compile(rf"\.({ignored})\b", re.IGNORECASE)
        scripts = [
            path for directory in ("vhdbuilder/packer/windows", "parts/windows", "staging/cse/windows")
            for path in sorted((ROOT / directory).glob("*.ps1"))
            if not path.name.endswith(".tests.ps1")
        ]
        self.assertTrue(scripts)
        hits = [
            f"{path.relative_to(ROOT)}:{number}: {line.strip()}"
            for path in scripts
            for number, line in enumerate(path.read_text(errors="replace").splitlines(), 1)
            if pattern.search(line)
        ]
        self.assertEqual(hits, [])


if __name__ == "__main__":
    unittest.main()
