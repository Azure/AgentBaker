package overrides

// Matches returns true iff the specified entity contains the required field
// and the corresponding value is present within the Matcher's value set.
func (m *Matcher) Matches(entity *Entity) bool {
	entityValue, ok := entity.Fields[m.Field]
	if !ok {
		// TODO(cameissner): add logging for these cases
		return false
	}
	return m.Values[entityValue]
}

// SatisfiedBy returns true iff the Rule is satisfied by the specified entity.
// A Rule is only satisfied by a given entity if the entity matches **all**
// of the Rule's Matchers.
func (r *Rule) SatisfiedBy(entity *Entity) bool {
	for _, m := range r.Matchers {
		if !m.Matches(entity) {
			return false
		}
	}
	return true
}

// getString returns the string value associated with the **first** rule matched within the named override.
func (o *Overrides) getString(name string, entity *Entity) string {
	override, ok := o.Overrides[name]
	if !ok {
		// TODO(cameissner): add logging for these cases
		return ""
	}
	for _, rule := range override.Rules {
		if rule.SatisfiedBy(entity) {
			return rule.Value
		}
	}
	return override.DefaultValue
}

// getMap returns the map value associated with the **first** rule matched within the named override.
func (o *Overrides) getMap(name string, entity *Entity) map[string]string {
	override, ok := o.Overrides[name]
	if !ok {
		// TODO(cameissner): add logging for these cases
		return map[string]string{}
	}
	for _, rule := range override.Rules {
		if rule.SatisfiedBy(entity) {
			return rule.MapValue
		}
	}
	return override.DefaultMapValue
}
