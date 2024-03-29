package toggles

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("toggles tests", func() {
	var (
		toggles *Toggles
		e       = &Entity{Fields: map[string]string{
			"subscriptionId": "sid",
			"tenantId":       "tid",
		}}
	)

	BeforeEach(func() {
		toggles = &Toggles{
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
		When("toggle does not exist", func() {
			It("should return the correct default value", func() {
				m := toggles.getMap("mt", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(BeEmpty())
			})
		})

		When("toggle exists", func() {
			It("should return the correct value", func() {
				m := toggles.getMap("mt1", e)
				Expect(len(m)).To(Equal(1))
				Expect(m).To(HaveKeyWithValue("key", "value"))

				m = toggles.getMap("mt2", e)
				Expect(len(m)).To(Equal(2))
				Expect(m).To(HaveKeyWithValue("otherKey", "otherValue"))
				Expect(m).To(HaveKeyWithValue("someOtherKey", "someOtherValue"))
			})
		})
	})

	Context("getString tests", func() {
		When("toggle does not exist", func() {
			It("should return the correct default value", func() {
				s := toggles.getString("st", e)
				Expect(s).To(BeEmpty())
			})
		})

		When("toggle exists", func() {
			It("should return the correct value", func() {
				s := toggles.getString("st1", e)
				Expect(s).To(Equal("value"))

				s = toggles.getString("st2", e)
				Expect(s).To(Equal("otherValue"))
			})
		})
	})
})
