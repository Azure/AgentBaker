package scenario

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Azure/agentbaker/e2e/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v7"
	"github.com/stretchr/testify/require"
)

func TestACLIPEVMSSNoProfileTag(t *testing.T) {
	var acl *Scenario
	for _, s := range List() {
		if s.Name == "ACL" {
			acl = s
			break
		}
	}
	require.NotNil(t, acl)
	for _, mode := range []string{"", "off", "audit"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(aclIPEModeEnv, mode)
			vmss := &armcompute.VirtualMachineScaleSet{
				Tags: map[string]*string{"owner": to.Ptr("scenario")},
			}
			acl.VMConfigMutator(vmss)
			require.Equal(t, map[string]*string{"owner": to.Ptr("scenario")}, vmss.Tags)
			require.NoError(t, validateACLIPEVMSSNoProfileTag(vmss.Tags))
		})
	}
	require.NoError(t, validateACLIPEVMSSNoProfileTag(nil))
	for _, name := range []string{aclIPESecurityProfileTag, "ACL-Node-Security-Profile"} {
		require.ErrorContains(t, validateACLIPEVMSSNoProfileTag(map[string]*string{name: nil}), "unexpected ACL IPE security profile tag")
	}
}

func TestACLIPEExpectedModeOptIn(t *testing.T) {
	t.Setenv(aclIPEModeEnv, "")
	acl := &Scenario{Name: "ACL"}
	require.False(t, aclIPEValidationRequested(acl))
	mode, err := aclIPEExpectedMode(acl)
	require.NoError(t, err)
	require.Empty(t, mode)
	require.NoError(t, ValidateACLIPEFirstBoot(context.Background(), acl))
	require.Empty(t, acl.adoTestCases)

	for _, expected := range []string{"off", "audit"} {
		t.Run(expected, func(t *testing.T) {
			t.Setenv(aclIPEModeEnv, expected)
			mode, err := aclIPEExpectedMode(acl)
			require.NoError(t, err)
			require.Equal(t, expected, mode)
			require.True(t, aclIPEValidationRequested(acl))
			require.False(t, aclIPEValidationRequested(&Scenario{Name: "ACL_CustomCA"}))
		})
	}

	t.Run("invalid mode fails before image lookup", func(t *testing.T) {
		t.Setenv(aclIPEModeEnv, "enforce")
		err := maybeSkipScenario(context.Background(), "ACL", acl)
		require.ErrorContains(t, err, "ACL_IPE_EXPECTED_MODE must be off or audit")
	})
}

func TestACLIPEFirstBootMissingIdentityFailsMeasurement(t *testing.T) {
	t.Setenv(aclIPEModeEnv, "audit")
	acl := &Scenario{Name: "ACL"}
	err := ValidateACLIPEFirstBoot(context.Background(), acl)
	require.ErrorContains(t, err, "Azure VM ID")
	require.Len(t, acl.adoTestCases, 1)
	require.Equal(t, "ACL_IPE_FirstBoot_audit", acl.adoTestCases[0].Name)
	require.Contains(t, acl.adoTestCases[0].Message, "Azure VM ID")
}

func TestACLIPEFirstBootRequiresUntaggedVMSS(t *testing.T) {
	t.Setenv(aclIPEModeEnv, "audit")
	newScenario := func(vmssTags, vmTags map[string]*string) *Scenario {
		return &Scenario{
			Name: "ACL",
			Runtime: &ScenarioRuntime{
				VM: &ScenarioVM{
					VMSS: &armcompute.VirtualMachineScaleSet{Tags: vmssTags},
					VM: &armcompute.VirtualMachineScaleSetVM{
						Tags:       vmTags,
						Properties: &armcompute.VirtualMachineScaleSetVMProperties{VMID: to.Ptr("e7b560b4-621d-4831-8224-fb759f961de3")},
					},
				},
			},
		}
	}
	for _, tc := range []struct {
		name string
		vmss map[string]*string
		vm   map[string]*string
		want string
	}{
		{"missing VMSS tag evidence", nil, nil, "requires the created scenario VMSS tags"},
		{"tagged VMSS", map[string]*string{aclIPESecurityProfileTag: to.Ptr("ipe=audit")}, nil, "created scenario VMSS tags: unexpected ACL IPE security profile tag"},
		{"tagged VM instance", map[string]*string{"owner": to.Ptr("scenario")}, map[string]*string{aclIPESecurityProfileTag: to.Ptr("ipe=audit")}, "scenario VM instance tags: unexpected ACL IPE security profile tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newScenario(tc.vmss, tc.vm)
			require.ErrorContains(t, ValidateACLIPEFirstBoot(context.Background(), s), tc.want)
			require.Len(t, s.adoTestCases, 1)
			require.Equal(t, "ACL_IPE_FirstBoot_audit", s.adoTestCases[0].Name)
		})
	}
}

