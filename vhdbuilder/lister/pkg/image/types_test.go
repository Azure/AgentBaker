package image

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImage(t *testing.T) {
	cases := []struct {
		name       string
		testFunc   func(img *Image) error
		expectFunc func(img *Image) error
		err        error
	}{
		{
			name: "should return an error when the same image tries to be set with mutliple IDs",
			testFunc: func(img *Image) error {
				if err := img.SetID("id"); err != nil {
					return err
				}
				if err := img.SetID("id"); err != nil {
					return err
				}
				if err := img.SetID("otherId"); err != nil {
					return err
				}
				return nil
			},
			err: fmt.Errorf("found multiple IDs for the same container image: id and otherId"),
		},
		{
			name: "should return no error and properly set the ID if an image gets the same ID multiple times",
			testFunc: func(img *Image) error {
				if err := img.SetID("id"); err != nil {
					return err
				}
				if err := img.SetID("id"); err != nil {
					return err
				}
				if err := img.SetID("id"); err != nil {
					return err
				}
				return nil
			},
			expectFunc: func(img *Image) error {
				if img.ID != "id" {
					return fmt.Errorf("expected image to have ID: %q, but was: %q", "id", img.ID)
				}
				return nil
			},
		},
		{
			name: "should return an error if an image tries to be set with multple byte sizes",
			testFunc: func(img *Image) error {
				if err := img.SetByteSize(10); err != nil {
					return err
				}
				if err := img.SetByteSize(10); err != nil {
					return err
				}
				if err := img.SetByteSize(20); err != nil {
					return err
				}
				return nil
			},
			err: fmt.Errorf("found mismatching byte sizes for the same container image: (10, 20)"),
		},
		{
			name: "should return no error and properly set the byte size if an image gets the same byte size multiple times",
			testFunc: func(img *Image) error {
				if err := img.SetByteSize(10); err != nil {
					return err
				}
				if err := img.SetByteSize(10); err != nil {
					return err
				}
				return nil
			},
			expectFunc: func(img *Image) error {
				if img.Bytes != 10 {
					return fmt.Errorf("expected image to have byte size: 10, but was: %d", img.Bytes)
				}
				return nil
			},
		},
		{
			name: "should properly add a new digest",
			testFunc: func(img *Image) error {
				img.AddDigest("digest")
				return nil
			},
			expectFunc: func(img *Image) error {
				if _, ok := img.Digests["digest"]; !ok {
					return fmt.Errorf("expected image to have digest %q, but did not", "digest")
				}
				return nil
			},
		},
		{
			name: "should properly add a new tag",
			testFunc: func(img *Image) error {
				img.AddTag("tag")
				return nil
			},
			expectFunc: func(img *Image) error {
				if _, ok := img.Tags["tag"]; !ok {
					return fmt.Errorf("expected image to have tag %q, but did not", "tag")
				}
				return nil
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			img := New()
			if c.testFunc != nil {
				err := c.testFunc(img)
				if c.err != nil {
					assert.EqualError(t, err, c.err.Error())
				} else {
					assert.NoError(t, err)
				}
			}
			if c.expectFunc != nil {
				err := c.expectFunc(img)
				assert.NoError(t, err)
			}
		})
	}
}

func TestImageMarshal(t *testing.T) {
	cases := []struct {
		name     string
		img      *Image
		expected []byte
	}{
		{
			name: "should correctly marshal to JSON",
			img: &Image{
				ID:    "id",
				Bytes: 100000,
				Tags: map[string]struct{}{
					"t1": struct{}{},
					"t2": struct{}{},
					"t3": struct{}{},
				},
				Digests: map[string]struct{}{
					"d1": struct{}{},
				},
			},
			expected: []byte(`{"id":"id","bytes":100000,"size":"97.7 KiB","repoTags":["t1","t2","t3"],"repoDigests":["d1"]}`),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := json.Marshal(c.img)
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}
