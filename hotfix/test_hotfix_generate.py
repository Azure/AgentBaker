import subprocess
import unittest
from unittest import mock

from hotfix import hotfix_generate


class AncProductionChangedTests(unittest.TestCase):
    @staticmethod
    def git_diff_result(*paths):
        return subprocess.CompletedProcess(
            args=[],
            returncode=0,
            stdout="\0".join(paths) + ("\0" if paths else ""),
        )

    @mock.patch.object(hotfix_generate.subprocess, "run")
    def test_production_only_diff_triggers_binary_hotfix(self, run):
        run.return_value = self.git_diff_result(
            "aks-node-controller/parser/parser.go",
            "aks-node-controller/proto/aksnodeconfig/v1/config.proto",
        )

        self.assertTrue(hotfix_generate.anc_production_changed("origin/main"))

    @mock.patch.object(hotfix_generate.subprocess, "run")
    def test_test_only_diff_does_not_trigger_binary_hotfix(self, run):
        run.return_value = self.git_diff_result(
            "aks-node-controller/parser/parser_test.go",
            "aks-node-controller/parser/testdata/test_aksnodeconfig.json",
            "aks-node-controller/parser/testdata/scenario/generatedCSECommand",
        )

        self.assertFalse(hotfix_generate.anc_production_changed("origin/main"))

    @mock.patch.object(hotfix_generate.subprocess, "run")
    def test_mixed_diff_triggers_binary_hotfix(self, run):
        run.return_value = self.git_diff_result(
            "aks-node-controller/parser/parser_test.go",
            "aks-node-controller/parser/testdata/test_aksnodeconfig.json",
            "aks-node-controller/parser/parser.go",
        )

        self.assertTrue(hotfix_generate.anc_production_changed("origin/main"))


if __name__ == "__main__":
    unittest.main()
