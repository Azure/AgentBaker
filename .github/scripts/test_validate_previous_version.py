import importlib.util
import pathlib
import sys
import unittest


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


if __name__ == "__main__":
    unittest.main()