func TestACLIPEPolicyState(t *testing.T) {
	const policy = "policy_name=acl_ipe_boot_policy policy_version=0.0.1\n\nDEFAULT action=ALLOW\n\nDEFAULT op=EXECUTE action=DENY\nop=EXECUTE boot_verified=TRUE action=ALLOW\nop=EXECUTE dmverity_signature=TRUE action=ALLOW\n"
	output := func(enforce, active, body string) string {
		return "enforce=" + enforce + "\nactive=" + active + "\npolicy_base64=" + base64.StdEncoding.EncodeToString([]byte(body)) + "\n"
	}
	for _, tc := range []struct {
		name, output, expected, wantErr string
	}{
		{"off loaded inactive", output("1", "0", policy), "off", ""},
		{"audit active permissive", output("0", "1", policy), "audit", ""},
		{"off with active policy", output("0", "1", policy), "off", "loaded but inactive"},
		{"audit with inactive policy", output("0", "0", policy), "audit", "expected active PR52"},
		{"audit enforcing", output("1", "1", policy), "audit", "expected active PR52"},
		{"off without policy", "enforce=1\n", "off", "missing or invalid PR52"},
		{"audit with unrelated policy", output("0", "1", strings.Replace(policy, "acl_ipe_boot_policy", "unrelated", 1)), "audit", "does not match PR52"},
		{"off with wrong version", output("1", "0", strings.Replace(policy, "0.0.1", "0.0.2", 1)), "off", "does not match PR52"},
		{"audit with extra rule", output("0", "1", policy+"DEFAULT op=OPEN action=ALLOW\n"), "audit", "does not match PR52"},
		{"off with missing deny", output("1", "0", strings.Replace(policy, "DEFAULT op=EXECUTE action=DENY\n", "", 1)), "off", "does not match PR52"},
		{"missing enforcement", "active=1\npolicy_base64=" + base64.StdEncoding.EncodeToString([]byte(policy)) + "\n", "audit", "missing or invalid IPE enforce"},
		{"missing policy", "enforce=0\nactive=1\n", "audit", "missing or invalid PR52"},
		{"duplicate flag", output("0", "1", policy) + "active=0\n", "audit", "duplicate IPE active"},
		{"invalid base64", "enforce=0\nactive=1\npolicy_base64=!\n", "audit", "decode IPE policy"},
		{"unknown field", output("1", "0", policy) + "unknown=value\n", "off", "unexpected IPE policy state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := parseACLIPEPolicyState(tc.output)
			if err == nil {
				err = validateACLIPEPolicyState(state, tc.expected)
			}
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func aclIPETestBash(t *testing.T) string {
	t.Helper()
	bash := "bash"
	if _, err := exec.LookPath(bash); err != nil && runtime.GOOS == "windows" {
		git, gitErr := exec.LookPath("git")
		require.NoError(t, gitErr)
		bash = filepath.Join(filepath.Dir(filepath.Dir(git)), "bin", "bash.exe")
	}
	_, err := exec.LookPath(bash)
	require.NoError(t, err)
	return bash
}

func TestACLIPEPolicyStateShellProbe(t *testing.T) {
	bash := aclIPETestBash(t)
	const policy = "policy_name=acl_ipe_boot_policy policy_version=0.0.1\nDEFAULT action=ALLOW\nDEFAULT op=EXECUTE action=DENY\nop=EXECUTE boot_verified=TRUE action=ALLOW\nop=EXECUTE dmverity_signature=TRUE action=ALLOW\n"
	run := func(t *testing.T, setup func(string), prefix ...string) (string, error) {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(root, "enforce"), []byte("1\n"), 0600))
		if setup != nil {
			setup(root)
		}
		cmd := exec.Command(bash, "-c", strings.Join(prefix, "")+aclIPEPolicyStateProbe, "--", filepath.ToSlash(root))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	createPolicy := func(t *testing.T, root string) {
		t.Helper()
		dir := filepath.Join(root, "policies", aclIPEPolicyName)
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "active"), []byte("0\n"), 0600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "policy"), []byte(policy), 0600))
	}
	t.Run("loaded inactive PR52 policy", func(t *testing.T) {
		out, err := run(t, func(root string) { createPolicy(t, root) })
		require.NoError(t, err, out)
		state, err := parseACLIPEPolicyState(out)
		require.NoError(t, err)
		require.NoError(t, validateACLIPEPolicyState(state, "off"))
	})
	t.Run("missing policies directory", func(t *testing.T) {
		out, err := run(t, nil)
		require.Error(t, err)
		require.Contains(t, out, "missing or unreadable IPE policies directory")
	})
	t.Run("unreadable policies directory", func(t *testing.T) {
		prefix := ""
		if runtime.GOOS == "windows" {
			prefix = "stat() { printf '000\\n'; }\n"
		}
		out, err := run(t, func(root string) {
			createPolicy(t, root)
			dir := filepath.Join(root, "policies")
			t.Cleanup(func() { require.NoError(t, os.Chmod(dir, 0700)) })
			require.NoError(t, os.Chmod(dir, 0000))
		}, prefix)
		require.Error(t, err)
		require.Contains(t, out, "unreadable IPE policies directory")
	})
	t.Run("failed command", func(t *testing.T) {
		out, err := run(t, func(root string) { createPolicy(t, root) },
			"cat() { return 1; }\n")
		require.Error(t, err)
		require.Contains(t, out, "cannot read IPE enforce")
	})
	t.Run("missing policy", func(t *testing.T) {
		out, err := run(t, func(root string) {
			require.NoError(t, os.Mkdir(filepath.Join(root, "policies"), 0700))
		})
		require.Error(t, err)
		require.Contains(t, out, "expected exactly one loaded IPE policy")
	})
	t.Run("failed policy read", func(t *testing.T) {
		out, err := run(t, func(root string) {
			createPolicy(t, root)
			require.NoError(t, os.Remove(filepath.Join(root, "policies", aclIPEPolicyName, "policy")))
		})
		require.Error(t, err)
		require.Contains(t, out, "PR52 IPE policy missing")
	})
	require.Contains(t, aclIPEPrivilegedProbe(aclIPEPolicyStateProbe), "sudo -n bash -s")
}

