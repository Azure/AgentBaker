import contextlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import urllib.error

import acl_release_notes as acl


def notes(tag="3.0.20261016", source=None):
    text = (
        '=== os-release Begin\nID=azurelinux\nVARIANT_ID=azurecontainerlinux\n'
        f'NAME="Microsoft Azure Container Linux"\nVERSION_ID="{tag}"\n'
        'BUILD_ID=1226753\n=== os-release End\n'
    )
    if source is not None:
        text += (
            f"=== {acl.PROVENANCE_SECTION} Begin\nACL_RELEASE_TAG={tag}\n"
            f"ACL_RELEASE_URL={acl.release_url(tag)}\nACL_ACG_SOURCE={source}\n"
            f"=== {acl.PROVENANCE_SECTION} End\n"
        )
    return text


def release(tag, **fields):
    return {"tag_name": tag, "draft": False, "prerelease": False, **fields}


class ProvenanceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.image_notes = self.root / "release-notes.txt"
        self.image_notes.write_text(notes(), encoding="utf-8")
        self.env = mock.patch.dict(os.environ, {
            "SIG_SOURCE_GALLERY_UNIQUE_NAME": "acl-gallery",
            "SIG_SOURCE_IMAGE_NAME": "acl-3.0-amd64",
            "SIG_SOURCE_IMAGE_VERSION": "20261016.1226753.0",
        })
        self.env.start()
        self.addCleanup(self.env.stop)
    @mock.patch("acl_release_notes.urllib.request.urlopen")
    def test_records_actual_guest_version_not_gallery_date_and_is_idempotent(self, urlopen):
        self.image_notes.write_text(notes("3.0.20261014"), encoding="utf-8")
        acl.enrich_image(self.image_notes)
        first = self.image_notes.read_text(encoding="utf-8")
        self.assertIn("ACL_RELEASE_TAG=3.0.20261014", first)
        self.assertIn(f"ACL_RELEASE_URL={acl.release_url('3.0.20261014')}", first)
        self.assertIn("ACL_RELEASE_URL_STATUS=unverified", first)
        self.assertIn("ACL_ACG_SOURCE=acl-gallery/acl-3.0-amd64/20261016.1226753.0", first)
        acl.enrich_image(self.image_notes)
        self.assertEqual(first, self.image_notes.read_text(encoding="utf-8"))
        urlopen.assert_not_called()

    def test_arm64_keeps_its_exact_source(self):
        os.environ["SIG_SOURCE_IMAGE_NAME"] = "acl-3.0-arm64"
        os.environ["SIG_SOURCE_IMAGE_VERSION"] = "20261016.1226751.0"
        acl.enrich_image(self.image_notes)
        self.assertEqual(
            acl.read_image(self.image_notes.read_text(encoding="utf-8")).base_image,
            "acl-gallery/acl-3.0-arm64/20261016.1226751.0",
        )

    def test_dev_builds_do_not_get_fabricated_release_links(self):
        for tag in ("3.0.20261016-dev", "3.0.20261340"):
            with self.subTest(tag=tag):
                self.image_notes.write_text(notes(tag), encoding="utf-8")
                with contextlib.redirect_stderr(io.StringIO()) as stderr:
                    acl.enrich_image(self.image_notes)
                self.assertIn("WARNING:", stderr.getvalue())
                result = self.image_notes.read_text(encoding="utf-8")
                self.assertIn("ACL_RELEASE_URL=\n", result)
                self.assertNotIn("/releases/tag/", result)

    def test_missing_or_unpinned_source_fails_without_modifying_notes(self):
        for source in ("", "latest"):
            with self.subTest(source=source):
                original = self.image_notes.read_text(encoding="utf-8")
                os.environ["SIG_SOURCE_IMAGE_VERSION"] = source
                with self.assertRaisesRegex(ValueError, "pinned ACG"):
                    acl.enrich_image(self.image_notes)
                self.assertEqual(original, self.image_notes.read_text(encoding="utf-8"))

    @mock.patch("acl_release_notes.urllib.request.urlopen")
    def test_existing_source_mismatch_fails_before_querying_github(self, urlopen):
        self.image_notes.write_text(
            notes(source="another-gallery/acl/20261016.1.0"), encoding="utf-8"
        )
        with self.assertRaisesRegex(ValueError, "pinned ACG source"):
            acl.enrich_image(self.image_notes)
        urlopen.assert_not_called()

    def test_rejects_non_acl_and_ambiguous_or_inconsistent_notes(self):
        cases = [
            notes().replace("azurecontainerlinux", "ubuntu"),
            notes() + notes(),
            notes().replace("VERSION_ID=", "VERSION_ID=3.0.20261013\nVERSION_ID="),
            notes() + f"=== {acl.PROVENANCE_SECTION} Begin\n",
            notes() + "=== os-release End\n",
            notes(source="gallery/acl/20261016.1.0").replace(
                "ACL_RELEASE_TAG=3.0.20261016", "ACL_RELEASE_TAG=3.0.20261013"
            ),
            notes(source="gallery/acl/latest"),
        ]
        for text in cases:
            with self.subTest(text=text), self.assertRaises(ValueError):
                acl.read_image(text)

    def test_does_not_execute_os_release_substitutions(self):
        self.assertEqual(
            acl.read_image(notes().replace(
                'NAME="Microsoft Azure Container Linux"', 'NAME="$(not-a-command)"'
            )).tag,
            "3.0.20261016",
        )


