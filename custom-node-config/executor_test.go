package customnodeconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeHost struct {
	values                             map[string]string
	scripts, restarts, writes          int
	failScript, failRestart, failWrite bool
}

func (h *fakeHost) RunScript(context.Context, string) error {
	h.scripts++
	if h.failScript {
		return errors.New("script failed")
	}
	return nil
}
func (*fakeHost) ValidateContainerd(context.Context, string) error { return nil }
func (h *fakeHost) RestartContainerd(context.Context) error {
	h.restarts++
	if h.failRestart {
		return errors.New("restart failed")
	}
	return nil
}
func (h *fakeHost) ReadSysctl(_ context.Context, name string) (string, error) {
	return h.values[name], nil
}
func (h *fakeHost) WriteSysctl(_ context.Context, name, value string) error {
	h.writes++
	if h.failWrite {
		h.failWrite = false
		return errors.New("write failed")
	}
	h.values[name] = value
	return nil
}

func profile(t *testing.T, s Spec) Profile {
	t.Helper()
	p, err := NewProfile(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
func put(t *testing.T, root, path, contents string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}
func read(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBootCustomerWinsAndRetryDoesNotReplayScript(t *testing.T) {
	root := t.TempDir()
	put(t, root, "etc/default/kubeletconfig.json", `{"kind":"KubeletConfiguration","apiVersion":"kubelet.config.k8s.io/v1beta1","serializeImagePulls":true,"maxParallelImagePulls":1}`)
	put(t, root, "etc/default/kubelet", "KUBELET_FLAGS=--serialize-image-pulls=true --cloud-provider=external\n")
	put(t, root, "etc/containerd/config.toml", "version = 2\n[plugins.'io.containerd.grpc.v1.cri']\nimage_pull_with_sync_fs = false\nsandbox_image = 'managed'\n")
	h := &fakeHost{values: map[string]string{"net.ipv4.tcp_retries2": "8"}}
	e := Executor{Root: root, Host: h}
	p := profile(t, Spec{BootScript: "#!/bin/bash\ntrue\n", Kubelet: map[string]json.RawMessage{"serializeImagePulls": json.RawMessage("false"), "maxParallelImagePulls": json.RawMessage("4")}, ContainerdTOML: "[plugins.'io.containerd.grpc.v1.cri']\nimage_pull_with_sync_fs = true\n", Sysctls: map[string]string{"net.ipv4.tcp_retries2": "15"}})
	if err := e.Apply(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(t, root, "etc/default/kubeletconfig.json"), `"maxParallelImagePulls":4`) {
		t.Fatal("customer kubelet value lost")
	}
	flags := read(t, root, "etc/default/kubelet")
	if strings.Contains(flags, "serialize-image-pulls") || !strings.Contains(flags, "cloud-provider=external") {
		t.Fatalf("incorrect flag precedence: %s", flags)
	}
	containerd := read(t, root, "etc/containerd/config.toml")
	if !strings.Contains(containerd, "image_pull_with_sync_fs = true") || !strings.Contains(containerd, "sandbox_image = 'managed'") {
		t.Fatalf("containerd merge failed: %s", containerd)
	}
	if h.values["net.ipv4.tcp_retries2"] != "15" || h.scripts != 1 || h.restarts != 1 {
		t.Fatalf("incorrect activation: %+v", h)
	}
	if err := e.Apply(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	if h.scripts != 1 || h.restarts != 1 || h.writes != 1 {
		t.Fatalf("retry caused unnecessary actions: %+v", h)
	}
	// Simulate AKS regenerating flags during a CSE retry: the journal must not
	// skip finalization simply because this revision succeeded previously.
	put(t, root, "etc/default/kubelet", "KUBELET_FLAGS=--serialize-image-pulls=true\n")
	if err := e.Apply(context.Background(), p, false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, root, "etc/default/kubelet"), "serialize-image-pulls") {
		t.Fatal("CSE retry overwrote customer precedence")
	}
}

func TestFailuresRemainVisibleAndRestoreConfiguration(t *testing.T) {
	t.Run("script", func(t *testing.T) {
		root := t.TempDir()
		h := &fakeHost{failScript: true}
		err := (Executor{Root: root, Host: h}).Apply(context.Background(), profile(t, Spec{BootScript: "#!/bin/bash\nfalse\n"}), false)
		if err == nil || !strings.Contains(err.Error(), "customer boot script failed") {
			t.Fatalf("failure swallowed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, stateDir, "state.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("failed script recorded as applied")
		}
	})
	t.Run("sysctl", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "etc/sysctl.d/99-zz-aks-custom-node-config-poc.conf", "original")
		h := &fakeHost{values: map[string]string{"net.ipv4.tcp_retries2": "8"}, failWrite: true}
		err := (Executor{Root: root, Host: h}).Apply(context.Background(), profile(t, Spec{Sysctls: map[string]string{"net.ipv4.tcp_retries2": "15"}}), true)
		if err == nil {
			t.Fatal("failure swallowed")
		}
		if read(t, root, "etc/sysctl.d/99-zz-aks-custom-node-config-poc.conf") != "original" || h.values["net.ipv4.tcp_retries2"] != "8" {
			t.Fatal("rollback failed")
		}
	})
	t.Run("restart", func(t *testing.T) {
		root := t.TempDir()
		base := "version = 2\n[plugins.'io.containerd.grpc.v1.cri']\nimage_pull_with_sync_fs = false\n"
		put(t, root, "etc/containerd/config.toml", base)
		h := &fakeHost{failRestart: true}
		err := (Executor{Root: root, Host: h}).Apply(context.Background(), profile(t, Spec{ContainerdTOML: "[plugins.'io.containerd.grpc.v1.cri']\nimage_pull_with_sync_fs = true\n"}), false)
		if err == nil || !strings.Contains(err.Error(), "restart restored containerd") {
			t.Fatalf("rollback failure hidden: %v", err)
		}
		if read(t, root, "etc/containerd/config.toml") != base {
			t.Fatal("managed config not restored")
		}
	})
}

func TestRuntimeHashesTargetingAndDisruptionRejection(t *testing.T) {
	p := profile(t, Spec{Sysctls: map[string]string{"net.ipv4.tcp_retries2": "15"}})
	poolData, _ := json.Marshal(PoolProfiles{APIVersion: Version, AgentPools: map[string]Profile{"pool1": p}})
	data, _ := json.Marshal(Document{Components: []ComponentConfig{{Name: Component, NodeConfig: string(poolData)}}})
	sum := sha256.Sum256(data)
	goal := hex.EncodeToString(sum[:])
	h := &fakeHost{values: map[string]string{"net.ipv4.tcp_retries2": "8"}}
	e := Executor{Root: t.TempDir(), Host: h}
	if _, err := Reconcile(context.Background(), goal, append(data, '\n'), "pool1", e); err == nil || h.writes != 0 {
		t.Fatal("stale bytes applied")
	}
	s, err := Reconcile(context.Background(), goal, data, "pool1", e)
	if err != nil || !s.Succeeded(p.Revision) {
		t.Fatalf("did not converge: %+v %v", s, err)
	}
	s, err = Reconcile(context.Background(), goal, data, "pool2", e)
	if err != nil || s.Components[Component].Current != "not-targeted" || h.writes != 1 {
		t.Fatal("pool selection failed")
	}
	if err := profile(t, Spec{Kubelet: map[string]json.RawMessage{"maxParallelImagePulls": json.RawMessage("4")}}).ValidateLive(); err == nil {
		t.Fatal("disruptive update admitted")
	}
}

func TestProfileValidationAndLimits(t *testing.T) {
	p := profile(t, Spec{})
	data, _ := json.Marshal(p)
	if _, err := Decode(data); err != nil {
		t.Fatal(err)
	}
	p.Revision = "tampered"
	data, _ = json.Marshal(p)
	if _, err := Decode(data); err == nil {
		t.Fatal("revision mismatch accepted")
	}
	if _, err := Decode([]byte(strings.Repeat(" ", MaxProfileBytes+1))); err == nil {
		t.Fatal("oversized profile accepted")
	}
	if _, err := NewProfile(Spec{BootScript: "#!/bin/bash\n" + strings.Repeat("x", MaxScriptBytes-len("#!/bin/bash\n"))}); err != nil {
		t.Fatalf("exact script limit rejected: %v", err)
	}
	if _, err := NewProfile(Spec{BootScript: "#!/bin/bash\n" + strings.Repeat("x", MaxScriptBytes)}); err == nil {
		t.Fatal("oversized script accepted")
	}
	if _, err := NewProfile(Spec{Kubelet: map[string]json.RawMessage{"authentication": json.RawMessage(`{}`)}}); err == nil {
		t.Fatal("protected setting accepted")
	}
	if _, err := Decode(append(data, []byte("{}")...)); err == nil {
		t.Fatal("multiple JSON documents accepted")
	}
}

func TestExactEncodedProfileBoundary(t *testing.T) {
	empty := profile(t, Spec{Kubelet: map[string]json.RawMessage{"longTail": json.RawMessage(`""`)}})
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{MaxProfileBytes - 1, MaxProfileBytes, MaxProfileBytes + 1} {
		spec := Spec{Kubelet: map[string]json.RawMessage{"longTail": json.RawMessage(`"` + strings.Repeat("x", size-len(encoded)) + `"`)}}
		p, validationErr := NewProfile(spec)
		data, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != size {
			t.Fatalf("boundary fixture has %d bytes, want %d", len(data), size)
		}
		_, decodeErr := Decode(data)
		if size <= MaxProfileBytes && (validationErr != nil || decodeErr != nil) {
			t.Fatalf("valid %d-byte profile rejected: %v %v", size, validationErr, decodeErr)
		}
		if size > MaxProfileBytes && (validationErr == nil || decodeErr == nil) {
			t.Fatal("over-limit profile accepted")
		}
	}
}

func TestRootCannotEscapeThroughSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "etc")); err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{values: map[string]string{"net.ipv4.tcp_retries2": "8"}}
	err := (Executor{Root: root, Host: h}).Apply(context.Background(), profile(t, Spec{Sysctls: map[string]string{"net.ipv4.tcp_retries2": "15"}}), true)
	if err == nil {
		t.Fatal("escaped node filesystem root")
	}
}

