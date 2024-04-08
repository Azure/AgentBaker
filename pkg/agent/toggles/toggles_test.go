package toggles

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("tgls tests", func() {
	var (
		tgls *Toggles
		e    = &Entity{Fields: map[string]string{
			"subscriptionId": "sid",
			"tenantId":       "tid",
		}}
	)

	BeforeEach(func() {
		tgls = &Toggles{
			Maps: map[string]MapToggle{
				"mt1": func(entity *Entity) map[string]string {
					return map[string]string{"key": "value"}
				},
				"mt2": func(entity *Entity) map[string]string {
					return map[string]string{
						"otherKey":     "otherValue",
						"someOtherKey": "someOtherValue",
					}
				},
			},
			Strings: map[string]StringToggle{
				"st1": func(entity *Entity) string {
					return "value"
				},
				"st2": func(entity *Entity) string {
					return "otherValue"
				},
			},
		}
	})

	Context("getMap tests", func() {
		When("toggles are nil", func() {
			It("should return the empty default value", func() {
				tgls = nil
				m := tgls.getMap("mt", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(BeEmpty())
			})
		})

		When("map toggles are nil", func() {
			It("should return the empty default value", func() {
				tgls.Maps = nil
				m := tgls.getMap("mt", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(BeEmpty())
			})
		})

		When("toggle does not exist", func() {
			It("should return the correct default value", func() {
				m := tgls.getMap("mt", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(BeEmpty())
			})
		})

		When("toggle exists", func() {
			It("should return the correct value", func() {
				m := tgls.getMap("mt1", e)
				Expect(len(m)).To(Equal(1))
				Expect(m).To(HaveKeyWithValue("key", "value"))

				m = tgls.getMap("mt2", e)
				Expect(len(m)).To(Equal(2))
				Expect(m).To(HaveKeyWithValue("otherKey", "otherValue"))
				Expect(m).To(HaveKeyWithValue("someOtherKey", "someOtherValue"))
			})
		})
	})

	Context("getString tests", func() {
		When("toggles are nil", func() {
			It("should return the empty default value", func() {
				s := tgls.getString("st", e)
				Expect(s).To(BeEmpty())
			})
		})

		When("string toggles are nil", func() {
			It("should return the empty default value", func() {
				s := tgls.getString("st", e)
				Expect(s).To(BeEmpty())
			})
		})

		When("toggle does not exist", func() {
			It("should return the correct default value", func() {
				s := tgls.getString("st", e)
				Expect(s).To(BeEmpty())
			})
		})

		When("toggle exists", func() {
			It("should return the correct value", func() {
				s := tgls.getString("st1", e)
				Expect(s).To(Equal("value"))

				s = tgls.getString("st2", e)
				Expect(s).To(Equal("otherValue"))
			})
		})
	})
})
