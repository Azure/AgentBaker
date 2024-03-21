package overrides

import (
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("types", func() {
	Context("entity", func() {
		When("constructed from raw fields", func() {
			It("should have the correct fields", func() {
				fields := map[string]string{
					"f1": "v1",
					"f2": "v2",
				}
				e := NewEntity().WithFields(fields)
				Expect(e).ToNot(BeNil())
				Expect(e.Fields).To(HaveKeyWithValue("f1", "v1"))
				Expect(e.Fields).To(HaveKeyWithValue("f2", "v2"))
				Expect(len(e.Fields)).To(Equal(2))
			})
		})

		When("constructed from a NodeBootstrappingConfiguration", func() {
			It("should have the correct fields", func() {
				nbc := &datamodel.NodeBootstrappingConfiguration{
					SubscriptionID: "subscriptionId",
					TenantID:       "tenantId",
				}
				e := NewEntity().FromNodeBootstrappingConfiguration(nbc)
				Expect(e).ToNot(BeNil())
				Expect(e.Fields).To(HaveKeyWithValue(SubscriptionIDFieldName, nbc.SubscriptionID))
				Expect(e.Fields).To(HaveKeyWithValue(TenantIDFieldName, nbc.TenantID))
				Expect(len(e.Fields)).To(Equal(2))
			})
		})
	})
})

var _ = Describe("overrides", func() {
	var e *Entity

	BeforeEach(func() {
		e = NewEntity().WithFields(map[string]string{
			"subscriptionId": "subscriptionId",
			"tenantId":       "tenantId",
		})
	})

	Context("Matcher tests", func() {
		When("entity does not match", func() {
			It("Matches() should return false", func() {
				m := &Matcher{
					Field:     "subscriptionId",
					RawValues: []string{"subscription"},
					Values:    ValueSet{"subscription": true},
				}
				matches := m.Matches(e)
				Expect(matches).To(BeFalse())
			})
		})

		When("entity does match", func() {
			It("Matches() should return true", func() {
				m := &Matcher{
					RawValues: []string{"subscriptionId"},
					Field:     "subscriptionId",
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
							Field:     "subscriptionId",
							RawValues: []string{"subscription"},
							Values:    ValueSet{"subscription": true},
						},
						{
							Field:     "tenantId",
							RawValues: []string{"tenant"},
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
							Field:     "subscriptionId",
							RawValues: []string{"subscriptionId"},
							Values:    ValueSet{"subscriptionId": true},
						},
						{
							Field:     "tenantId",
							RawValues: []string{"tenant"},
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
							Field:     "subscriptionId",
							RawValues: []string{"subscriptionId"},
							Values:    ValueSet{"subscriptionId": true},
						},
						{
							Field:     "tenantId",
							RawValues: []string{"tenantId"},
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
										Field:     "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										Field:     "tenantId",
										RawValues: []string{"tenantId"},
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

			When("no rules are satisfied", func() {
				It("should return the default value", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										Field:     "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										Field:     "tenantId",
										RawValues: []string{"tenantId"},
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value",
							},
						},
						DefaultValue: "defaultValue",
					}
					overrides.Overrides["o1"] = o
					e.Fields["subscriptionId"] = "someOtherSubscription"
					e.Fields["tenantId"] = "someOtherTenant"
					str := overrides.getString("o1", e)
					Expect(str).To(Equal("defaultValue"))
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
										Field:     "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										Field:     "tenantId",
										RawValues: []string{"tenantId"},
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
										Field:     "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										Field:     "tenantId",
										RawValues: []string{"tenantId"},
										Values:    ValueSet{"tenantId": true},
									},
								},
								Value: "value1",
							},
							{
								Matchers: []*Matcher{
									{
										Field:     "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										Field:     "tenantId",
										RawValues: []string{"tenantId"},
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
				It("return an empty map", func() {
					overrides := NewOverrides()
					o := &Override{
						Rules: []*Rule{
							{
								Matchers: []*Matcher{
									{
										Field:     "subscriptionId",
										RawValues: []string{"subscriptionId"},
										Values:    ValueSet{"subscriptionId": true},
									},
									{
										Field:     "tenantId",
										RawValues: []string{"tenantId"},
										Values:    ValueSet{"tenantId": true},
									},
								},
								MapValue: map[string]string{"key": "value"},
							},
						},
					}
					overrides.Overrides["o1"] = o
					m := overrides.getMap("o2", e)
					Expect(m).To(BeEmpty())
				})

				When("no rules are satisifed", func() {
					It("should return the default map value", func() {
						overrides := NewOverrides()
						o := &Override{
							Rules: []*Rule{
								{
									Matchers: []*Matcher{
										{
											Field:     "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											Field:     "tenantId",
											RawValues: []string{"tenantId"},
											Values:    ValueSet{"tenantId": true},
										},
									},
									MapValue: map[string]string{"key": "value"},
								},
							},
							DefaultMapValue: map[string]string{"default": "value"},
						}
						overrides.Overrides["o1"] = o
						e.Fields["subscriptionId"] = "someOtherSubscription"
						e.Fields["tenantId"] = "someOtherTenant"
						m := overrides.getMap("o1", e)
						Expect(m).ToNot(BeNil())
						Expect(m).To(HaveKeyWithValue("default", "value"))
						Expect(len(m)).To(Equal(1))
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
											Field:     "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											Field:     "tenantId",
											RawValues: []string{"tenantId"},
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
											Field:     "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											Field:     "tenantId",
											RawValues: []string{"tenantId"},
											Values:    ValueSet{"tenantId": true},
										},
									},
									MapValue: map[string]string{"key": "value"},
								},
								{
									Matchers: []*Matcher{
										{
											Field:     "subscriptionId",
											RawValues: []string{"subscriptionId"},
											Values:    ValueSet{"subscriptionId": true},
										},
										{
											Field:     "tenantId",
											RawValues: []string{"tenantId"},
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
						Expect(len(m)).To(Equal(1))
					})
				})
			})
		})
	})
})