func TestACLIPEBootEvidenceShellProbe(t *testing.T) {
	bash := aclIPETestBash(t)
	const bootID = "e7b560b4-621d-4831-8224-fb759f961de3"
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, "boot_id"),
		filepath.Join(root, "cmdline"),
		filepath.Join(root, "cache"),
		filepath.Join(root, "failed"),
	}
	require.NoError(t, os.WriteFile(paths[0], []byte(bootID), 0600))
	require.NoError(t, os.WriteFile(paths[1], []byte("flatcar.first_boot=detected acl.ipe.policy_sha256="+strings.Repeat("a", 64)), 0600))
	require.NoError(t, os.WriteFile(paths[2], []byte{}, 0600))
	run := func(journalCommand string) (string, error) {
		args := []string{"-c", "journalctl() { " + journalCommand + "; }\n" + aclIPEBootEvidenceProbe, "--"}
		for _, path := range paths {
			args = append(args, filepath.ToSlash(path))
		}
		cmd := exec.Command(bash, args...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	const journal = `printf 'acl-ipe-load: Credential SHA-256 verified: test\n'`
	out, err := run(journal)
	require.NoError(t, err, out)
	evidence, err := parseACLIPEBootEvidence(out)
	require.NoError(t, err)
	require.Equal(t, bootID, evidence.bootID)
	require.Empty(t, evidence.cache)
	require.Contains(t, evidence.journal, "Credential SHA-256 verified")
	require.NoError(t, os.WriteFile(paths[3], []byte{}, 0600))
	out, err = run(journal)
	require.Error(t, err)
	require.Contains(t, out, "initrd IMDS lookup failed")
	require.NoError(t, os.Remove(paths[3]))
	require.NoError(t, os.Remove(paths[2]))
	out, err = run(journal)
	require.Error(t, err)
	require.Contains(t, out, "missing successful initrd IMDS profile cache")
	require.NoError(t, os.WriteFile(paths[2], []byte{}, 0600))
	out, err = run("return 1")
	require.Error(t, err)
	require.Contains(t, out, "cannot read current-boot IPE loader journal")
	require.Contains(t, aclIPEPrivilegedProbe(aclIPEBootEvidenceProbe), "sudo -n bash -s")
}

func TestACLIPEIMDSTags(t *testing.T) {
	for _, tc := range []struct {
		name, tags, wantErr string
	}{
		{"no tag", `[]`, ""},
		{"unrelated tag", `[{"name":"owner","value":"scenario"}]`, ""},
		{"audit tag is forbidden", `[{"name":"acl-node-security-profile","value":"ipe=audit"}]`, "unexpected ACL IPE security profile tag"},
		{"disabled tag is forbidden", `[{"name":"acl-node-security-profile","value":"ipe=disabled"}]`, "unexpected ACL IPE security profile tag"},
		{"case variant is forbidden", `[{"name":"ACL-Node-Security-Profile","value":""}]`, "unexpected ACL IPE security profile tag"},
		{"malformed JSON", `[`, "invalid IMDS"},
		{"null document", `null`, "invalid IMDS"},
		{"malformed tag", `[{"value":"ipe=audit"}]`, "malformed IMDS"},
		{"missing value", `[{"name":"owner"}]`, "malformed IMDS"},
		{"duplicate", `[{"name":"acl-node-security-profile","value":"ipe=audit"},{"name":"acl-node-security-profile","value":"ipe=off"}]`, "unexpected ACL IPE security profile tag"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateACLIPEIMDSTags(tc.tags)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestACLIPEBootEvidence(t *testing.T) {
	const hash = "ae0c485653602f88e5f0ae35d1b79bf72a324e6fa91f80a3fc95ab184c590d06"
	const bootID = "e7b560b4-621d-4831-8224-fb759f961de3"
	evidence := aclIPEBootEvidence{
		bootID:  bootID,
		cmdline: "flatcar.first_boot=detected acl.ipe.policy_sha256=" + hash,
		journal: "acl-ipe-load: Credential SHA-256 verified: " + hash + "\n" +
			"acl-ipe-load: Loaded policy acl_ipe_boot_policy into kernel IPE.\n" +
			"acl-ipe-load: Using IPE mode 'off'.\n" +
			"acl-ipe-load: IPE mode is off; policy loaded but activation skipped.",
	}
	encode := func(e aclIPEBootEvidence) string {
		return "boot_id=" + e.bootID + "\ncmdline=" + e.cmdline + "\ncache=" + e.cache +
			"\njournal_base64=" + base64.StdEncoding.EncodeToString([]byte(e.journal)) + "\n"
	}
	parsed, err := parseACLIPEBootEvidence(encode(evidence))
	require.NoError(t, err)
	require.NoError(t, validateACLIPEBootEvidence(parsed, "off"))
	audit := evidence
	audit.journal = strings.ReplaceAll(audit.journal, "Using IPE mode 'off'.", "Using IPE mode 'audit'.")
	audit.journal = strings.ReplaceAll(audit.journal, "IPE mode is off; policy loaded but activation skipped.", "Activated policy acl_ipe_boot_policy.")
	require.NoError(t, validateACLIPEBootEvidence(audit, "audit"))
	require.ErrorContains(t, validateACLIPEBootEvidence(evidence, "audit"), "Using IPE mode 'audit'")
	auditMissingPolicy := audit
	auditMissingPolicy.journal = strings.ReplaceAll(audit.journal, "Loaded policy", "Failed policy")
	require.ErrorContains(t, validateACLIPEBootEvidence(auditMissingPolicy, "audit"), "Loaded policy")
	auditTaggedCache := audit
	auditTaggedCache.cache = "ipe=audit"
	require.ErrorContains(t, validateACLIPEBootEvidence(auditTaggedCache, "audit"), "empty initrd IMDS profile cache")
	for _, tc := range []struct {
		name string
		edit func(*aclIPEBootEvidence)
		want string
	}{
		{"wrong boot id", func(e *aclIPEBootEvidence) { e.bootID = "invalid" }, "invalid current boot ID"},
		{"missing hash token", func(e *aclIPEBootEvidence) { e.cmdline = "flatcar.first_boot=detected" }, "UKI-bound"},
		{"duplicate hash token", func(e *aclIPEBootEvidence) { e.cmdline += " acl.ipe.policy_sha256=" + hash }, "UKI-bound"},
		{"missing first boot marker", func(e *aclIPEBootEvidence) { e.cmdline = "acl.ipe.policy_sha256=" + hash }, "first-boot"},
		{"credential hash mismatch", func(e *aclIPEBootEvidence) { e.journal = strings.Replace(e.journal, hash, strings.Repeat("a", 64), 1) }, "Credential SHA-256 verified"},
		{"no policy load", func(e *aclIPEBootEvidence) {
			e.journal = strings.ReplaceAll(e.journal, "Loaded policy", "Failed policy")
		}, "Loaded policy"},
		{"wrong mode", func(e *aclIPEBootEvidence) { e.journal = strings.ReplaceAll(e.journal, "mode 'off'", "mode 'audit'") }, "Using IPE mode"},
		{"cache mismatch", func(e *aclIPEBootEvidence) { e.cache = "ipe=off" }, "cache"},
		{"empty journal", func(e *aclIPEBootEvidence) { e.journal = "" }, "current-boot loader journal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := evidence
			tc.edit(&got)
			require.ErrorContains(t, validateACLIPEBootEvidence(got, "off"), tc.want)
		})
	}
	_, err = parseACLIPEBootEvidence("boot_id=" + bootID + "\n")
	require.ErrorContains(t, err, "missing")
	_, err = parseACLIPEBootEvidence(encode(evidence) + "cache=unexpected\n")
	require.ErrorContains(t, err, "duplicate")
	_, err = parseACLIPEBootEvidence(strings.Replace(encode(evidence), "journal_base64="+base64.StdEncoding.EncodeToString([]byte(evidence.journal)), "journal_base64=!", 1))
	require.ErrorContains(t, err, "decode")
}

func TestACLIPEIMDSField(t *testing.T) {
	const id = "E7B560B4-621D-4831-8224-FB759F961DE3"
	require.NoError(t, validateACLIPEIMDSField("vmId", "e7b560b4-621d-4831-8224-fb759f961de3\n", id))
	require.NoError(t, validateACLIPEIMDSField("vmScaleSetName", "abe2e-acl\n", "abe2e-acl"))
	require.ErrorContains(t, validateACLIPEIMDSField("vmId", "not-a-uuid", id), "invalid IMDS vmId")
	require.ErrorContains(t, validateACLIPEIMDSField("vmId", id, "not-a-uuid"), "invalid Azure VM ID")
	require.ErrorContains(t, validateACLIPEIMDSField("vmId", id, "00000000-0000-0000-0000-000000000001"), "does not match")
	require.ErrorContains(t, validateACLIPEIMDSField("vmScaleSetName", "", "abe2e-acl"), "does not match")
	require.ErrorContains(t, validateACLIPEIMDSField("vmScaleSetName", "system-pool", "abe2e-acl"), "does not match")
	require.ErrorContains(t, validateACLIPEIMDSField("other", "x", "x"), "unknown IMDS field")
}

func TestACLIPEOptInDoesNotSkipMissingVHD(t *testing.T) {
	t.Setenv(aclIPEModeEnv, "")
	original := config.Config.IgnoreScenariosWithMissingVHD
	config.Config.IgnoreScenariosWithMissingVHD = true
	t.Cleanup(func() { config.Config.IgnoreScenariosWithMissingVHD = original })

	missing := errors.Join(config.ErrNotFound, errors.New("not replicated to test region"))
	require.True(t, shouldSkipMissingVHD(&Scenario{Name: "ACL"}, missing))
	for _, mode := range []string{"off", "audit"} {
		t.Setenv(aclIPEModeEnv, mode)
		actual, err := ACLIPEExpectedMode()
		require.NoError(t, err)
		require.Equal(t, mode, actual)
		require.False(t, shouldSkipMissingVHD(&Scenario{Name: "ACL"}, missing))
		require.True(t, shouldSkipMissingVHD(&Scenario{Name: "ACL_CustomCA"}, missing))
	}
	t.Setenv(aclIPEModeEnv, "enforce")
	_, err := ACLIPEExpectedMode()
	require.ErrorContains(t, err, "must be off or audit")
	require.False(t, shouldSkipMissingVHD(&Scenario{Name: "ACL"}, errors.New("other error")))
}
