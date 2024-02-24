package overrides

import (
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

// EntityField represents a named field of the Entity type to use for evaluating overrides. We primarily use this
// to ensure the field names from override yaml definitions are valid, and to avoid usage of reflect.
type EntityField int

const (
	SubscriptionID EntityField = iota
	TenantID
)

// Fields holds a mapping between all valid field name strings and their corresponding EntityField value.
var Fields = map[string]EntityField{
	strings.ToLower("subscriptionId"): SubscriptionID,
	strings.ToLower("tenantId"):       TenantID,
}

// Entity is what we resolve overrides against. It contains any and all fields currently
// used to resolve the set of overrides applied to the agentbakersvc instance.
type Entity struct {
	SubscriptionID string
	TenantID       string
}

func NewEntityFromNodeBootstrappingConfiguration(nbc *datamodel.NodeBootstrappingConfiguration) *Entity {
	entity := &Entity{}
	if nbc != nil {
		// should we log something out cases where nbc is nil during entity construction?
		entity.SubscriptionID = nbc.SubscriptionID
		entity.TenantID = nbc.TenantID
	}
	return entity
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
	Rules []*Rule `yaml:"rules,omitempty"`
}

// Rule represents one or more Matchers to match for a particular value (or values) to be yielded.
// For a rule to yield the value, the particular entity must match all of the Rule's matchers.
type Rule struct {
	Matchers []*Matcher        `yaml:"matchers,omitempty"`
	Value    string            `yaml:"value"`
	MapValue map[string]string `yaml:"mapValue,omitempty"`
}

// ValueSet represents a set of values to match against. The underlying type is a map to achieve optimum performance.
type ValueSet map[string]bool

// Matcher matches a particular Entity field against one or more values. Matcher is parameterized by the name
// of the Entity field on which to match, and the set of values to match against for equality.
type Matcher struct {
	RawField  string      `yaml:"field"`
	RawValues []string    `yaml:"values"`
	Field     EntityField `yaml:"-"`
	Values    ValueSet    `yaml:"-"`
}

func (m *Matcher) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// we unmarshal to an instance of an alias of the Matcher type to avoid infinitely recursing,
	// as the alias type will not have the UnmarshalYAML method
	type plain Matcher
	if err := unmarshal((*plain)(m)); err != nil {
		return fmt.Errorf("unmarshaling override matcher from yaml: %w", err)
	}
	field, ok := Fields[strings.ToLower(m.RawField)]
	if !ok {
		return fmt.Errorf("unrecognized Entity field for agentbakersvc override matcher: %q", m.RawField)
	}
	m.Field = field
	m.Values = make(ValueSet, len(m.RawValues))
	for _, value := range m.RawValues {
		m.Values[value] = true
	}
	return nil
}
