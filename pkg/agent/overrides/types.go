package overrides

import (
	"fmt"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	SubscriptionIDFieldName = "subscriptionId"
	TenantIDFieldName       = "tenantId"
)

// Entity is what we resolve overrides against. It contains any and all fields currently
// used to resolve the set of overrides applied to the agentbakersvc instance.
type Entity struct {
	Fields map[string]string
}

func NewEntity() *Entity {
	return &Entity{}
}

func (e *Entity) WithFields(fields map[string]string) *Entity {
	e.Fields = fields
	return e
}

func (e *Entity) FromNodeBootstrappingConfiguration(nbc *datamodel.NodeBootstrappingConfiguration) *Entity {
	e.Fields = map[string]string{
		SubscriptionIDFieldName: nbc.SubscriptionID,
		TenantIDFieldName:       nbc.TenantID,
	}
	return e
}

// Overrides represents the set of overrides to resolve within agentbakersvc requests.
// Overrides are always resolved against Entity's at runtime.
type Overrides struct {
	Overrides map[string]*Override
}

func NewOverrides() *Overrides {
	return &Overrides{
		Overrides: make(map[string]*Override),
	}
}

// Override repesents a single override, parameterized by one or more Rules.
type Override struct {
	Rules           []*Rule           `yaml:"rules"`
	DefaultValue    string            `yaml:"defaultValue"`
	DefaultMapValue map[string]string `yaml:"defaultMapValue"`
}

// Rule represents one or more Matchers to match for a particular value (or values) to be yielded.
// For a rule to yield the value, the particular entity must match all of the Rule's matchers.
type Rule struct {
	Matchers []*Matcher        `yaml:"matchers"`
	Value    string            `yaml:"value"`
	MapValue map[string]string `yaml:"mapValue"`
}

// ValueSet represents a set of values to match against. The underlying type is a map to achieve optimum performance.
type ValueSet map[string]bool

// Matcher matches a particular Entity field against one or more values. Matcher is parameterized by the name
// of the Entity field on which to match, and the set of values to match against for equality.
type Matcher struct {
	Field     string   `yaml:"field"`
	RawValues []string `yaml:"values"`
	Values    ValueSet `yaml:"-"`
}

// UnmarshalYAML is needed to specify custom YAML unmarshaling behavior.
// For any Matcher, we convert the RawValues slice to a ValueSet to support
// faster subsequent lookups. If we later want to add further validation,
// such as enforcing Matcher fields are contained within a predefined set,
// that can be added here as well.
func (m *Matcher) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// we unmarshal to an instance of an alias of the Matcher type to avoid infinitely recursing,
	// as the alias type will not have the UnmarshalYAML method
	type plain Matcher
	if err := unmarshal((*plain)(m)); err != nil {
		return fmt.Errorf("unmarshaling override matcher from yaml: %w", err)
	}
	m.Values = make(ValueSet, len(m.RawValues))
	for _, value := range m.RawValues {
		m.Values[value] = true
	}
	return nil
}
