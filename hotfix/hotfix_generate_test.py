import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from hotfix import hotfix_generate


COMMON_TEMPLATE = """#cloud-config
write_files:
{{if EnableScriptlessCSECmd}}
- path: /opt/azure/containers/scriptless-cse-overrides.txt
  permissions: "0644"
  owner: root
  content: phase-1
{{- else }}
- path: {{GetCSEHelpersScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}
{{- end }}
"""

COMMON_TRADITIONAL = """- path: {{GetCSEHelpersScriptFilepath}}
  permissions: "0744"
  encoding: gzip
  owner: root
  content: !!binary |
    {{GetVariableProperty "cloudInitData" "provisionSource"}}
"""


DISTRO_TEMPLATE = """{{if IsACL }}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionSourceACL"}}
{{- else if IsAzlOSGuard}}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionSourceAzlOSGuard"}}
{{- else if IsMariner}}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionSourceMariner"}}
{{- else if IsFlatcar }}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionSourceFlatcar"}}
{{- else }}
- path: {{GetCSEHelpersScriptDistroFilepath}}
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionSourceUbuntu"}}
{{end}}
"""


class HotfixGenerateTest(unittest.TestCase):
    def test_source_mapping_validation_rejects_duplicate_variable_keys(self):
        with mock.patch.object(
            hotfix_generate,
            "SOURCE_TO_VARKEY",
            {"one.sh": "shared", "two.sh": "shared"},
        ), mock.patch.object(
            hotfix_generate,
            "VARKEY_TO_SOURCE",
            {"shared": "two.sh"},
        ), mock.patch.object(
            hotfix_generate,
            "VARKEY_TO_BLOCK_GROUP",
            {},
        ):
            with self.assertRaisesRegex(
                hotfix_generate.GenerationError,
                "duplicate variable keys",
            ):
                hotfix_generate.validate_source_mappings()

    def test_template_platforms_are_derived_from_existing_branches(self):
        entries = hotfix_generate.parse_write_file_entries(
            DISTRO_TEMPLATE.splitlines(keepends=True)
        )

        self.assertEqual(
            [
                ("provisionSourceACL", ("acl",)),
                ("provisionSourceAzlOSGuard", ("azlosguard",)),
                ("provisionSourceMariner", ("mariner",)),
                ("provisionSourceFlatcar", ("flatcar",)),
                ("provisionSourceUbuntu", ("ubuntu",)),
            ],
            [(entry["varkey"], entry["platforms"]) for entry in entries],
        )

    def test_older_compound_platform_branches_preserve_template_semantics(self):
        template = """{{if IsAzlOSGuard}}
{{- else if IsMariner}}
- path: /etc/systemd/system/snapshot-update.service
  permissions: "0644"
  content: {{GetVariableProperty "cloudInitData" "packageUpdateServiceMariner"}}
{{- else if not IsFlatcar }}
- path: /etc/systemd/system/snapshot-update.service
  permissions: "0644"
  content: {{GetVariableProperty "cloudInitData" "snapshotUpdateService"}}
{{end}}
{{ if not (or IsMariner IsFlatcar IsACL) -}}
- path: /opt/azure/manifest.json
  permissions: "0644"
  content: {{GetVariableProperty "cloudInitData" "componentManifestFile"}}
{{- end }}
"""
        entries = hotfix_generate.parse_write_file_entries(
            template.splitlines(keepends=True)
        )

        self.assertEqual(
            [
                ("packageUpdateServiceMariner", ("mariner",)),
                ("snapshotUpdateService", ("ubuntu", "acl")),
                ("componentManifestFile", ("ubuntu",)),
            ],
            [(entry["varkey"], entry["platforms"]) for entry in entries],
        )

    def test_runtime_path_rejects_traversal_before_normalizing(self):
        with self.assertRaisesRegex(
            hotfix_generate.GenerationError,
            "unsafe runtime path expression",
        ):
            hotfix_generate.resolve_runtime_path("/opt/../etc/example")

    def test_legacy_custom_cloud_init_mapping_is_supported(self):
        self.assertEqual(
            "initAKSCustomCloud",
            hotfix_generate.SOURCE_TO_VARKEY["init-aks-custom-cloud.sh"],
        )
        self.assertEqual(
            "/opt/azure/containers/init-aks-custom-cloud.sh",
            hotfix_generate.resolve_runtime_path(
                "{{GetInitAKSCustomCloudFilepath}}"
            ),
        )

    def test_build_manifest_fails_when_selected_source_is_missing(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ):
                with self.assertRaisesRegex(
                    hotfix_generate.GenerationError,
                    "does not exist",
                ):
                    hotfix_generate.build_manifest_entries(
                        {"provisionSource"},
                        COMMON_TRADITIONAL.splitlines(keepends=True),
                    )

    def test_build_manifest_fails_without_runtime_template_mapping(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            source = Path(temp_dir) / "cse_send_logs.py"
            source.write_text("print('logs')")
            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ):
                with self.assertRaisesRegex(
                    hotfix_generate.GenerationError,
                    "no matching write_files entry",
                ):
                    hotfix_generate.build_manifest_entries(
                        {"provisionSendLogs"},
                        COMMON_TRADITIONAL.splitlines(keepends=True),
                    )

    def test_changed_hotfixable_script_without_source_mapping_fails(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            changed = f"{temp_dir}/unmapped.sh"
            Path(changed).write_text("#!/bin/sh")
            result = subprocess.CompletedProcess(
                args=[],
                returncode=0,
                stdout=f"{changed}\n",
            )
            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ), mock.patch.object(
                hotfix_generate.subprocess,
                "run",
                return_value=result,
            ):
                with self.assertRaisesRegex(
                    hotfix_generate.GenerationError,
                    "has no source/runtime mapping",
                ):
                    hotfix_generate.detect_changed_varkeys("base")

    def test_build_manifest_rejects_duplicate_destination_for_platform(self):
        duplicate = """- path: /opt/duplicate
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionSource"}}
- path: /opt/duplicate
  permissions: "0744"
  content: {{GetVariableProperty "cloudInitData" "provisionScript"}}
"""
        with tempfile.TemporaryDirectory() as temp_dir:
            artifacts = Path(temp_dir)
            (artifacts / "cse_helpers.sh").write_bytes(b"helpers")
            (artifacts / "cse_main.sh").write_bytes(b"main")
            with mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", temp_dir
            ):
                with self.assertRaisesRegex(
                    hotfix_generate.GenerationError,
                    "duplicate runtime destination",
                ):
                    hotfix_generate.build_manifest_entries(
                        {"provisionSource", "provisionScript"},
                        duplicate.splitlines(keepends=True),
                    )

    def test_default_generation_keeps_crp_and_writes_matching_embedded_entry(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            template = root / "nodecustomdata.yml"
            template.write_text(COMMON_TEMPLATE)
            artifacts = root / "artifacts"
            artifacts.mkdir()
            source = artifacts / "cse_helpers.sh"
            source.write_bytes(b"#!/bin/sh\necho hotfix\n")
            generated = root / "generated"
            written_versions = []

            patches = self._main_patches(
                template=template,
                artifacts=artifacts,
                generated=generated,
                argv=["hotfix_generate.py", "base"],
                anc_changed=True,
                written_versions=written_versions,
            )
            with patches:
                hotfix_generate.main()

            rendered = template.read_text()
            self.assertIn(hotfix_generate.SCRIPTS_BEGIN, rendered)
            self.assertIn("provisionSource", rendered)

            manifest = json.loads((generated / "manifest.json").read_text())
            self.assertEqual(1, len(manifest["entries"]))
            entry = manifest["entries"][0]
            self.assertEqual("cse_helpers.sh", entry["source"])
            self.assertEqual(
                "/opt/azure/containers/provision_source.sh",
                entry["destination"],
            )
            self.assertEqual("0744", entry["mode"])
            self.assertEqual(
                list(hotfix_generate.ALL_PLATFORMS),
                entry["platforms"],
            )
            self.assertEqual(source.read_bytes(), (
                generated / entry["payload"]
            ).read_bytes())
            self.assertEqual(
                [("202607.20.1", "202607.20.1")],
                written_versions,
            )

    def test_embedded_only_is_explicit_and_default_off(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            template = root / "nodecustomdata.yml"
            template.write_text(COMMON_TEMPLATE)
            artifacts = root / "artifacts"
            artifacts.mkdir()
            (artifacts / "cse_helpers.sh").write_bytes(b"hotfix")
            generated = root / "generated"

            patches = self._main_patches(
                template=template,
                artifacts=artifacts,
                generated=generated,
                argv=["hotfix_generate.py", "--embedded-only", "base"],
            )
            with patches:
                hotfix_generate.main()

            self.assertNotIn(
                hotfix_generate.SCRIPTS_BEGIN,
                template.read_text(),
            )
            manifest = json.loads((generated / "manifest.json").read_text())
            self.assertEqual(1, len(manifest["entries"]))

    def test_empty_generation_restores_embeddable_placeholder(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            template = root / "nodecustomdata.yml"
            template.write_text(COMMON_TEMPLATE)
            artifacts = root / "artifacts"
            artifacts.mkdir()
            generated = root / "generated"
            stale = generated / "payloads" / "stale.sh"
            stale.parent.mkdir(parents=True)
            stale.write_text("stale")

            patches = self._main_patches(
                template=template,
                artifacts=artifacts,
                generated=generated,
                argv=["hotfix_generate.py", "base"],
                selected=set(),
            )
            with patches:
                hotfix_generate.main()

            manifest = json.loads((generated / "manifest.json").read_text())
            self.assertEqual(
                {"schema_version": 1, "entries": []},
                manifest,
            )
            self.assertFalse((generated / "payloads").exists())

    def test_existing_active_crp_hotfix_is_preserved_without_new_changes(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            legacy_block = f"""{hotfix_generate.LEGACY_SCRIPTS_BEGIN}
{COMMON_TRADITIONAL}{hotfix_generate.LEGACY_SCRIPTS_END}
"""
            template = root / "nodecustomdata.yml"
            template.write_text(COMMON_TEMPLATE.replace(
                "{{- else }}",
                legacy_block + "{{- else }}",
                1,
            ))
            artifacts = root / "artifacts"
            artifacts.mkdir()
            source = artifacts / "cse_helpers.sh"
            source.write_bytes(b"still-active")
            generated = root / "generated"

            patches = self._main_patches(
                template=template,
                artifacts=artifacts,
                generated=generated,
                argv=["hotfix_generate.py", "base"],
                selected=set(),
            )
            with patches:
                hotfix_generate.main()

            rendered = template.read_text()
            self.assertIn(hotfix_generate.SCRIPTS_BEGIN, rendered)
            self.assertNotIn(hotfix_generate.LEGACY_SCRIPTS_BEGIN, rendered)
            manifest = json.loads((generated / "manifest.json").read_text())
            self.assertEqual(
                ["cse_helpers.sh"],
                [entry["source"] for entry in manifest["entries"]],
            )

    def test_distro_hotfix_generation_is_idempotent_across_two_runs(self):
        template_content = f"""#cloud-config
write_files:
{{{{if EnableScriptlessCSECmd}}}}
- path: /opt/override
  permissions: "0644"
  content: hotfix
{{{{- else }}}}
{DISTRO_TEMPLATE}{{{{- end }}}}
"""
        with tempfile.TemporaryDirectory() as temp_dir:
            template = Path(temp_dir) / "nodecustomdata.yml"
            template.write_text(template_content)
            with mock.patch.object(
                hotfix_generate, "TEMPLATE", str(template)
            ):
                target = {
                    "provisionSourceUbuntu",
                    "provisionSourceMariner",
                    "provisionSourceAzlOSGuard",
                    "provisionSourceFlatcar",
                    "provisionSourceACL",
                }
                self.assertTrue(hotfix_generate.inject_scripts(target))
                self.assertTrue(hotfix_generate.inject_scripts(target))

            rendered = template.read_text()
            self.assertEqual(1, rendered.count(hotfix_generate.SCRIPTS_BEGIN))
            lines = rendered.splitlines(keepends=True)
            start, outer_else, end = hotfix_generate.find_block_boundaries(lines)
            self.assertIsNotNone(start)
            self.assertIsNotNone(outer_else)
            self.assertIsNotNone(end)

    def test_embedded_only_removes_legacy_crp_block(self):
        content = (
            hotfix_generate.LEGACY_SCRIPTS_BEGIN
            + "\n"
            + COMMON_TRADITIONAL
            + hotfix_generate.LEGACY_SCRIPTS_END
            + "\n"
        )
        self.assertEqual(
            "",
            hotfix_generate.remove_generated_scripts_blocks(content),
        )

    def _main_patches(
        self,
        *,
        template,
        artifacts,
        generated,
        argv,
        selected=None,
        anc_changed=False,
        written_versions=None,
    ):
        if selected is None:
            selected = {"provisionSource"}
        if written_versions is None:
            written_versions = []
        return _PatchStack(
            mock.patch.object(hotfix_generate, "TEMPLATE", str(template)),
            mock.patch.object(
                hotfix_generate, "ARTIFACTS_DIR", str(artifacts)
            ),
            mock.patch.object(
                hotfix_generate, "GENERATED_DIR", str(generated)
            ),
            mock.patch.object(
                hotfix_generate,
                "GENERATED_MANIFEST",
                str(generated / "manifest.json"),
            ),
            mock.patch.object(
                hotfix_generate,
                "GENERATED_PAYLOADS_DIR",
                str(generated / "payloads"),
            ),
            mock.patch.object(
                hotfix_generate,
                "detect_changed_varkeys",
                return_value=selected,
            ),
            mock.patch.object(
                hotfix_generate, "read_base_version", return_value="202607.20.0"
            ),
            mock.patch.object(
                hotfix_generate,
                "path_changed",
                side_effect=lambda _base, *paths: (
                    paths == (str(template),)
                    or (anc_changed and paths[0] == hotfix_generate.ANC_DIR)
                ),
            ),
            mock.patch.object(
                hotfix_generate,
                "bump_version",
                return_value="202607.20.1",
            ),
            mock.patch.object(
                hotfix_generate,
                "write_hotfix_file",
                side_effect=lambda version, scripts_version: (
                    written_versions.append((version, scripts_version))
                ),
            ),
            mock.patch.object(hotfix_generate.subprocess, "run"),
            mock.patch.object(sys, "argv", argv),
        )


class _PatchStack:
    def __init__(self, *patches):
        self.patches = patches

    def __enter__(self):
        for patch in self.patches:
            patch.start()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        for patch in reversed(self.patches):
            patch.stop()
        return False


if __name__ == "__main__":
    unittest.main()
