#!/usr/bin/env python3
"""Differential checks against the distro updater; all writes use temporary roots."""

import argparse
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tempfile
import unittest


class Fixture:
    def __init__(self, root, certificates):
        self.root = root
        self.system = root / "system"
        self.local = root / "local"
        self.store = root / "store"
        self.hooks = root / "hooks"
        self.logs = root / "logs"
        for directory in (self.system / "vendor", self.local / "nested", self.store / "nested",
                          self.hooks, self.logs):
            directory.mkdir(parents=True, exist_ok=True)
        (self.system / "vendor/first.crt").write_bytes(certificates[0])
        (self.system / "vendor/disabled.crt").write_bytes(certificates[1])
        (self.local / "nested/Local (CA, one).crt").write_bytes(certificates[2].rstrip(b"\n"))
        (self.local / "empty.crt").write_bytes(b"")
        (self.local / "ignored.pem").write_bytes(certificates[0])
        (self.local / os.fsdecode(b"Raw-\xff.crt")).write_bytes(certificates[1])
        (root / "foreign.pem").write_bytes(certificates[1])
        (self.store / "foreign.pem").symlink_to(root / "foreign.pem")
        (self.store / "disabled.pem").symlink_to(self.system / "vendor/disabled.crt")
        (self.store / "orphan.pem").symlink_to(self.local / "deleted.crt")
        (self.store / "nested/orphan").symlink_to("missing")
        self.config = root / "ca-certificates.conf"
        self.config.write_text(
            "# fixture\nvendor/first.crt\n!vendor/disabled.crt\nvendor/missing.crt\n",
            encoding="ascii",
        )
        self.hook("10-record")
        self.hook("99-ignore.disabled", "exit 61\n")

    def hook(self, name, body=None):
        if body is None:
            body = (
                f'printf "%s\\n" "$*" > "$FIXTURE_LOG/{name}.args"\n'
                f'cat > "$FIXTURE_LOG/{name}.input"\n'
                f'printf "%s\\n" "{name}" >> "$FIXTURE_LOG/order"\n'
            )
        path = self.hooks / name
        path.write_text("#!/bin/sh\n" + body, encoding="ascii")
        path.chmod(0o755)

    def arguments(self):
        return [
            "--certsconf", str(self.config),
            "--certsdir", str(self.system),
            "--localcertsdir", str(self.local),
            "--etccertsdir", str(self.store),
            "--hooksdir", str(self.hooks),
        ]

    def run(self, executable, *flags, environment=None):
        env = dict(os.environ, LC_ALL="C", FIXTURE_LOG=str(self.logs))
        if environment:
            env.update(environment)
        return subprocess.run(
            [str(executable), *self.arguments(), *flags],
            env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30, check=False,
        )

    def normalize(self, value):
        return value.replace(os.fsencode(self.root), b"<root>")

    def snapshot(self):
        result = {}
        for directory, directories, files in os.walk(self.store):
            for name in directories + files:
                path = Path(directory) / name
                relative = os.fsencode(path.relative_to(self.store))
                if path.is_symlink():
                    result[relative] = ("link", self.normalize(os.fsencode(os.readlink(path))))
                elif path.is_file():
                    result[relative] = (
                        "file", path.read_bytes(), stat.S_IMODE(path.stat().st_mode),
                    )
        return result

    def hook_output(self):
        return {path.name: self.normalize(path.read_bytes()) for path in self.logs.iterdir()}


class CompatibilityTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.certificate_directory = tempfile.TemporaryDirectory(prefix="rust-ca-certificates-")
        root = Path(cls.certificate_directory.name)
        cls.certificates = []
        for number in range(3):
            certificate = root / f"{number}.crt"
            subprocess.run(
                ["openssl", "req", "-x509", "-newkey", "ec",
                 "-pkeyopt", "ec_paramgen_curve:prime256v1", "-nodes", "-days", "1",
                 "-subj", f"/CN=Rust updater fixture {number}",
                 "-keyout", str(root / f"{number}.key"), "-out", str(certificate)],
                check=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE,
            )
            cls.certificates.append(certificate.read_bytes())

    @classmethod
    def tearDownClass(cls):
        cls.certificate_directory.cleanup()

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="rust-ca-comparison-")
        self.addCleanup(self.directory.cleanup)
        root = Path(self.directory.name)
        self.reference = Fixture(root / "reference", self.certificates)
        self.rust = Fixture(root / "rust", self.certificates)

    def compare(self, *flags):
        old = self.reference.run(REFERENCE, *flags)
        new = self.rust.run(BINARY, *flags)
        self.assertEqual(old.returncode, 0, old.stderr.decode(errors="replace"))
        self.assertEqual(new.returncode, 0, new.stderr.decode(errors="replace"))
        self.assertEqual(self.reference.snapshot(), self.rust.snapshot())
        self.assertEqual(self.reference.hook_output(), self.rust.hook_output())
        self.assertEqual(
            re.findall(rb"\d+ added, \d+ removed; done\.", old.stdout),
            re.findall(rb"\d+ added, \d+ removed; done\.", new.stdout),
        )
        return old, new

    def test_selection_mapping_bundle_hashes_and_hooks(self):
        self.compare()
        self.assertEqual(
            (self.rust.store / "ca-certificates.crt").read_bytes(),
            self.certificates[0] + self.certificates[1] + self.certificates[2],
        )
        self.assertTrue((self.rust.store / "Local_=CA__one=.pem").is_symlink())
        self.assertFalse((self.rust.store / "disabled.pem").exists())
        self.assertFalse((self.rust.store / "nested/orphan").is_symlink())
        self.assertNotIn("99-ignore.disabled.args", self.rust.hook_output())

    def test_second_run_skips_rehash_but_still_runs_hooks(self):
        self.compare()
        for fixture in (self.reference, self.rust):
            tools = fixture.root / "tools"
            tools.mkdir()
            openssl = tools / "openssl"
            openssl.write_text("#!/bin/sh\nexit 73\n", encoding="ascii")
            openssl.chmod(0o755)
            (fixture.store / "late-orphan.pem").symlink_to("still-missing")
            result = fixture.run(
                REFERENCE if fixture is self.reference else BINARY,
                environment={"PATH": str(tools) + os.pathsep + os.environ["PATH"]},
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn(b"0 added, 0 removed", result.stdout)
            self.assertTrue((fixture.store / "late-orphan.pem").is_symlink())
            self.assertEqual((fixture.logs / "10-record.input").read_bytes(), b"")
        self.assertEqual(self.reference.snapshot(), self.rust.snapshot())

    def test_fresh_and_directory_symlinks(self):
        for fixture in (self.reference, self.rust):
            (fixture.store / "directory-link").symlink_to(fixture.root, target_is_directory=True)
            (fixture.store / "old.pem").symlink_to(fixture.system / "vendor/first.crt")
        self.compare("--fresh")
        self.assertFalse((self.rust.store / "directory-link").is_symlink())
        self.assertEqual((self.rust.logs / "10-record.args").read_bytes(), b"--fresh\n")

    def test_default_preserves_add_then_remove_hook_order(self):
        self.compare("--default")
        changes = (self.rust.logs / "10-record.input").read_bytes().splitlines()
        first_removal = next(index for index, value in enumerate(changes) if value.startswith(b"-"))
        self.assertTrue(all(value.startswith(b"+") for value in changes[:first_removal]))
        self.assertTrue(all(value.startswith(b"-") for value in changes[first_removal:]))
        self.assertEqual((self.rust.logs / "10-record.args").read_bytes(), b"--fresh\n")

    def test_certificate_content_changes_rebuild_bundle_without_rehash(self):
        self.compare()
        for fixture in (self.reference, self.rust):
            (fixture.system / "vendor/first.crt").write_bytes(self.certificates[2])
        self.compare()
        self.assertTrue(
            (self.rust.store / "ca-certificates.crt").read_bytes().startswith(self.certificates[2])
        )
        self.assertEqual((self.rust.logs / "10-record.input").read_bytes(), b"")

    def test_removal_of_previously_enabled_certificate(self):
        self.compare()
        for fixture in (self.reference, self.rust):
            fixture.config.write_text("!vendor/first.crt\n", encoding="ascii")
        self.compare()
        self.assertFalse((self.rust.store / "first.pem").is_symlink())

    def test_symlinked_source_files_and_directories(self):
        for fixture in (self.reference, self.rust):
            external = fixture.root / "external"
            external.mkdir()
            (external / "linked.crt").write_bytes(self.certificates[0])
            (fixture.local / "alias.crt").symlink_to(fixture.system / "vendor/disabled.crt")
            (fixture.local / "external").symlink_to(external, target_is_directory=True)
        self.compare()

    def test_sorting_matches_c_locale_even_for_directory_prefixes(self):
        for fixture in (self.reference, self.rust):
            (fixture.local / "group").mkdir()
            (fixture.local / "group.crt").write_bytes(self.certificates[0])
            (fixture.local / "group/inside.crt").write_bytes(self.certificates[1])
        self.compare()

    def test_equivalent_but_textually_different_symlink_is_replaced(self):
        for fixture in (self.reference, self.rust):
            (fixture.store / "first.pem").symlink_to(
                str(fixture.system) + "/vendor//first.crt",
            )
        self.compare()
        self.assertEqual(
            os.readlink(self.rust.store / "first.pem"),
            str(self.rust.system / "vendor/first.crt"),
        )

    def test_missing_and_empty_configuration_trigger_fresh(self):
        for missing in (False, True):
            with self.subTest(missing=missing):
                for fixture in (self.reference, self.rust):
                    if missing:
                        fixture.config.unlink()
                    else:
                        fixture.config.write_bytes(b"")
                self.compare()
                self.assertEqual((self.rust.logs / "10-record.args").read_bytes(), b"--fresh\n")

    def test_no_selected_certificates_preserves_previous_bundle(self):
        for fixture in (self.reference, self.rust):
            fixture.config.write_text("# none selected\n", encoding="ascii")
            shutil.rmtree(fixture.local)
            (fixture.store / "ca-certificates.crt").write_bytes(b"previous bundle\n")
        self.compare()
        self.assertEqual(
            (self.rust.store / "ca-certificates.crt").read_bytes(), b"previous bundle\n",
        )

    def test_custom_bundle_and_verbose(self):
        self.compare("--verbose", "--certbundle", "custom-bundle.crt")
        self.assertTrue((self.rust.store / "custom-bundle.crt").is_file())
        self.assertFalse((self.rust.store / "ca-certificates.crt").exists())

    def test_failing_hook_is_reported_and_later_hook_runs(self):
        for fixture in (self.reference, self.rust):
            fixture.hook("20-fail", 'cat > "$FIXTURE_LOG/failed.input"\nexit 7\n')
            fixture.hook("30-record")
        old, new = self.compare()
        self.assertIn(b"exited with code 7.", old.stdout)
        self.assertIn(b"exited with code 7.", new.stdout)
        self.assertTrue((self.rust.logs / "30-record.args").exists())

    def test_executable_hook_without_shebang_uses_posix_shell(self):
        for fixture in (self.reference, self.rust):
            fixture.hook("20-no-shebang")
            path = fixture.hooks / "20-no-shebang"
            path.write_bytes(path.read_bytes().removeprefix(b"#!/bin/sh\n"))
        self.compare()
        self.assertTrue((self.rust.logs / "20-no-shebang.input").exists())

    def test_disappearing_hook_is_reported_and_later_hook_runs(self):
        for fixture in (self.reference, self.rust):
            fixture.hook("05-delete", 'rm "$FIXTURE_LOG/../hooks/20-victim"\ncat > /dev/null\n')
            fixture.hook("20-victim")
            fixture.hook("30-record")
        old, new = self.compare()
        self.assertIn(b"20-victim", old.stdout)
        self.assertIn(b"could not execute hook", new.stderr)
        self.assertTrue((self.rust.logs / "30-record.input").exists())

    def test_large_store_and_hook_that_does_not_read_stdin(self):
        for fixture in (self.reference, self.rust):
            for number in range(700):
                (fixture.local / f"certificate-{number:04d}.crt").write_bytes(self.certificates[0])
            fixture.hook("05-no-read", "exit 0\n")
        self.compare()

    def test_rehash_failure_preserves_bundle_and_does_not_run_hooks(self):
        (self.rust.store / "ca-certificates.crt").write_bytes(b"old\n")
        tools = self.rust.root / "tools"
        tools.mkdir()
        openssl = tools / "openssl"
        openssl.write_text("#!/bin/sh\nexit 23\n", encoding="ascii")
        openssl.chmod(0o755)
        result = self.rust.run(
            BINARY, environment={"PATH": str(tools) + os.pathsep + os.environ["PATH"]},
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"openssl rehash failed", result.stderr)
        self.assertEqual((self.rust.store / "ca-certificates.crt").read_bytes(), b"old\n")
        self.assertEqual(list(self.rust.logs.iterdir()), [])
        self.assertEqual(list(self.rust.store.glob(".update-ca-certificates-rs-*")), [])

    def test_source_directory_loop_fails_instead_of_hanging(self):
        (self.rust.local / "loop").symlink_to(self.rust.local, target_is_directory=True)
        result = self.rust.run(BINARY)
        self.assertNotEqual(result.returncode, 0)
        self.assertIn(b"Directory symlink loop", result.stderr)
        self.assertFalse((self.rust.store / "ca-certificates.crt").exists())

    def test_invalid_cli_and_help_do_not_touch_store(self):
        before = self.rust.snapshot()
        for flag in ("--unknown", "--certsconf"):
            result = self.rust.run(BINARY, flag)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn(b"update-ca-certificates-rs:", result.stderr)
        result = self.rust.run(BINARY, "--help")
        self.assertEqual(result.returncode, 0)
        self.assertIn(b"--sysroot", result.stdout)
        self.assertEqual(before, self.rust.snapshot())

    def test_sysroot_is_confined_to_fixture(self):
        root = self.rust.root / "sysroot"
        for directory in ("etc/ssl/certs", "etc/ca-certificates/update.d",
                          "usr/share/ca-certificates", "usr/local/share/ca-certificates"):
            (root / directory).mkdir(parents=True)
        (root / "etc/ca-certificates.conf").write_bytes(b"# local only\n")
        (root / "usr/local/share/ca-certificates/local.crt").write_bytes(self.certificates[0])
        result = subprocess.run(
            [str(BINARY), "--sysroot", str(root)],
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30, check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(
            (root / "etc/ssl/certs/ca-certificates.crt").read_bytes(), self.certificates[0],
        )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=lambda value: Path(value).resolve())
    parser.add_argument("--reference", required=True, type=lambda value: Path(value).resolve())
    arguments, unittest_arguments = parser.parse_known_args()
    BINARY = arguments.binary
    REFERENCE = arguments.reference
    if not BINARY.is_file() or not REFERENCE.is_file():
        parser.error("Both the Rust executable and reference updater must exist")
    unittest.main(argv=[__file__, *unittest_arguments])
