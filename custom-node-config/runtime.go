package customnodeconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type ComponentConfig struct {
	Name       string `json:"name"`
	NodeConfig string `json:"nodeConfig,omitempty"`
}
type Document struct {
	Components []ComponentConfig `json:"components"`
}
type ComponentStatus struct {
	Code    string `json:"code"`
	Current string `json:"current,omitempty"`
	Message string `json:"message,omitempty"`
}
type Status struct {
	CurrentHash string                     `json:"currentHash"`
	Components  map[string]ComponentStatus `json:"components"`
}

// Reconcile processes only this POC's handler; unsupported components stay visible,
// rather than making a generic goal hash look like successful execution.
func Reconcile(ctx context.Context, goal string, data []byte, pool string, e Executor) (Status, error) {
	status := Status{Components: map[string]ComponentStatus{}}
	sum := sha256.Sum256(data)
	if goal == "" || goal != hex.EncodeToString(sum[:]) {
		return status, fmt.Errorf("goal/config hash mismatch; configuration not applied")
	}
	if len(data) > 256*1024 {
		return status, fmt.Errorf("runtime document exceeds POC limit")
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return status, fmt.Errorf("decode runtime document: %w", err)
	}
	seen := map[string]bool{}
	for _, c := range doc.Components {
		if c.Name == "" || seen[c.Name] {
			return status, fmt.Errorf("empty or duplicate runtime component %q", c.Name)
		}
		seen[c.Name] = true
	}
	if !seen[Component] {
		return status, fmt.Errorf("required component %q is missing", Component)
	}
	for _, c := range doc.Components {
		if c.Name != Component {
			status.Components[c.Name] = ComponentStatus{Code: "Unsupported", Message: "handler not installed in reference POC"}
			continue
		}
		var profiles PoolProfiles
		if err := json.Unmarshal([]byte(c.NodeConfig), &profiles); err != nil {
			return status, fmt.Errorf("decode pool configuration: %w", err)
		}
		if profiles.APIVersion != Version {
			return status, fmt.Errorf("unsupported pool configuration version")
		}
		p, ok := profiles.AgentPools[pool]
		if !ok {
			status.Components[c.Name] = ComponentStatus{Code: "Succeeded", Current: "not-targeted"}
			continue
		}
		if err := e.Apply(ctx, p, true); err != nil {
			status.Components[c.Name] = ComponentStatus{Code: "Failed", Current: p.Revision, Message: err.Error()}
		} else {
			status.Components[c.Name] = ComponentStatus{Code: "Succeeded", Current: p.Revision}
		}
	}
	status.CurrentHash = goal
	return status, nil
}

func (s Status) Succeeded(revision string) bool {
	c, ok := s.Components[Component]
	return ok && c.Code == "Succeeded" && c.Current == revision
}
