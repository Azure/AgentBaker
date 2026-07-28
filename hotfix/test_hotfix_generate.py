import importlib.util
import json
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock


MODULE_PATH = Path(__file__).with_name("hotfix_generate.py")
SPEC = importlib.util.spec_from_file_location("hotfix_generate", MODULE_PATH)
hotfix_generate = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(hotfix_generate)


class PointerOnlyVersionTests(unittest.TestCase):
    def test_accepts_higher_patch_in_same_version_stream(self):
        self.assertEqual(
            "202607.20.2",
            hotfix_generate.validate_pointer_only_version(
                "202607.20.0", "202607.20.2"
            ),
        )

    def test_rejects_different_version_stream(self):
        with self.assertRaisesRegex(ValueError, "must use base '202607.20'"):
            hotfix_generate.validate_pointer_only_version(
                "202607.20.0", "202607.27.1"
            )

    def test_rejects_non_increasing_patch(self):
        with self.assertRaisesRegex(ValueError, "must have a higher patch"):
            hotfix_generate.validate_pointer_only_version(
                "202607.20.1", "202607.20.1"
            )

    def test_rejects_invalid_version(self):
        with self.assertRaisesRegex(ValueError, "expected YYYYMM.DD.PATCH"):
            hotfix_generate.validate_pointer_only_version(
                "202607.20.0", "v202607.20.2"
            )

    def test_reads_version_only_when_target_differs_from_base(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            target = Path(temp_dir) / "aks-node-controller-hotfix.json"
            target.write_text(
                json.dumps(
                    {
                        "version": "202607.20.2",
                        "scripts_version": "untrusted-generator-owned-value",
                    }
                )
            )
            with (
                mock.patch.object(hotfix_generate, "TARGET_FILE", str(target)),
                mock.patch.object(
                    hotfix_generate, "target_file_changed", return_value=True
                ),
            ):
                self.assertEqual(
                    "202607.20.2",
                    hotfix_generate.read_pointer_only_version(
                        "origin/official/v20260720", "202607.20.0"
                    ),
                )

            with (
                mock.patch.object(hotfix_generate, "TARGET_FILE", str(target)),
                mock.patch.object(
                    hotfix_generate, "target_file_changed", return_value=False
                ),
            ):
                self.assertEqual(
                    "",
                    hotfix_generate.read_pointer_only_version(
                        "origin/official/v20260720", "202607.20.0"
                    ),
                )

    def test_detects_new_untracked_target_when_absent_from_base(self):
        with (
            mock.patch.object(hotfix_generate, "path_changed", return_value=False),
            mock.patch.object(hotfix_generate.os.path, "exists", return_value=True),
            mock.patch.object(
                hotfix_generate.subprocess,
                "run",
                return_value=mock.Mock(returncode=1),
            ) as run,
        ):
            self.assertTrue(hotfix_generate.target_file_changed("base-ref"))

        run.assert_called_once_with(
            [
                "git",
                "cat-file",
                "-e",
                f"base-ref:{hotfix_generate.TARGET_FILE}",
            ],
            capture_output=True,
        )

    def test_anc_source_change_takes_precedence_over_pointer(self):
        with (
            mock.patch.object(sys, "argv", ["hotfix_generate.py", "base-ref"]),
            mock.patch.object(hotfix_generate.subprocess, "run"),
            mock.patch.object(hotfix_generate, "detect_changed_varkeys", return_value=set()),
            mock.patch.object(hotfix_generate, "remove_scripts_block"),
            mock.patch.object(
                hotfix_generate, "read_base_version", return_value="202607.20.0"
            ),
            mock.patch.object(
                hotfix_generate,
                "path_changed",
                side_effect=lambda _base_ref, path: path == hotfix_generate.ANC_DIR,
            ),
            mock.patch.object(
                hotfix_generate, "bump_version", return_value="202607.20.1"
            ),
            mock.patch.object(hotfix_generate, "read_pointer_only_version") as read_pointer,
            mock.patch.object(hotfix_generate, "write_hotfix_file") as write_hotfix,
        ):
            hotfix_generate.main()

        read_pointer.assert_not_called()
        write_hotfix.assert_called_once_with("202607.20.1", "")


if __name__ == "__main__":
    unittest.main()
