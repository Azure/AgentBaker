package toggles

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/agent/toggles/fieldnames"
)

// Entity is what we resolve overrides against. It contains any and all fields currently
// used to resolve the set of overrides applied to the agentbakersvc instance.
type Entity struct {
	Fields map[string]string
}

// NewEntity constructs a new Entity from the specified fields.
func NewEntity(fields map[string]string) *Entity {
	return &Entity{
		Fields: fields,
	}
}

func NewEntityFromEnvironmentConfig(ctx *datamodel.EnvironmentConfig) *Entity {
	return &Entity{
		Fields: map[string]string{
			fieldnames.SubscriptionID: ctx.SubscriptionID,
			fieldnames.TenantID:       ctx.TenantID,
			fieldnames.Region:         ctx.Region,
		},
	}
}

func NewEntityFromNodeBootstrappingConfiguration(nbc *datamodel.NodeBootstrappingConfiguration) *Entity {
	return &Entity{
		Fields: map[string]string{
			fieldnames.SubscriptionID: nbc.SubscriptionID,
			fieldnames.TenantID:       nbc.TenantID,
			fieldnames.Region:         nbc.ContainerService.Location,
		},
	}
}

// MapToggle represents a toggle which resolves a map against a specified Entity.
type MapToggle func(entity *Entity) map[string]string

// StringToggle represents a toggle which resolves a string against a specified Entity.
type StringToggle func(entity *Entity) string

// Toggles represents a set of toggles to use within a service context.
type Toggles struct {
	// MapToggles is the set of toggles which return map values.
	MapToggles map[string]MapToggle
	// StringToggles is the set of toggles which return string values
	StringToggles map[string]StringToggle
}

func NewToggles() *Toggles {
	return &Toggles{
		MapToggles:    make(map[string]MapToggle),
		StringToggles: make(map[string]StringToggle),
	}
}

func (t *Toggles) getMap(toggleName string, entity *Entity) map[string]string {
	if toggle, ok := t.MapToggles[toggleName]; ok {
		return toggle(entity)
	}
	return map[string]string{}
}

func (t *Toggles) getString(toggleName string, entity *Entity) string {
	if toggle, ok := t.StringToggles[toggleName]; ok {
		return toggle(entity)
	}
	return ""
}
