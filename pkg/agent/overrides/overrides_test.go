package overrides

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("overrides", func() {
	var e *Entity

	BeforeEach(func() {
		e = &Entity{
			SubscriptionID: "subscriptionId",
			TenantID:       "tenantId",
		}
	})

	Context("Matcher tests", func() {
		When("entity does not match", func() {
			It("Matches() should return false", func() {
				m := &Matcher{
					RawField:  "subscriptionId",
					RawValues: []string{"subscription"},
					Field:     SubscriptionID,
					Values:    ValueSet{"subscription": true},
				}
				matches := m.Matches(e)
				Expect(matches).To(BeFalse())
			})
		})

		When("entity does match", func() {
			It("Matches() should return true", func() {
				m := &Matcher{
					RawField:  "subscriptionId",
					RawValues: []string{"subscriptionId"},
					Field:     SubscriptionID,
					Values:    ValueSet{"subscriptionId": true},
				}
				matches := m.Matches(e)
				Expect(matches).To(BeTrue())
			})
		})
	})

	Context("Rule tests", func() {
		When("all matchers fail", func() {
			It("should not be satisfied", func() {
				r := &Rule{
					Matchers: []*Matcher{
						{
							RawField:  "subscriptionId",
							RawValues: []string{"subscription"},
							Field:     SubscriptionID,
							Values:    ValueSet{"subscription": true},
						},
						{
							RawField:  "tenantId",
							RawValues: []string{"tenant"},
							Field:     TenantID,
							Values:    ValueSet{"tenant": true},
						},
					},
					Value: "value",
				}
				satisfied := r.SatisfiedBy(e)
				Expect(satisfied).To(BeFalse())
			})
		})

		When("at least one matcher fails", func() {
			It("should not be satisfied", func() {
				r := &Rule{
					Matchers: []*Matcher{
						{
							RawField:  "subscriptionId",
							RawValues: []string{"subscriptionId"},
							Field:     SubscriptionID,
							Values:    ValueSet{"subscriptionId": true},
						},
						{
							RawField:  "tenantId",
							RawValues: []string{"tenant"},
							Field:     TenantID,
							Values:    ValueSet{"tenant": true},
						},
					},
					Value: "value",
				}
				satisfied := r.SatisfiedBy(e)
				Expect(satisfied).To(BeFalse())
			})
		})

		When("all matchers succeed", func() {
			It("should be satisifed", func() {
				r := &Rule{
					Matchers: []*Matcher{
						{
							RawField:  "subscriptionId",
							RawValues: []string{"subscriptionId"},
							Field:     SubscriptionID,
							Values:    ValueSet{"subscriptionId": true},
						},
						{
							RawField:  "tenantId",
							RawValues: []string{"tenantId"},
							Field:     TenantID,
							Values:    ValueSet{"tenantId": true},
						},
					},
					Value: "value",
				}
				satisfied := r.SatisfiedBy(e)
				Expect(satisfied).To(BeTrue())
			})
		})
	})

	Context("Overrides tests", func() {
		Context("getString tests", func() {
			When("the specified override is not found", func() {
				It("should return an empty string", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										RawField:  "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Field:     SubscriptionID,
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										RawField:  "tenantId",
										RawValues: []string{"tenantId"},
										Field:     TenantID,
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value",
							},
						},
					}
					overrides.Overrides["o1"] = o
					str := overrides.getString("o2", e)
					Expect(str).To(BeEmpty())
				})
			})

			When("no matchers are satisfied", func() {
				It("should return an empty string", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										RawField:  "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Field:     SubscriptionID,
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										RawField:  "tenantId",
										RawValues: []string{"tenantId"},
										Field:     TenantID,
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value",
							},
						},
					}
					overrides.Overrides["o1"] = o
					e.SubscriptionID = "someOtherSubscription"
					e.TenantID = "someOtherTenant"
					str := overrides.getString("o1", e)
					Expect(str).To(BeEmpty())
				})
			})

			When("the singular rule is satisifed", func() {
				It("should return the string value corresponding to the rule", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										RawField:  "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Field:     SubscriptionID,
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										RawField:  "tenantId",
										RawValues: []string{"tenantId"},
										Field:     TenantID,
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value",
							},
						},
					}
					overrides.Overrides["o1"] = o
					str := overrides.getString("o1", e)
					Expect(str).To(Equal(o.Rules[0].Value))
				})
			})

			When("all rules are satisfied", func() {
				It("should return the string value specifically from the first rule", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										RawField:  "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Field:     SubscriptionID,
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										RawField:  "tenantId",
										RawValues: []string{"tenantId"},
										Field:     TenantID,
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value1",
							},
							{
								Matchers: []*Matcher{
									{
										RawField:  "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Field:     SubscriptionID,
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										RawField:  "tenantId",
										RawValues: []string{"tenantId"},
										Field:     TenantID,
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value2",
							},
						},
					}
					overrides.Overrides["o1"] = o
					str := overrides.getString("o1", e)
					Expect(str).To(Equal(o.Rules[0].Value))
				})
			})
		})

		Context("getMap tests", func() {
			When("the specified override is not found", func() {
				It("return a nil map", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										RawField:  "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Field:     SubscriptionID,
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										RawField:  "tenantId",
										RawValues: []string{"tenantId"},
										Field:     TenantID,
										Values:    ValueSet{"tenantId": true},
									},
								},
								MapValue: map[string]string{"key": "value"},
							},
						},
					}
					overrides.Overrides["o1"] = o
					m := overrides.getMap("o2", e)
					Expect(m).To(BeNil())
				})

				When("no rules are satisifed", func() {
					It("should return a nil map", func() {
						overrides := NewOverrides()
						o := &Override{
							Rules: []*Rule{
								{
									Matchers: []*Matcher{
										{
											RawField:  "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Field:     SubscriptionID,
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											RawField:  "tenantId",
											RawValues: []string{"tenantId"},
											Field:     TenantID,
											Values:    ValueSet{"tenantId": true},
										},
									},
									MapValue: map[string]string{"key": "value"},
								},
							},
						}
						overrides.Overrides["o1"] = o
						e.SubscriptionID = "someOtherSubscription"
						e.TenantID = "someOtherTenant"
						m := overrides.getMap("o1", e)
						Expect(m).To(BeNil())
					})
				})

				When("the singular rule is satisfied", func() {
					It("should return the map value corresponding to the rule", func() {
						overrides := NewOverrides()
						o := &Override{
							Rules: []*Rule{
								{
									Matchers: []*Matcher{
										{
											RawField:  "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Field:     SubscriptionID,
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											RawField:  "tenantId",
											RawValues: []string{"tenantId"},
											Field:     TenantID,
											Values:    ValueSet{"tenantId": true},
										},
									},
									MapValue: map[string]string{"key": "value"},
								},
							},
						}
						overrides.Overrides["o1"] = o
						m := overrides.getMap("o1", e)
						Expect(m).ToNot(BeNil())
						Expect(m).To(HaveKeyWithValue("key", "value"))
					})
				})

				When("all rules are satisfied", func() {
					It("should return the map value specifically from the first rule", func() {
						overrides := NewOverrides()
						o := &Override{
							Rules: []*Rule{
								{
									Matchers: []*Matcher{
										{
											RawField:  "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Field:     SubscriptionID,
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											RawField:  "tenantId",
											RawValues: []string{"tenantId"},
											Field:     TenantID,
											Values:    ValueSet{"tenantId": true},
										},
									},
									MapValue: map[string]string{"key": "value"},
								},
								{
									Matchers: []*Matcher{
										{
											RawField:  "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Field:     SubscriptionID,
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											RawField:  "tenantId",
											RawValues: []string{"tenantId"},
											Field:     TenantID,
											Values:    ValueSet{"tenantId": true},
										},
									},
									MapValue: map[string]string{"otherKey": "otherValue"},
								},
							},
						}
						overrides.Overrides["o1"] = o
						m := overrides.getMap("o1", e)
						Expect(m).ToNot(BeNil())
						Expect(m).To(HaveKeyWithValue("key", "value"))
						Expect(m).ToNot(HaveKeyWithValue("otherKey", "otherValue"))
					})
				})
			})
		})
	})
})
