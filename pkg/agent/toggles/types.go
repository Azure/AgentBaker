package toggles

import (
	"encoding/json"
	"log"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// Entity is what we resolve toggles against. It contains any and all fields currently
// used to resolve the set of toggles applied to the agentbakersvc instance.
type Entity struct {
	SubscriptionID string
	TenantID       string
	Region         string
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

// NewEntityFromEnvironmentInfo constructs and returns a new Entity populated with fields
// from the specified EnvironmentInfo.
func NewEntityFromEnvironmentInfo(envInfo *datamodel.EnvironmentInfo) *Entity {
	return &Entity{
		SubscriptionID: envInfo.SubscriptionID,
		TenantID:       envInfo.TenantID,
		Region:         envInfo.Region,
	}
}

// NewEntityFromNodeBootstrappingConfiguration constructs and returns a new Entity with fields
// from the specified NodeBootstrappingConfiguration.
func NewEntityFromNodeBootstrappingConfiguration(nbc *datamodel.NodeBootstrappingConfiguration) *Entity {
	return &Entity{
		SubscriptionID: nbc.SubscriptionID,
		TenantID:       nbc.TenantID,
		Region:         nbc.ContainerService.Location,
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