class GitHubTests(unittest.TestCase):
    @mock.patch("acl_release_notes.urllib.request.urlopen")
    def test_caches_release_lookups_and_sets_timeout(self, urlopen):
        urlopen.return_value = io.BytesIO(json.dumps(release("3.0.20261016")).encode())
        github = acl.GitHub()
        self.assertTrue(github.published("3.0.20261016"))
        self.assertTrue(github.published("3.0.20261016"))
        self.assertEqual(urlopen.call_count, 1)
        self.assertEqual(urlopen.call_args.kwargs["timeout"], 30)

    @mock.patch("acl_release_notes.urllib.request.urlopen")
    def test_only_404_is_treated_as_unpublished(self, urlopen):
        for status in (404, 403, 429, 500):
            with self.subTest(status=status):
                urlopen.side_effect = urllib.error.HTTPError(acl.API, status, "failure", {}, None)
                if status == 404:
                    self.assertFalse(acl.GitHub().published("3.0.20261016"))
                else:
                    with self.assertRaises(urllib.error.HTTPError):
                        acl.GitHub().published("3.0.20261016")

    def test_rejects_draft_prerelease_and_malformed_release_responses(self):
        for fields in ({"draft": True}, {"prerelease": True}):
            github = acl.GitHub()
            github.responses["releases/tags/3.0.20261016"] = release("3.0.20261016", **fields)
            self.assertFalse(github.published("3.0.20261016"))
        for data in ([], {}, {"tag_name": "3.0.20261016"}, release("3.0.20261014")):
            github = acl.GitHub()
            github.responses["releases/tags/3.0.20261016"] = data
            with self.subTest(data=data), self.assertRaises(ValueError):
                github.published("3.0.20261016")

    def test_paginated_intermediate_releases_are_filtered_by_version_and_ancestry(self):
        github = acl.GitHub()
        github.responses.update({
            "releases?per_page=100&page=1": [release("3.0.20261016")] * 100,
            "releases?per_page=100&page=2": [
                release("3.0.20261014"), release("3.0.20261013"),
                release("3.0.20261012"), release("3.0.20261007"),
                release("3.0.20261020"), release("4.0.20261014"),
                release("3.0.20261015", draft=True),
                release("3.0.20261015", prerelease=True),
                release("3.0.20261015-dev"),
            ],
            "compare/3.0.20261012...3.0.20261016": {"status": "diverged"},
            "compare/3.0.20261013...3.0.20261016": {"status": "ahead"},
            "compare/3.0.20261014...3.0.20261016": {"status": "identical"},
        })
        self.assertEqual(
            github.intervening("3.0.20261007", "3.0.20261016"),
            ["3.0.20261013", "3.0.20261014", "3.0.20261016"],
        )

    def test_invalid_api_shapes_fail_explicitly(self):
        github = acl.GitHub()
        github.responses["compare/3.0.20261013...3.0.20261016"] = None
        with self.assertRaisesRegex(ValueError, "comparison"):
            github.compare("3.0.20261013", "3.0.20261016")
        github.responses["releases?per_page=100&page=1"] = {"message": "rate limited"}
        with self.assertRaisesRegex(ValueError, "release list"):
            github.intervening("3.0.20261013", "3.0.20261016")
        github.responses["releases?per_page=100&page=1"] = [{"tag_name": "3.0.20261014"}]
        with self.assertRaisesRegex(ValueError, "release list"):
            github.intervening("3.0.20261013", "3.0.20261016")


class WeeklyTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.previous = self.root / "previous"
        self.current = self.root / "current"
        self.previous.mkdir()
        self.current.mkdir()
        self.github = acl.GitHub()
        for tag in ("3.0.20261007", "3.0.20261013", "3.0.20261014", "3.0.20261016"):
            self.github.responses[f"releases/tags/{tag}"] = release(tag)
        self.github.responses.update({
            "releases?per_page=100&page=1": [
                release("3.0.20261016"), release("3.0.20261014"),
                release("3.0.20261013"), release("3.0.20261007"),
            ],
            "compare/3.0.20261007...3.0.20261016": {"status": "ahead"},
            "compare/3.0.20261013...3.0.20261016": {"status": "ahead"},
            "compare/3.0.20261014...3.0.20261016": {"status": "ahead"},
        })

    def snapshot(self, root, sku, tag, source=None):
        path = root / sku / "latest.txt"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(notes(tag, source), encoding="utf-8")
        return path

    def test_multiple_bumps_use_published_per_sku_baselines(self):
        self.snapshot(self.previous, "gen2tl", "3.0.20261007")
        self.snapshot(self.previous, "arm64gen2tl", "3.0.20261013")
        self.snapshot(self.current, "gen2tl", "3.0.20261016", "gallery/amd64/20261016.1226753.0")
        self.snapshot(self.current, "arm64gen2tl", "3.0.20261016", "gallery/arm64/20261016.1226751.0")
        # A superseded build in the same directory is not the published latest image.
        (self.current / "gen2tl" / "superseded.txt").write_text(notes("3.0.20261014"), encoding="utf-8")
        result = acl.weekly_rollup(self.previous, self.current, self.github)
        amd64 = result.split("### gen2tl")[1]
        arm64 = result.split("### arm64gen2tl")[1].split("### gen2tl")[0]
        self.assertIn("/compare/3.0.20261007...3.0.20261016", amd64)
        self.assertIn("/compare/3.0.20261013...3.0.20261016", arm64)
        self.assertIn("gallery/amd64/20261016.1226753.0", amd64)
        self.assertIn("gallery/arm64/20261016.1226751.0", arm64)
        for tag in ("3.0.20261013", "3.0.20261014", "3.0.20261016"):
            self.assertIn(f"- [{tag}]", amd64)
        self.assertNotIn("- [3.0.20261013]", arm64)
        self.assertIn("- [3.0.20261014]", arm64)

    def test_unchanged_tag_can_still_have_a_different_acg_build(self):
        self.snapshot(self.previous, "gen2tl", "3.0.20261016", "gallery/amd64/20261016.1.0")
        self.snapshot(self.current, "gen2tl", "3.0.20261016", "gallery/amd64/20261016.2.0")
        result = acl.weekly_rollup(self.previous, self.current, self.github)
        self.assertIn("Upstream release unchanged", result)
        self.assertIn("ACG source build changed", result)
        self.assertNotIn("/compare/", result)

    def test_new_sku_does_not_invent_a_baseline(self):
        self.snapshot(self.current, "cvmgen2", "3.0.20261016")
        result = acl.weekly_rollup(self.previous, self.current, self.github)
        self.assertIn("No previous published-image baseline", result)
        self.assertIn("not recorded in historical notes", result)
        self.assertNotIn("/compare/", result)

    def test_rollback_is_not_reported_as_forward_changes(self):
        self.snapshot(self.previous, "gen2tl", "3.0.20261016")
        self.snapshot(self.current, "gen2tl", "3.0.20261013")
        self.github.responses["compare/3.0.20261016...3.0.20261013"] = {"status": "behind"}
        result = acl.weekly_rollup(self.previous, self.current, self.github)
        self.assertIn("rollback", result)
        self.assertNotIn("Included upstream", result)

    def test_diverged_or_unpublished_releases_fail(self):
        self.snapshot(self.previous, "gen2tl", "3.0.20261013")
        self.snapshot(self.current, "gen2tl", "3.0.20261016")
        self.github.responses["compare/3.0.20261013...3.0.20261016"] = {"status": "diverged"}
        with self.assertRaisesRegex(ValueError, "diverged"):
            acl.weekly_rollup(self.previous, self.current, self.github)
        self.github.responses["releases/tags/3.0.20261016"] = None
        with self.assertRaisesRegex(ValueError, "unpublished"):
            acl.weekly_rollup(self.previous, self.current, self.github)

    def test_missing_empty_and_corrupt_snapshots_fail(self):
        with self.assertRaisesRegex(ValueError, "must exist"):
            acl.weekly_rollup(self.root / "missing", self.current, self.github)
        with self.assertRaisesRegex(ValueError, "no per-SKU"):
            acl.weekly_rollup(self.previous, self.current, self.github)
        path = self.snapshot(self.current, "gen2tl", "3.0.20261016")
        path.write_text("corrupt notes", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "os-release"):
            acl.weekly_rollup(self.previous, self.current, self.github)

    def test_cli_failure_does_not_replace_existing_output(self):
        self.snapshot(self.current, "gen2tl", "3.0.20261016")
        output = self.root / "weekly.md"
        output.write_text("existing output", encoding="utf-8")
        args = [
            "acl_release_notes.py", "weekly",
            "--previous-notes", str(self.previous),
            "--current-notes", str(self.current), "--output", str(output),
        ]
        with mock.patch("sys.argv", args), mock.patch.object(acl, "GitHub", return_value=self.github):
            self.github.responses["releases/tags/3.0.20261016"] = None
            with contextlib.redirect_stderr(io.StringIO()) as stderr:
                with self.assertRaises(SystemExit) as error:
                    acl.main()
        self.assertEqual(error.exception.code, 1)
        self.assertIn("ERROR:", stderr.getvalue())
        self.assertEqual(output.read_text(encoding="utf-8"), "existing output")


class IntegrationTests(unittest.TestCase):
    def test_shared_pipeline_enriches_only_acl_notes_before_export(self):
        root = Path(__file__).resolve().parents[2]
        template = root / ".pipelines" / "templates" / ".builder-release-template.yaml"
        text = template.read_text(encoding="utf-8")
        step = text.split("python3 vhdbuilder/packer/acl_release_notes.py image release-notes.txt", 1)[1]
        self.assertIn("eq(variables.OS_SKU, 'AzureContainerLinux')", step.split("- task:", 1)[0])
        self.assertLess(
            text.index("displayName: Enrich ACL Base Image Release Notes"),
            text.index("displayName: Publish Release Notes"),
        )


if __name__ == "__main__":
    unittest.main()
