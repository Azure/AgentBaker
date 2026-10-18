package customnodeconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const stateDir = "var/lib/aks/custom-node-config-poc"

type Host interface {
	RunScript(context.Context, string) error
	ValidateContainerd(context.Context, string) error
	RestartContainerd(context.Context) error
	ReadSysctl(context.Context, string) (string, error)
	WriteSysctl(context.Context, string, string) error
}

type Executor struct {
	Root string
	Host Host
}

type State struct {
	ScriptRevision  string `json:"scriptRevision,omitempty"`
	AppliedRevision string `json:"appliedRevision,omitempty"`
}

type fileChange struct {
	path    string
	before  []byte
	after   []byte
	existed bool
}

func (e Executor) Apply(ctx context.Context, p Profile, live bool) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if live {
		if err := p.ValidateLive(); err != nil {
			return err
		}
	}
	if e.Host == nil || e.Root == "" {
		return fmt.Errorf("executor requires an explicit root and host implementation")
	}
	root, err := os.OpenRoot(e.Root)
	if err != nil {
		return fmt.Errorf("open node root: %w", err)
	}
	defer root.Close()
	if err := root.MkdirAll(stateDir, 0700); err != nil {
		return err
	}
	lock, err := root.OpenFile(stateDir+"/lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("configuration operation already running: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	var state State
	data, err := root.ReadFile(stateDir + "/state.json")
	if err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return fmt.Errorf("read configuration journal: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var disk syscall.Statfs_t
	directory, err := root.Open(".")
	if err != nil {
		return err
	}
	err = syscall.Fstatfs(int(directory.Fd()), &disk)
	directory.Close()
	if err != nil {
		return fmt.Errorf("check available disk: %w", err)
	}
	if disk.Bavail*uint64(disk.Bsize) < 1024*1024 {
		return fmt.Errorf("insufficient disk space: POC requires 1 MiB available")
	}
	if p.Spec.BootScript != "" && state.ScriptRevision != p.Revision {
		scriptCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		err := e.Host.RunScript(scriptCtx, p.Spec.BootScript)
		cancel()
		if err != nil {
			return fmt.Errorf("customer boot script failed (configuration not activated): %w", err)
		}
		state.ScriptRevision = p.Revision
		if err := saveState(root, state); err != nil {
			return err
		}
	}
	changes, err := planFiles(root, p.Spec)
	if err != nil {
		return err
	}
	if p.Spec.ContainerdTOML != "" {
		for _, c := range changes {
			if c.path == "etc/containerd/config.toml" {
				if err := e.Host.ValidateContainerd(ctx, string(c.after)); err != nil {
					return fmt.Errorf("containerd validation failed: %w", err)
				}
			}
		}
	}
	oldSysctls := map[string]string{}
	for _, name := range sortedKeys(p.Spec.Sysctls) {
		value, err := e.Host.ReadSysctl(ctx, name)
		if err != nil {
			return fmt.Errorf("read sysctl %s: %w", name, err)
		}
		oldSysctls[name] = value
	}
	applied := []fileChange{}
	writtenSysctls := []string{}
	containerdChanged := false
	rollback := func(cause error) error {
		var failures []error
		for i := len(writtenSysctls) - 1; i >= 0; i-- {
			name := writtenSysctls[i]
			if err := e.Host.WriteSysctl(ctx, name, oldSysctls[name]); err != nil {
				failures = append(failures, fmt.Errorf("rollback sysctl %s: %w", name, err))
			}
		}
		for i := len(applied) - 1; i >= 0; i-- {
			c := applied[i]
			if c.existed {
				err = atomicWrite(root, c.path, c.before)
			} else {
				err = root.Remove(c.path)
			}
			if err != nil {
				failures = append(failures, fmt.Errorf("rollback %s: %w", c.path, err))
			}
		}
		if containerdChanged {
			if err := e.Host.RestartContainerd(ctx); err != nil {
				failures = append(failures, fmt.Errorf("restart restored containerd: %w", err))
			}
		}
		return errors.Join(append([]error{cause}, failures...)...)
	}
	for _, c := range changes {
		if string(c.before) == string(c.after) {
			continue
		}
		if err := atomicWrite(root, c.path, c.after); err != nil {
			return rollback(fmt.Errorf("activate %s: %w", c.path, err))
		}
		applied = append(applied, c)
		if c.path == "etc/containerd/config.toml" {
			containerdChanged = true
		}
	}
	for _, name := range sortedKeys(p.Spec.Sysctls) {
		if oldSysctls[name] == p.Spec.Sysctls[name] {
			continue
		}
		// A failing write may have partially taken effect, so include it in compensation.
		writtenSysctls = append(writtenSysctls, name)
		if err := e.Host.WriteSysctl(ctx, name, p.Spec.Sysctls[name]); err != nil {
			return rollback(fmt.Errorf("write sysctl %s: %w", name, err))
		}
		actual, err := e.Host.ReadSysctl(ctx, name)
		if err != nil || actual != p.Spec.Sysctls[name] {
			return rollback(fmt.Errorf("sysctl %s did not converge: actual=%q: %w", name, actual, errors.Join(err, errors.New("effective value mismatch"))))
		}
	}
	if containerdChanged {
		if err := e.Host.RestartContainerd(ctx); err != nil {
			return rollback(fmt.Errorf("activate containerd: %w", err))
		}
	}
	state.AppliedRevision = p.Revision
	if err := saveState(root, state); err != nil {
		return rollback(fmt.Errorf("persist applied revision: %w", err))
	}
	return nil
}

func saveState(root *os.Root, state State) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return atomicWrite(root, stateDir+"/state.json", data)
}

