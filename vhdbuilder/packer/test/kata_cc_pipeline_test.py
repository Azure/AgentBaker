"""Local wiring checks; no Azure calls or VHD build. Requires PyYAML."""
import json
from pathlib import Path
import subprocess
import unittest

import yaml

ROOT = Path(__file__).resolve().parents[3]


class KataCCPipelineTests(unittest.TestCase):
    def test_signed_job_contract(self):
        pipeline = yaml.safe_load((ROOT / '.pipelines/.vsts-vhd-builder-release.yaml').read_text())
        parameters = {p['name']: p for p in pipeline['parameters']}
        self.assertIs(parameters['buildAzureLinuxV3gen2katacc']['default'], False)
        self.assertEqual(parameters['kataCCSigningBuildId']['default'], '')
        jobs = {j['job']: j for j in pipeline['stages'][0]['jobs']}
        cc = jobs['buildAzureLinuxV3gen2katacc']
        self.assertEqual(cc['condition'], "eq('${{ parameters.buildAzureLinuxV3gen2katacc }}', true)")
        self.assertEqual(cc['steps'][-1]['parameters']['artifactName'], 'azurelinuxv3-gen2-kata-cc')
        self.assertIn('variable=OS_VERSION]V3katacc', cc['steps'][0]['bash'])
        self.assertIn('variable=FEATURE_FLAGS]kata,kata-cc', cc['steps'][0]['bash'])
        template = yaml.safe_load((ROOT / '.pipelines/templates/.builder-release-template.yaml').read_text())
        steps = next(s["${{ if eq(parameters.kataCC, true) }}"] for s in template['steps'] if "${{ if eq(parameters.kataCC, true) }}" in s)
        download = next(s for s in steps if s.get('task') == 'DownloadPipelineArtifact@2')['inputs']
        self.assertEqual(download['pipeline'], 318279)
        self.assertEqual(download['artifactName'], 'drop_container_job_sign_uvm_azl3')
        self.assertEqual(download['buildVersionToDownload'], 'specific')
        self.assertIs(download['allowPartiallySucceededBuilds'], False)
        unsigned = next(s for s in template['steps'] if s.get('inputs', {}).get('pipeline') == 417429)
        self.assertIn("eq('${{ parameters.kataCC }}', false)", unsigned['condition'])
        for step in steps + cc['steps']:
            if 'bash' in step:
                subprocess.run(['bash', '-n'], input=step['bash'], text=True, check=True)

    def test_packer_upload_is_cc_only(self):
        command = ['make', '-n', '-f', 'packer.mk', 'build-packer', 'OS_SKU=AzureLinux', 'ARCHITECTURE=X86_64', 'HYPERV_GENERATION=V2']
        cc = subprocess.check_output(command + ['FEATURE_FLAGS=kata,kata-cc'], cwd=ROOT, text=True)
        ordinary = subprocess.check_output(command + ['FEATURE_FLAGS=kata'], cwd=ROOT, text=True)
        self.assertIn('vhd-image-builder-kata-cc.generated.json', cc)
        self.assertNotIn('kata-cc-payload', ordinary)
        # Evaluate the same jq expression emitted by make, without running Packer.
        jq_command = next(line for line in cc.splitlines() if line.startswith('jq ')).split(' > ')[0]
        template = json.loads(subprocess.check_output(jq_command, cwd=ROOT, shell=True, text=True))
        self.assertEqual(template['provisioners'][1], {'type': 'file', 'source': 'kata-cc-payload', 'destination': '/home/packer'})


if __name__ == '__main__':
    unittest.main()
