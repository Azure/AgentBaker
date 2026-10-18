package customnodeconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	Version         = "aks.custom-node-config/v1alpha1"
	Component       = "customNodeConfiguration"
	ProfilePath     = "/opt/azure/containers/custom-node-config-poc.json"
	MaxProfileBytes = 32 * 1024
	MaxScriptBytes  = 16 * 1024
)

type Spec struct {
	BootScript     string                     `json:"bootScript,omitempty"`
	Kubelet        map[string]json.RawMessage `json:"kubelet,omitempty"`
	KubeletFlags   map[string]string          `json:"kubeletFlags,omitempty"`
	ContainerdTOML string                     `json:"containerdTOML,omitempty"`
	Sysctls        map[string]string          `json:"sysctls,omitempty"`
}

type Profile struct {
	APIVersion string `json:"apiVersion"`
	Revision   string `json:"revision"`
	Spec       Spec   `json:"spec"`
}

type PoolProfiles struct {
	APIVersion string             `json:"apiVersion"`
	AgentPools map[string]Profile `json:"agentPools"`
}

var settingName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_.-]*$`)
var flagValue = regexp.MustCompile(`^[a-zA-Z0-9_.,:/=+-]+$`)

func NewProfile(spec Spec) (Profile, error) {
	p := Profile{APIVersion: Version, Spec: spec}
	data, err := json.Marshal(spec)
	if err != nil {
		return p, fmt.Errorf("encode specification: %w", err)
	}
	sum := sha256.Sum256(data)
	p.Revision = hex.EncodeToString(sum[:])
	return p, p.Validate()
}

func Decode(data []byte) (Profile, error) {
	var p Profile
	if len(data) > MaxProfileBytes {
		return p, fmt.Errorf("profile exceeds POC limit %d bytes", MaxProfileBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return p, fmt.Errorf("decode profile: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("profile must contain exactly one JSON document")
	}
	return p, p.Validate()
}

func (p Profile) Validate() error {
	if p.APIVersion != Version {
		return fmt.Errorf("unsupported configuration version %q", p.APIVersion)
	}
	data, err := json.Marshal(p.Spec)
	if err != nil {
		return fmt.Errorf("encode specification: %w", err)
	}
	sum := sha256.Sum256(data)
	if p.Revision != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("configuration revision does not match specification")
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	if len(encoded) > MaxProfileBytes {
		return fmt.Errorf("profile exceeds POC limit %d bytes", MaxProfileBytes)
	}
	if len(p.Spec.BootScript) > MaxScriptBytes {
		return fmt.Errorf("boot script exceeds POC limit %d bytes", MaxScriptBytes)
	}
	if p.Spec.BootScript != "" && !strings.HasPrefix(p.Spec.BootScript, "#!/bin/bash\n") {
		return fmt.Errorf("POC boot content must be a bash script; cloud-config/MIME input is not implemented")
	}
	protected := map[string]bool{"authentication": true, "authorization": true, "clusterDNS": true, "clusterDomain": true, "tlsCertFile": true, "tlsPrivateKeyFile": true, "serverTLSBootstrap": true, "rotateCertificates": true, "cgroupDriver": true, "staticPodPath": true, "readOnlyPort": true, "featureGates": true, "kind": true, "apiVersion": true}
	for name, value := range p.Spec.Kubelet {
		if !settingName.MatchString(name) || protected[name] {
			return fmt.Errorf("kubelet field %q is not admitted by the POC", name)
		}
		if !json.Valid(value) || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fmt.Errorf("kubelet field %q must contain a non-null JSON value", name)
		}
	}
	for name, value := range p.Spec.KubeletFlags {
		if !strings.HasPrefix(name, "--") || !settingName.MatchString(strings.TrimPrefix(name, "--")) || !flagValue.MatchString(value) {
			return fmt.Errorf("invalid kubelet flag %q or value", name)
		}
		// POC flags are deliberately narrow until the protected-flag contract is reviewed.
		if name != "--image-gc-high-threshold" && name != "--image-gc-low-threshold" {
			return fmt.Errorf("kubelet flag %q is not admitted by the POC", name)
		}
	}
	if p.Spec.ContainerdTOML != "" {
		var overlay map[string]any
		if err := toml.Unmarshal([]byte(p.Spec.ContainerdTOML), &overlay); err != nil {
			return fmt.Errorf("parse containerd overlay: %w", err)
		}
		for name := range overlay {
			if name != "plugins" {
				return fmt.Errorf("containerd top-level field %q is not admitted by the POC", name)
			}
		}
	}
	for name, value := range p.Spec.Sysctls {
		if !settingName.MatchString(name) || value == "" || strings.ContainsAny(value, "\r\n\x00") {
			return fmt.Errorf("invalid sysctl %q or value", name)
		}
	}
	return nil
}

func (p Profile) Runtime() (Profile, error) {
	spec := p.Spec
	spec.BootScript = ""
	return NewProfile(spec)
}

func (p Profile) ValidateLive() error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Spec.BootScript != "" {
		return fmt.Errorf("boot scripts cannot be replayed at runtime")
	}
	if len(p.Spec.Kubelet) > 0 || len(p.Spec.KubeletFlags) > 0 || p.Spec.ContainerdTOML != "" {
		return fmt.Errorf("kubelet/containerd updates require disruption orchestration; use replacement/reimage in this POC")
	}
	for name, value := range p.Spec.Sysctls {
		if name != "net.ipv4.tcp_retries2" || value != "15" {
			return fmt.Errorf("live sysctl %q=%q is not admitted; POC only restores net.ipv4.tcp_retries2=15", name, value)
		}
	}
	return nil
}
