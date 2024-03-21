package overrides

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("read tests", func() {
	Context("ReadFromDir", func() {
		When("dir is empty", func() {
			It("should return empty overrides", func() {
				dir, err := os.MkdirTemp("testdata", "*")
				Expect(err).To(BeNil())
				defer os.Remove(dir)
				overrides, err := ReadDir(dir)
				Expect(err).To(BeNil())
				Expect(overrides.Overrides).To(BeEmpty())
			})
		})

		When("dir does not exist", func() {
			It("should return an error", func() {
				overrides, err := ReadDir("testdata/does-not-exist")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("stat overrides location"))
			})
		})

		When("location is not a dir", func() {
			It("should return an error", func() {
				overrides, err := ReadDir("testdata/singular/the-single-override.yaml")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("overrides location is not a directory"))
			})
		})

		When("dir contains a single invalid yaml definition", func() {
			It("should return an error", func() {
				overrides, err := ReadDir("testdata/invalid-singular")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unmarshaling override yaml"))
				Expect(err.Error()).To(ContainSubstring("block sequence entries are not allowed in this context"))
			})
		})

		When("dir contains multiple invalid yaml definitions", func() {
			It("should return an error", func() {
				overrides, err := ReadDir("testdata/invalid-multi")
				Expect(overrides).To(BeNil())
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unmarshaling override yaml"))
				Expect(err.Error()).To(ContainSubstring("inferring override name from yaml file name: \"override.1.yaml\", override yaml name must be in the form of <name>.yaml"))
				Expect(err.Error()).To(ContainSubstring("block sequence entries are not allowed in this context"))
			})
		})

		When("dir contains a single valid yaml definition", func() {
			It("should read the singular override", func() {
				overrides, err := ReadDir("testdata/singular")
				Expect(err).To(BeNil())
				Expect(overrides.Overrides).ToNot(BeNil())
				Expect(overrides.Overrides).To(HaveKey("the-single-override"))
				e := NewEntity().WithFields(map[string]string{
					"subscriptionId": "sub2",
					"tenantId":       "t1",
				})
				str := overrides.getString("the-single-override", e)
				Expect(str).To(Equal("superCoolSub"))

				e.Fields["tenantId"] = "otherTenantId"
				str = overrides.getString("the-singular-override", e)
				Expect(str).To(BeEmpty())
			})
		})

		When("dir contains multiple valid yaml definitions", func() {
			It("should read all the overrides", func() {
				overrides, err := ReadDir("testdata/multi")
				Expect(err).To(BeNil())
				Expect(overrides.Overrides).To(HaveKey("override1"))
				Expect(overrides.Overrides).To(HaveKey("override2"))
				Expect(overrides.Overrides).To(HaveKey("override3"))

				e := NewEntity().WithFields(map[string]string{
					"subscriptionId": "sub3",
					"tenantId":       "tenantId",
				})
				str := overrides.getString("override1", e)
				Expect(str).To(Equal("superCoolSub"))

				e.Fields["subscriptionId"] = "sub1"
				e.Fields["tenantId"] = "t3"
				m := overrides.getMap("override2", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(HaveKeyWithValue("key1", "value1"))
				Expect(m).To(HaveKeyWithValue("key2", "value2"))
				Expect(m).To(HaveKeyWithValue("key3", "value3"))

				e.Fields["subscriptionId"] = "sub4"
				str = overrides.getString("override1", e)
				Expect(str).To(Equal("default"))

				e.Fields["tenantId"] = "tenantId"
				m = overrides.getMap("override2", e)
				Expect(m).To(BeEmpty())

				e.Fields["subscriptionId"] = "sub1"
				e.Fields["tenantId"] = "t1"
				m = overrides.getMap("override3", e)
				Expect(m).To(HaveKeyWithValue("key", "value1"))
				Expect(m).ToNot(HaveKeyWithValue("key", "value2"))

				e.Fields["subscriptionId"] = "sub2"
				e.Fields["tenantId"] = "t1"
				m = overrides.getMap("override3", e)
				Expect(m).To(HaveKeyWithValue("key", "value2"))
				Expect(m).ToNot(HaveKeyWithValue("key", "value1"))
			})
		})

		When("dir contains overrides only with default values", func() {
			It("should correctly read all the overrides", func() {
				overrides, err := ReadDir("testdata/defaults")
				Expect(err).To(BeNil())
				Expect(overrides.Overrides).To(HaveKey("map-override"))
				Expect(overrides.Overrides).To(HaveKey("string-override"))
				Expect(overrides.Overrides).To(HaveKey("empty-map-override"))
				Expect(overrides.Overrides).To(HaveKey("empty-string-override"))
				Expect(len(overrides.Overrides)).To(Equal(4))

				e := NewEntity().WithFields(map[string]string{
					"subscriptionId": "sub3",
					"tenantId":       "tenantId",
				})
				str := overrides.getString("string-override", e)
				Expect(str).To(Equal("default"))

				m := overrides.getMap("map-override", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(HaveKeyWithValue("key1", "value1"))
				Expect(m).To(HaveKeyWithValue("key2", "value2"))
				Expect(m).To(HaveKeyWithValue("key3", "value3"))

				str = overrides.getString("empty-string-override", e)
				Expect(str).To(BeEmpty())

				m = overrides.getMap("empty-map-override", e)
				Expect(m).ToNot(BeNil())
				Expect(m).To(BeEmpty())
			})
		})
	})
})
