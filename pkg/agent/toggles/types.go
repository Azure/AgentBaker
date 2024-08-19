package toggles

import (
	"encoding/json"
	"log"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/Azure/agentbaker/pkg/agent/toggles/fieldnames"
)

// Entity is what we resolve toggles against. It contains any and all fields currently
// used to resolve the set of toggles applied to the agentbakersvc instance.
type Entity struct {
	Fields map[string]string `json:"fields"`
}

type Toggles interface {
	GetLinuxNodeImageVersion(entity *Entity, distro datamodel.Distro) string
}

type defaultToggles struct{}

func (t *defaultToggles) GetLinuxNodeImageVersion(entity *Entity, distro datamodel.Distro) string {
	return ""
}

func NewDefaultToggles() Toggles {
	return &defaultToggles{}
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

func (e *Entity) String() string {
	return marshalToString(e)
}

func marshalToString(obj any) string {
	raw, err := json.Marshal(obj)
	if err != nil {
		log.Printf("error marshalling JSON object for logs: %s", err)
		return ""
	}
	return string(raw)
}