func TestRuntimeReportsFailuresAndUnknownHandlers(t *testing.T) {
	p := profile(t, Spec{Kubelet: map[string]json.RawMessage{"maxParallelImagePulls": json.RawMessage("4")}})
	pools, _ := json.Marshal(PoolProfiles{APIVersion: Version, AgentPools: map[string]Profile{"pool1": p}})
	data, _ := json.Marshal(Document{Components: []ComponentConfig{
		{Name: Component, NodeConfig: string(pools)}, {Name: "futureHandler"},
	}})
	hash := sha256.Sum256(data)
	s, err := Reconcile(context.Background(), hex.EncodeToString(hash[:]), data, "pool1",
		Executor{Root: t.TempDir(), Host: &fakeHost{}})
	if err != nil {
		t.Fatal(err)
	}
	if s.Components[Component].Code != "Failed" || s.Components["futureHandler"].Code != "Unsupported" || s.Succeeded(p.Revision) {
		t.Fatalf("processed hash incorrectly implies success: %+v", s)
	}
	data, _ = json.Marshal(Document{Components: []ComponentConfig{{Name: "futureHandler"}}})
	hash = sha256.Sum256(data)
	if _, err := Reconcile(context.Background(), hex.EncodeToString(hash[:]), data, "pool1", Executor{}); err == nil {
		t.Fatal("missing required handler silently succeeded")
	}
}

