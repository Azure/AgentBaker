package toggles

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/agent/toggles/fieldnames"
)

// Entity is what we resolve toggles against. It contains any and all fields currently
// used to resolve the set of toggles applied to the agentbakersvc instance.
type Entity struct {
	Fields map[string]string
}

// NewEntity constructs a new Entity from the specified fields.
func NewEntity(fields map[string]string) *Entity {
	return &Entity{
		Fields: fields,
	}
}

// NewEntityFromEnvironmentInfo constructs and returns a new Entity populated with fields
// from the specified EnvironmentInfo.
func NewEntityFromEnvironmentInfo(envInfo *datamodel.EnvironmentInfo) *Entity {
	return &Entity{
		Fields: map[string]string{
			fieldnames.SubscriptionID: envInfo.SubscriptionID,
			fieldnames.TenantID:       envInfo.TenantID,
			fieldnames.Region:         envInfo.Region,
		},
	}
}

// NewEntityFromNodeBootstrappingConfiguration constructs and returns a new Entity with fields
// from the specified NodeBootstrappingConfiguration.
func NewEntityFromNodeBootstrappingConfiguration(nbc *datamodel.NodeBootstrappingConfiguration) *Entity {
	return &Entity{
		Fields: map[string]string{
			fieldnames.SubscriptionID: nbc.SubscriptionID,
			fieldnames.TenantID:       nbc.TenantID,
			fieldnames.Region:         nbc.ContainerService.Location,
		},
	}
}

// MapToggle is a toggle which resolves a map against a specified Entity.
type MapToggle func(entity *Entity) map[string]string

// StringToggle is a toggle which resolves a string against a specified Entity.
type StringToggle func(entity *Entity) string

// Toggles is a set of toggles to run the agentbakersvc instance with.
type Toggles struct {
	// Maps is the set of toggles which return map values.
	Maps map[string]MapToggle
	// Strings is the set of toggles which return string values
	Strings map[string]StringToggle
}

// New constructs a new and empty set of toggles.
func New() *Toggles {
	return &Toggles{
		Maps:    make(map[string]MapToggle),
		Strings: make(map[string]StringToggle),
	}
}

// getMap attempts to resolve the named map toggle against the specified Entity.
func (t *Toggles) getMap(name string, entity *Entity) map[string]string {
	if t != nil && t.Maps != nil {
		if toggle, ok := t.Maps[name]; ok {
			return toggle(entity)
		}
	}
	return map[string]string{}
}

// getString attempts to resolve the named string toggle against the specified Entity.
func (t *Toggles) getString(name string, entity *Entity) string {
	if t != nil && t.Strings != nil {
		if toggle, ok := t.Strings[name]; ok {
			return toggle(entity)
		}
	}
	return ""
}
