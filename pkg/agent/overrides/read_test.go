package overrides

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("read tests", func() {
	Context("ReadFromDir", func() {
		When("dir is empty", func() {
			It("should return nil overrides", func() {
				overrides, err := ReadFromDir("testdata/empty")
				Expect(err).To(BeNil())
				Expect(overrides).To(BeNil())
			})
		})

		When("dir does not exist", func() {
			It("should return an error", func() {
				overrides, err := ReadFromDir("testdata/does-not-exist")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("stat overrides location"))
			})
		})

		When("location is not a dir", func() {
			It("should return an error", func() {
				overrides, err := ReadFromDir("testdata/singular/the-single-override.yaml")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("overrides location is not a directory"))
			})
		})

		When("dir contains a single invalid yaml definition", func() {
			It("should return an error", func() {
				overrides, err := ReadFromDir("testdata/invalid-singular")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unmarshaling override yaml"))
				Expect(err.Error()).To(ContainSubstring("unrecognized Entity field for agentbakersvc override matcher: \"ResourceGroupName\""))
			})
		})

		When("dir contains multiple invalid yaml definitions", func() {
			It("should return an error", func() {
				overrides, err := ReadFromDir("testdata/invalid-multi")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("inferring override name from yaml file name: \"override.1.yaml\", override yaml name must be in the form of <name>.yaml"))
				Expect(err.Error()).To(ContainSubstring("unmarshaling override yaml"))
				Expect(err.Error()).To(ContainSubstring("unrecognized Entity field for agentbakersvc override matcher: \"ResourceGroupName\""))
			})
		})

		When("dir contains a single valid yaml definition", func() {
			It("should read the singular override", func() {
				overrides, err := ReadFromDir("testdata/singular")
				Expect(err).To(BeNil())
				Expect(overrides.Overrides).ToNot(BeNil())
				Expect(overrides.Overrides).To(HaveKey("the-single-override"))

				e := &Entity{
					SubscriptionID: "sub2",
					TenantID:       "t1",
				}
				str := overrides.getString("the-single-override", e)
				Expect(str).To(Equal("superCoolSub"))

				e.TenantID = "otherTenantId"
				str = overrides.getString("the-singular-override", e)
				Expect(str).To(BeEmpty())
			})
		})

		When("dir contains multiple valid yaml definitions", func() {
			It("should read all the overrides", func() {
				overrides, err := ReadFromDir("testdata/multi")
				Expect(err).To(BeNil())
				Expect(overrides.Overrides).To(HaveKey("override1"))
				Expect(overrides.Overrides).To(HaveKey("override2"))
				Expect(overrides.Overrides).To(HaveKey("override3"))

				e := &Entity{
					SubscriptionID: "sub3",
					TenantID:       "tenantId",
				}
				str := overrides.getString("override1", e)
				Expect(str).To(Equal("superCoolSub"))

				e.SubscriptionID = "sub1"
				e.TenantID = "t3"
				m := overrides.getMap("override2", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(HaveKeyWithValue("key1", "value1"))
				Expect(m).To(HaveKeyWithValue("key2", "value2"))
				Expect(m).To(HaveKeyWithValue("key3", "value3"))

				e.SubscriptionID = "sub4"
				str = overrides.getString("override1", e)
				Expect(str).To(BeEmpty())

				e.TenantID = "tenantId"
				m = overrides.getMap("override2", e)
				Expect(m).To(BeNil())

				e.SubscriptionID = "sub1"
				e.TenantID = "t1"
				m = overrides.getMap("override3", e)
				Expect(m).To(HaveKeyWithValue("key", "value1"))
				Expect(m).ToNot(HaveKeyWithValue("key", "value2"))

				e.SubscriptionID = "sub2"
				e.TenantID = "t1"
				m = overrides.getMap("override3", e)
				Expect(m).To(HaveKeyWithValue("key", "value2"))
				Expect(m).ToNot(HaveKeyWithValue("key", "value1"))
			})
		})
	})
})
