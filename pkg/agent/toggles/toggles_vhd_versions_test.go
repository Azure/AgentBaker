package toggles

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetLinuxNodeImageVersion tests", func() {
	var (
		tgls *Toggles
		e    = &Entity{Fields: map[string]string{
			"subscriptionId": "sid",
			"tenantId":       "tid",
			"region":         "region-with-vhd",
		}}
	)

	BeforeEach(func() {
		tgls = &Toggles{
			Maps: map[string]MapToggle{
				"linux-node-image-version": func(entity *Entity) map[string]string {
					if entity.Fields["region"] == "region-with-vhd" {
						return map[string]string{
							"imageName": "imageVersion",
						}
					}
					return nil
				},
			},
		}
	})

	When("Region has an override", func() {
		It("returns the VHD for the region", func() {
			e.Fields["region"] = "region-with-vhd"
			versions := tgls.GetLinuxNodeImageVersion(e)
			Expect(versions).To(HaveKeyWithValue("imageName", "imageVersion"))
		})
	})

	When("Region has an override but there is no vhd for that region", func() {
		It("returns the VHD for the region", func() {
			e.Fields["region"] = "region-with-no-vhd"
			versions := tgls.GetLinuxNodeImageVersion(e)
			Expect(versions).To(BeNil())
		})
	})

	When("Region has no override", func() {
		It("returns nil", func() {
			e.Fields["region"] = "region-with-bad-vhd"
			versions := tgls.GetLinuxNodeImageVersion(e)
			Expect(versions).To(BeNil())
		})
	})
})
