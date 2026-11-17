import json
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from hotfix import hotfix_generate as generator


class HotfixGenerateTests(unittest.TestCase):
    def test_patch_is_newer_than_image_and_existing_tags(self):
        with mock.patch.object(generator.subprocess, "run", side_effect=[
            subprocess.CompletedProcess([], 0),
            subprocess.CompletedProcess([], 1),
        ]) as run:
            self.assertEqual("202610.30.5", generator.bump_version("202610.30.3"))
            self.assertIn("refs/tags/v0.20261030.4", run.call_args_list[0].args[0])

    def test_git_errors_are_not_absence(self):
        with mock.patch.object(generator.subprocess, "run", return_value=subprocess.CompletedProcess([], 128)):
            with self.assertRaises(subprocess.CalledProcessError):
                generator.bump_version("202610.30.0")
            with self.assertRaises(subprocess.CalledProcessError):
                generator.path_changed("base")

    def test_invalid_version_is_rejected(self):
        with self.assertRaises(ValueError):
            generator.bump_version("dev")

    def test_pointer_updates_whole_bundle_and_preserves_other_bases(self):
        with tempfile.TemporaryDirectory() as directory:
            version, target = Path(directory) / "version.json", Path(directory) / "hotfix.json"
            version.write_text('{"version":"202610.30.0"}')
            target.write_text('{"scripts_version":"202610.30.1","hotfixes":{"202609.01":"202609.01.2"}}')
            with mock.patch.object(generator, "VERSION_FILE", version), mock.patch.object(generator, "TARGET_FILE", target), \
                 mock.patch.object(generator, "path_changed", return_value=True), \
                 mock.patch.object(generator, "bump_version", return_value="202610.30.1"):
                generator.generate("base")
            self.assertEqual({"hotfixes": {"202609.01": "202609.01.2", "202610.30": "202610.30.1"}}, json.loads(target.read_text()))
            before = target.read_bytes()
            with mock.patch.object(generator, "TARGET_FILE", target), mock.patch.object(generator, "path_changed", return_value=False):
                generator.generate("base")
            self.assertEqual(before, target.read_bytes())


if __name__ == "__main__":
    unittest.main()