func TestInvalidBaselinesAndFlagsFailBeforeActivation(t *testing.T) {
	for _, tc := range []struct {
		name, baseline string
		spec           Spec
	}{
		{"missing kubelet", "", Spec{Kubelet: map[string]json.RawMessage{"maxParallelImagePulls": json.RawMessage("4")}}},
		{"duplicate flags", "KUBELET_FLAGS=--cloud-provider=external\nKUBELET_FLAGS=--other=true\n", Spec{KubeletFlags: map[string]string{"--image-gc-high-threshold": "85"}}},
		{"version three", "version = 3\n[plugins.'io.containerd.cri.v1.images']\nimage_pull_with_sync_fs = false\n", Spec{ContainerdTOML: "[plugins.'io.containerd.cri.v1.images']\nimage_pull_with_sync_fs = true\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if strings.Contains(tc.name, "flags") {
				put(t, root, "etc/default/kubelet", tc.baseline)
			}
			if strings.Contains(tc.name, "version") {
				put(t, root, "etc/containerd/config.toml", tc.baseline)
			}
			h := &fakeHost{}
			if err := (Executor{Root: root, Host: h}).Apply(context.Background(), profile(t, tc.spec), false); err == nil {
				t.Fatal("unsupported baseline accepted")
			}
			if h.restarts != 0 || h.writes != 0 {
				t.Fatal("validation failure activated changes")
			}
		})
	}
}
