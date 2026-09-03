import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest import mock

from hotfix import hotfix_generate


TRADITIONAL_TEMPLATE = """- path: {{GetCSEHelpersScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}
{{if IsACL }}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceACL"}}
{{- else if IsAzlOSGuard}}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceAzlOSGuard"}}
{{- else if IsMariner}}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceMariner"}}
{{- else if IsFlatcar }}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceFlatcar"}}
{{- else }}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSourceUbuntu"}}
{{end}}
"""


class HotfixGenerateTests(unittest.TestCase):
    def test_find_block_boundaries(self):
        content = f"""#cloud-config
write_files:
{{{{if EnableScriptlessCSECmd}}}}
{{{{- else }}}}
{TRADITIONAL_TEMPLATE}{{{{- end }}}}
"""

        start, outer_else, end = hotfix_generate.find_block_boundaries(
            content.splitlines(keepends=True)
        )

        self.assertEqual(2, start)
        self.assertEqual(3, outer_else)
        self.assertEqual(len(content.splitlines()) - 1, end)

    def test_parse_write_files_blocks_keeps_distro_chain_together(self):
        blocks = hotfix_generate.parse_write_files_blocks(
            TRADITIONAL_TEMPLATE.splitlines(keepends=True)
        )

        self.assertEqual(2, len(blocks))
        self.assertEqual({"provisionSource"}, blocks[0][0])
        self.assertEqual(
            {
                "provisionSourceUbuntu",
                "provisionSourceMariner",
                "provisionSourceAzlOSGuard",
                "provisionSourceFlatcar",
                "provisionSourceACL",
            },
            blocks[1][0],
        )

    def test_build_hotfix_template_selects_only_requested_blocks(self):
        rendered = hotfix_generate.build_hotfix_template(
            {"provisionSource"},
            TRADITIONAL_TEMPLATE.splitlines(keepends=True),
        )

        self.assertTrue(rendered.startswith("#cloud-config\nwrite_files:\n"))
        self.assertIn("provisionSource", rendered)
        self.assertNotIn("provisionSourceUbuntu", rendered)

    def test_build_hotfix_template_emits_valid_empty_document(self):
        rendered = hotfix_generate.build_hotfix_template(
            set(),
            TRADITIONAL_TEMPLATE.splitlines(keepends=True),
        )

        self.assertEqual("#cloud-config\nwrite_files: []\n", rendered)

    def test_detect_changed_varkeys_expands_distro_group(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            artifacts = Path(temp_dir)
            for source in (
                "ubuntu/cse_helpers_ubuntu.sh",
                "mariner/cse_helpers_mariner.sh",
                "azlosguard/cse_helpers_osguard.sh",
                "flatcar/cse_helpers_flatcar.sh",
                "acl/cse_helpers_acl.sh",
            ):
                path = artifacts / source
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("hotfix")
            changed = artifacts / "ubuntu/cse_helpers_ubuntu.sh"
            result = subprocess.CompletedProcess(
                args=[],
                returncode=0,
                stdout=f"{changed}\n",
            )
            available = {
                "provisionSourceUbuntu",
                "provisionSourceMariner",
                "provisionSourceAzlOSGuard",
                "provisionSourceFlatcar",
                "provisionSourceACL",
            }

            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ), mock.patch.object(
                hotfix_generate.subprocess, "run", return_value=result
            ):
                selected = hotfix_generate.detect_changed_varkeys(
                    "base",
                    available_varkeys=available,
                )

            self.assertEqual(available, selected)

    def test_write_rendered_payload_uses_canonical_renderer(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            generated = Path(temp_dir) / "generated"

            def render(command, check):
                self.assertTrue(check)
                self.assertEqual("go", command[0])
                output_dir = Path(command[command.index("--output-dir") + 1])
                for platform in (
                    "ubuntu",
                    "mariner",
                    "acl",
                    "azlosguard",
                    "flatcar",
                ):
                    (output_dir / f"rendered_nodecustomdata_{platform}.yml").write_text(
                        "#cloud-config\n"
                        "write_files:\n"
                        "- path: /opt/azure/containers/provision_source.sh\n"
                        "  permissions: \"0744\"\n"
                        "  owner: root\n"
                        "  content: hotfix\n"
                    )

            with mock.patch.object(
                hotfix_generate, "GENERATED_DIR", str(generated)
            ), mock.patch.object(
                hotfix_generate.subprocess, "run", side_effect=render
            ):
                hotfix_generate.write_rendered_payload(
                    {"provisionSource"},
                    TRADITIONAL_TEMPLATE.splitlines(keepends=True),
                )

            expected = {
                "ubuntu",
                "mariner",
                "acl",
                "azlosguard",
                "flatcar",
            }
            actual = {
                path.name.removeprefix("rendered_nodecustomdata_").removesuffix(".yml")
                for path in generated.glob("rendered_nodecustomdata_*.yml")
            }
            self.assertEqual(expected, actual)
            for path in generated.glob("rendered_nodecustomdata_*.yml"):
                content = path.read_text()
                self.assertIn("/opt/azure/containers/provision_source.sh", content)
                self.assertNotIn("{{", content)
            self.assertFalse((generated / ".nodecustomdata-hotfix.template").exists())
            self.assertEqual("true\n", (generated / "active").read_text())

    def test_write_rendered_payload_preserves_previous_hotfix_when_unchanged(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            generated = Path(temp_dir) / "generated"
            generated.mkdir()
            (generated / "active").write_text("true\n")
            platforms = ("ubuntu", "mariner", "acl", "azlosguard", "flatcar")
            for platform in platforms:
                (generated / f"rendered_nodecustomdata_{platform}.yml").write_text(
                    f"write_files:\n- path: /{platform}-existing\n"
                )

            with mock.patch.object(
                hotfix_generate, "GENERATED_DIR", str(generated)
            ), mock.patch.object(
                hotfix_generate.subprocess, "run"
            ) as run:
                hotfix_generate.write_rendered_payload(
                    set(),
                    TRADITIONAL_TEMPLATE.splitlines(keepends=True),
                )

            run.assert_not_called()
            self.assertEqual("true\n", (generated / "active").read_text())
            for platform in platforms:
                self.assertIn(
                    f"/{platform}-existing",
                    (
                        generated / f"rendered_nodecustomdata_{platform}.yml"
                    ).read_text(),
                )

    def test_write_hotfix_file_contains_only_anc_version_and_preserves_it(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            target = Path(temp_dir) / "hotfix.json"
            with mock.patch.object(
                hotfix_generate, "TARGET_FILE", str(target)
            ):
                hotfix_generate.write_hotfix_file("202608.14.1")
                self.assertEqual(
                    {"version": "202608.14.1"},
                    json.loads(target.read_text()),
                )
                hotfix_generate.write_hotfix_file("")
                self.assertEqual(
                    {"version": "202608.14.1"},
                    json.loads(target.read_text()),
                )

    def test_write_hotfix_file_without_version_keeps_missing_target_absent(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            target = Path(temp_dir) / "hotfix.json"
            with mock.patch.object(
                hotfix_generate, "TARGET_FILE", str(target)
            ):
                hotfix_generate.write_hotfix_file("")
                self.assertFalse(target.exists())

    def test_unmapped_hotfixable_script_fails(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            changed = Path(temp_dir) / "unmapped.sh"
            changed.write_text("#!/bin/sh")
            result = subprocess.CompletedProcess(
                args=[],
                returncode=0,
                stdout=f"{changed}\n",
            )
            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ), mock.patch.object(
                hotfix_generate.subprocess, "run", return_value=result
            ):
                with self.assertRaisesRegex(
                    hotfix_generate.GenerationError,
                    "has no source/runtime mapping",
                ):
                    hotfix_generate.detect_changed_varkeys("base")

    def test_mapped_source_without_renderable_entry_fails(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            changed = Path(temp_dir) / "mapped.sh"
            changed.write_text("#!/bin/sh")
            result = subprocess.CompletedProcess(
                args=[],
                returncode=0,
                stdout=f"{changed}\n",
            )
            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ), mock.patch.object(
                hotfix_generate,
                "SOURCE_TO_VARKEY",
                {"mapped.sh": "missingVariable"},
            ), mock.patch.object(
                hotfix_generate.subprocess,
                "run",
                return_value=result,
            ):
                with self.assertRaisesRegex(
                    hotfix_generate.GenerationError,
                    "has no traditional nodecustomdata write_files entry",
                ):
                    hotfix_generate.detect_changed_varkeys(
                        "base",
                        available_varkeys={"otherVariable"},
                    )


if __name__ == "__main__":
    unittest.main()
