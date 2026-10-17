import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
from urllib.error import HTTPError


spec = importlib.util.spec_from_file_location(
    "diagnostic", Path(__file__).parents[1] / "diagnose-acl-capture-boot.py"
)
diagnostic = importlib.util.module_from_spec(spec)
spec.loader.exec_module(diagnostic)


class BootDiagnosticTests(unittest.TestCase):
    def test_identical_security_configuration_for_both_images(self):
        images = [{"sharedGalleryImageId": diagnostic.SOURCE}, {"id": diagnostic.CAPTURE}]
        bodies = [diagnostic.vm_body("vm", image, "/nic", "public-key", {}) for image in images]
        self.assertNotEqual(bodies[0]["properties"]["storageProfile"]["imageReference"],
                            bodies[1]["properties"]["storageProfile"]["imageReference"])
        for body in bodies:
            props = body["properties"]
            self.assertEqual(props["securityProfile"], {
                "securityType": "TrustedLaunch",
                "uefiSettings": {"secureBootEnabled": True, "vTpmEnabled": True},
            })
            self.assertEqual(props["diagnosticsProfile"], {"bootDiagnostics": {"enabled": True}})
            self.assertTrue(props["osProfile"]["linuxConfiguration"]["disablePasswordAuthentication"])
            self.assertNotIn("adminPassword", props["osProfile"])
            self.assertNotIn("identity", body)
            self.assertEqual(props["networkProfile"], {"networkInterfaces": [{"id": "/nic"}]})
            del props["storageProfile"]["imageReference"]
        self.assertEqual(bodies[0], bodies[1])

    def test_wrong_subscription_rejected(self):
        env = {
            "BUILD_BUILDID": "123", "SYSTEM_JOBATTEMPT": "1",
            "BUILD_SOURCEBRANCH": "refs/heads/aadagarwal/diagnose-acl-capture-boot-20261017",
        }
        with patch.dict(diagnostic.os.environ, env, clear=True), \
                patch.object(diagnostic, "az", return_value={"id": "wrong"}):
            with self.assertRaisesRegex(ValueError, "subscription"):
                diagnostic.context()

    def test_other_branch_rejected_before_azure_call(self):
        env = {"BUILD_BUILDID": "123", "SYSTEM_JOBATTEMPT": "1",
               "BUILD_SOURCEBRANCH": "refs/heads/master"}
        with patch.dict(diagnostic.os.environ, env, clear=True), \
                patch.object(diagnostic, "az") as azure:
            with self.assertRaisesRegex(ValueError, "test branch"):
                diagnostic.context()
            azure.assert_not_called()

    def test_cleanup_refuses_mismatched_tags(self):
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(diagnostic, "context", return_value=("owned", {"build": "123"}, Path(tmp))), \
                patch.object(diagnostic, "az", side_effect=[True, {"tags": {"build": "456"}}]) as azure:
            with self.assertRaisesRegex(RuntimeError, "Ownership"):
                diagnostic.cleanup()
            self.assertEqual(azure.call_count, 2)

    def test_cleanup_waits_for_confirmed_deletion(self):
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(diagnostic, "context", return_value=("owned", {"build": "123"}, Path(tmp))), \
                patch.object(diagnostic, "az", side_effect=[
                    True, {"tags": {"build": "123"}}, None, False,
                ]) as azure:
            diagnostic.cleanup()
            self.assertEqual(azure.call_args_list[2].args,
                             ("group", "delete", "--name", "owned", "--yes", "--no-wait"))
            self.assertFalse(json.loads((Path(tmp) / "cleanup.json").read_text())["exists"])

    def test_download_error_does_not_persist_sas(self):
        uri = "https://example.blob.core.windows.net/serial?sig=SECRET"
        with tempfile.TemporaryDirectory() as tmp, \
                patch.object(diagnostic, "rest", side_effect=[
                    {}, {"statuses": []}, {"serialConsoleLogBlobUri": uri},
                ]), \
                patch.object(diagnostic, "urlopen", side_effect=HTTPError(uri, 403, "denied", {}, None)):
            evidence = diagnostic.capture_diagnostics(Path(tmp), "captured", "/vm", "final")
            self.assertIn("instance-view", evidence)
            content = "".join(path.read_text() for path in Path(tmp).iterdir())
            self.assertNotIn("SECRET", content)
            self.assertIn("HTTP 403", content)


if __name__ == "__main__":
    unittest.main()
