package overrides

func (m *Matcher) Matches(entity *Entity) bool {
	var field string
	switch m.Field {
	case SubscriptionID:
		field = entity.SubscriptionID
	case TenantID:
		field = entity.TenantID
	default:
		// this should never happen since we validate Matcher field names at unmarshal time
		return false
	}
	return m.Values[field]
}

func (r *Rule) SatisfiedBy(entity *Entity) bool {
	for _, m := range r.Matchers {
		if !m.Matches(entity) {
			return false
		}
	}
	return true
}

// getString returns the string associated with the **first** rule matched within the named override.
func (o *Overrides) getString(overrideName string, entity *Entity) string {
	override, ok := o.Overrides[overrideName]
	if !ok {
		// should we log this out?
		return ""
	}
	for _, rule := range override.Rules {
		if rule.SatisfiedBy(entity) {
			return rule.Value
		}
	}
	return ""
}

func (o *Overrides) getMap(overrideName string, entity *Entity) map[string]string {
	override, ok := o.Overrides[overrideName]
	if !ok {
		// should we log this out?
		return nil
	}
	for _, rule := range override.Rules {
		if rule.SatisfiedBy(entity) {
			return rule.MapValue
		}
	}
	return nil
}

// All agentbakersvc overrides go below

// GetLinuxNodeImageVersionOverrides returns the Linux node image version overrides
func (o *Overrides) GetLinuxNodeImageVersionOverrides(entity *Entity) map[string]string {
	return o.getMap("linux-node-image-version-override", entity)
}