func atomicWrite(root *os.Root, path string, data []byte) error {
	if err := root.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".poc-tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer root.Remove(tmp)
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return root.Rename(tmp, path)
}

func planFiles(root *os.Root, spec Spec) ([]fileChange, error) {
	var changes []fileChange
	add := func(path string, render func([]byte) ([]byte, error)) error {
		old, err := root.ReadFile(path)
		existed := err == nil
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next, err := render(old)
		if err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		changes = append(changes, fileChange{path, old, next, existed})
		return nil
	}
	if len(spec.Kubelet) > 0 {
		if err := add("etc/default/kubeletconfig.json", func(old []byte) ([]byte, error) {
			if len(old) == 0 {
				return nil, fmt.Errorf("managed kubelet configuration file is required; phase-3/equivalent baseline unavailable")
			}
			var base map[string]json.RawMessage
			if err := json.Unmarshal(old, &base); err != nil {
				return nil, err
			}
			if base == nil {
				return nil, fmt.Errorf("managed kubelet configuration must be an object")
			}
			for k, v := range spec.Kubelet {
				base[k] = v
			}
			return json.Marshal(base)
		}); err != nil {
			return nil, err
		}
	}
	if len(spec.KubeletFlags) > 0 || len(spec.Kubelet) > 0 {
		if err := add("etc/default/kubelet", func(old []byte) ([]byte, error) { return mergeFlags(old, spec) }); err != nil {
			return nil, err
		}
	}
	if spec.ContainerdTOML != "" {
		if err := add("etc/containerd/config.toml", func(old []byte) ([]byte, error) {
			if len(old) == 0 {
				return nil, fmt.Errorf("managed containerd configuration is required")
			}
			var base, overlay map[string]any
			if err := toml.Unmarshal(old, &base); err != nil {
				return nil, err
			}
			if err := toml.Unmarshal([]byte(spec.ContainerdTOML), &overlay); err != nil {
				return nil, err
			}
			version, ok := base["version"].(int64)
			if !ok || version != 2 {
				return nil, fmt.Errorf("POC only admits containerd configuration version 2")
			}
			if err := validatePluginOverlay(base, overlay); err != nil {
				return nil, err
			}
			mergeMaps(base, overlay)
			return toml.Marshal(base)
		}); err != nil {
			return nil, err
		}
	}
	if len(spec.Sysctls) > 0 {
		if err := add("etc/sysctl.d/99-zz-aks-custom-node-config-poc.conf", func([]byte) ([]byte, error) {
			var b strings.Builder
			for _, name := range sortedKeys(spec.Sysctls) {
				fmt.Fprintf(&b, "%s = %s\n", name, spec.Sysctls[name])
			}
			return []byte(b.String()), nil
		}); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

func validatePluginOverlay(base, overlay map[string]any) error {
	plugins, ok := overlay["plugins"].(map[string]any)
	if !ok {
		return fmt.Errorf("containerd overlay must contain a plugins table")
	}
	baseline, _ := base["plugins"].(map[string]any)
	for name, value := range plugins {
		table, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("containerd plugin %s must be a table", name)
		}
		if _, ok := baseline[name]; !ok {
			return fmt.Errorf("containerd plugin %s is absent from managed baseline", name)
		}
		for key := range table {
			if key != "image_pull_with_sync_fs" && key != "image_pull_progress_timeout" {
				return fmt.Errorf("containerd plugin field %s.%s is not admitted by POC", name, key)
			}
		}
	}
	return nil
}

func mergeMaps(base, overlay map[string]any) {
	for k, v := range overlay {
		child, childOK := v.(map[string]any)
		existing, existingOK := base[k].(map[string]any)
		if childOK && existingOK {
			mergeMaps(existing, child)
		} else {
			base[k] = v
		}
	}
}

func mergeFlags(old []byte, spec Spec) ([]byte, error) {
	lines := strings.Split(string(old), "\n")
	found := false
	remove := map[string]bool{}
	for name := range spec.KubeletFlags {
		remove[name] = true
	}
	fieldFlags := map[string]string{"imageGCHighThresholdPercent": "--image-gc-high-threshold", "imageGCLowThresholdPercent": "--image-gc-low-threshold", "kubeReserved": "--kube-reserved", "systemReserved": "--system-reserved", "evictionHard": "--eviction-hard", "evictionSoft": "--eviction-soft", "serializeImagePulls": "--serialize-image-pulls"}
	for field := range spec.Kubelet {
		if flag := fieldFlags[field]; flag != "" {
			remove[flag] = true
		}
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "KUBELET_FLAGS=") {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate KUBELET_FLAGS definitions")
		}
		found = true
		var flags []string
		for _, flag := range strings.Fields(strings.TrimPrefix(line, "KUBELET_FLAGS=")) {
			name, _, _ := strings.Cut(flag, "=")
			if remove[name] {
				if !strings.Contains(flag, "=") {
					return nil, fmt.Errorf("space-separated flag %s is unsupported by POC", name)
				}
				continue
			}
			flags = append(flags, flag)
		}
		for _, name := range sortedKeys(spec.KubeletFlags) {
			flags = append(flags, name+"="+spec.KubeletFlags[name])
		}
		lines[i] = "KUBELET_FLAGS=" + strings.Join(flags, " ")
	}
	if !found {
		return nil, fmt.Errorf("managed KUBELET_FLAGS baseline unavailable")
	}
	return []byte(strings.Join(lines, "\n")), nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type LinuxHost struct{}

func (LinuxHost) RunScript(ctx context.Context, script string) error {
	cmd := exec.CommandContext(ctx, "/bin/bash", "-e", "-s")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = time.Second
	cmd.Stdin = strings.NewReader(script)
	// Never copy customer script output (which may contain secrets) into CSE logs.
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}
func (LinuxHost) ValidateContainerd(ctx context.Context, config string) error {
	f, err := os.CreateTemp("/etc/containerd", "custom-node-config-poc-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.WriteString(config)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "containerd", "--config", f.Name(), "config", "dump").Run()
}
func (LinuxHost) RestartContainerd(ctx context.Context) error {
	if err := exec.CommandContext(ctx, "systemctl", "restart", "containerd").Run(); err != nil {
		return err
	}
	return exec.CommandContext(ctx, "crictl", "--runtime-endpoint=unix:///run/containerd/containerd.sock", "info").Run()
}
func (LinuxHost) ReadSysctl(ctx context.Context, name string) (string, error) {
	out, err := exec.CommandContext(ctx, "sysctl", "-n", name).Output()
	return strings.TrimSpace(string(out)), err
}
func (LinuxHost) WriteSysctl(ctx context.Context, name, value string) error {
	return exec.CommandContext(ctx, "sysctl", "-q", "-w", name+"="+value).Run()
}
