// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRecordCustomDataSize_WithinLimit(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	data := strings.Repeat("A", CustomDataMaxBase64Bytes)
	recordCustomDataSize(data, "linux", "ubuntu2204", customDataContext{})

	assert.Empty(t, buf.String(), "no log should be emitted at or below the limit")
}

func TestRecordCustomDataSize_ExceedsLimit(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(nil)

	data := strings.Repeat("A", CustomDataMaxBase64Bytes+1)
	recordCustomDataSize(data, "linux", "ubuntu2204", customDataContext{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-prod",
		VMSSName:       "vmss-pool1",
		PoolName:       "pool1",
	})

	output := buf.String()
	assert.Contains(t, output, "ERROR")
	assert.Contains(t, output, "exceeds VMSS limit")
	assert.Contains(t, output, "subscription=sub-123")
	assert.Contains(t, output, "vmss=vmss-pool1")
}

func TestCustomDataMaxBase64Bytes(t *testing.T) {
	assert.Equal(t, 87380, CustomDataMaxBase64Bytes)
}
