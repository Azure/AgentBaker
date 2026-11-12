// Package status defines the generic live-patching node contract: the annotations the live-patching controller (LPC)
// and the node-side live-patching loop exchange on each node, the component names they carry, and the
// live-patching-status document the node writes back.
//
// The node-side writer of this contract is the live-patching loop on the VHD
// (parts/root/shared/opt/azure/containers/ubuntu-snapshot-update.sh and
// parts/root/shared/opt/azure/containers/mariner-package-update.sh). Keep the two in sync.
package status

import (
	"encoding/json"
	"fmt"
)

const (
	// GoalHashAnnotationKey is stamped on each node by LPC with the sha256 of the live-patching-config the node
	// should apply. A change triggers the node-side live-patching loop.
	GoalHashAnnotationKey = "kubernetes.azure.com/live-patching-config-goal-hash"

	// StatusAnnotationKey is written by the node-side live-patching loop with a JSON Status document: the goal hash
	// the node last processed plus each component's result, e.g.
	//
	//	{"currentHash":"<sha256>","components":{"securityPatch":{"code":"Succeeded"}}}
	StatusAnnotationKey = "kubernetes.azure.com/live-patching-status"
)

// Live-patching component names, as used in the live-patching-config and in Status.Components.
const (
	ComponentSecurityPatch     = "securityPatch"
	ComponentNPDConfig         = "npdConfig"
	ComponentNPDBinary         = "npdBinary"
	ComponentLocalDNS          = "localDNS"
	ComponentAKSNodeController = "aksNodeController"
)

// CodeSucceeded is the component result code for a successful apply. Any other code (for example "Failed") means
// the component has not been applied successfully and the node-side loop will retry it.
const CodeSucceeded = "Succeeded"

// Status is the document stored in StatusAnnotationKey.
type Status struct {
	// CurrentHash is the goal hash of the live-patching-config the node last processed. The node advances it
	// best-effort, even if some component failed; per-component results are in Components.
	CurrentHash string `json:"currentHash,omitempty"`
	// Components maps a component name to its last result.
	Components map[string]ComponentStatus `json:"components,omitempty"`
}

// ComponentStatus is one component's node-reported result. Only Code has a fixed meaning; Current and Message are
// free-form and set by the component's node handler.
type ComponentStatus struct {
	// Code is the component's result, such as CodeSucceeded or "Failed".
	Code string `json:"code,omitempty"`
	// Current is a free-form value the handler reports, such as a version or a golden timestamp.
	Current string `json:"current,omitempty"`
	// Message is an optional human-readable detail, typically set on failure.
	Message string `json:"message,omitempty"`
}

// Parse parses the value of StatusAnnotationKey. An empty value yields a zero Status and no error, representing a
// node that has not reported yet.
func Parse(data string) (*Status, error) {
	s := &Status{}
	if data == "" {
		return s, nil
	}
	if err := json.Unmarshal([]byte(data), s); err != nil {
		return nil, fmt.Errorf("parse %s annotation: %w", StatusAnnotationKey, err)
	}
	return s, nil
}

// IsProcessed reports whether the node has processed the live-patching-config with the given goal hash. It says
// nothing about whether each component succeeded; see IsConverged and ComponentSucceeded.
func (s *Status) IsProcessed(goalHash string) bool {
	return s != nil && goalHash != "" && s.CurrentHash == goalHash
}

// IsConverged reports whether the node has processed the given goal hash and every reported component succeeded.
func (s *Status) IsConverged(goalHash string) bool {
	if !s.IsProcessed(goalHash) {
		return false
	}
	for _, c := range s.Components {
		if c.Code != CodeSucceeded {
			return false
		}
	}
	return true
}

// ComponentSucceeded reports whether the node has processed the given goal hash and the named component succeeded.
// Use it when only one component matters, for example the RP waiting on securityPatch, so that failures of
// unrelated components do not block it.
func (s *Status) ComponentSucceeded(goalHash, component string) bool {
	if !s.IsProcessed(goalHash) {
		return false
	}
	c, ok := s.Components[component]
	return ok && c.Code == CodeSucceeded
}
