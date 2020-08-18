// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package helpers

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"math/rand"
	"strings"
	"testing"

	"github.com/Azure/aks-engine/pkg/i18n"
)

func TestCreateSSH(t *testing.T) {
	rg := rand.New(rand.NewSource(42))

	translator := &i18n.Translator{
		Locale: nil,
	}

	privateKey, publicKey, err := CreateSSH(rg, translator)
	if err != nil {
		t.Fatalf("failed to generate SSH: %s", err)
	}
	pemBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}
	pemBuffer := bytes.Buffer{}
	pem.Encode(&pemBuffer, pemBlock)

	if !strings.HasPrefix(pemBuffer.String(), "-----BEGIN RSA PRIVATE KEY-----") {
		t.Fatalf("Private Key did not start with expected header")
	}

	if privateKey.N.BitLen() != SSHKeySize {
		t.Fatalf("Private Key was of length %d but %d was expected", privateKey.N.BitLen(), SSHKeySize)
	}

	if err := privateKey.Validate(); err != nil {
		t.Fatalf("Private Key failed validation: %v", err)
	}

	if !strings.HasPrefix(publicKey, "ssh-rsa ") {
		t.Fatalf("Public Key did not start with expected header")
	}
}

type ContainerService struct {
	ID       string `json:"id"`
	Location string `json:"location"`
	Name     string `json:"name"`
}

func TestJSONMarshal(t *testing.T) {
	input := &ContainerService{}
	result, _ := JSONMarshal(input, false)
	expected := "{\"id\":\"\",\"location\":\"\",\"name\":\"\"}\n"
	if string(result) != expected {
		t.Fatalf("JSONMarshal returned unexpected result: expected %s but got %s", expected, string(result))
	}
	result, _ = JSONMarshalIndent(input, "", "", false)
	expected = "{\n\"id\": \"\",\n\"location\": \"\",\n\"name\": \"\"\n}\n"
	if string(result) != expected {
		t.Fatalf("JSONMarshal returned unexpected result: expected \n%sbut got \n%s", expected, result)
	}
}

func TestGetCloudTargetEnv(t *testing.T) {
	testcases := []struct {
		location string
		expected string
	}{
		{
			"chinaeast",
			"AzureChinaCloud",
		},
		{
			"chinanorth",
			"AzureChinaCloud",
		},
		{
			"chinaeast",
			"AzureChinaCloud",
		},
		{
			"chinaeast2",
			"AzureChinaCloud",
		},
		{
			"chinanorth2",
			"AzureChinaCloud",
		},
		{
			"germanycentral",
			"AzureGermanCloud",
		},
		{
			"germanynortheast",
			"AzureGermanCloud",
		},
		{
			"usgov123",
			"AzureUSGovernmentCloud",
		},
		{
			"usdod-123",
			"AzureUSGovernmentCloud",
		},
		{
			"sampleinput",
			"AzurePublicCloud",
		},
	}

	for _, testcase := range testcases {
		actual := GetCloudTargetEnv(testcase.location)
		if testcase.expected != actual {
			t.Errorf("expected GetCloudTargetEnv to return %s, but got %s", testcase.expected, actual)
		}
	}
}
func TestGetTargetEnv(t *testing.T) {
	testcases := []struct {
		location   string
		clouldName string
		expected   string
	}{
		{
			"chinaeast",
			"",
			"AzureChinaCloud",
		},
		{
			"chinanorth",
			"",
			"AzureChinaCloud",
		},
		{
			"chinaeast",
			"",
			"AzureChinaCloud",
		},
		{
			"chinaeast2",
			"",
			"AzureChinaCloud",
		},
		{
			"chinanorth2",
			"",
			"AzureChinaCloud",
		},
		{
			"germanycentral",
			"",
			"AzureGermanCloud",
		},
		{
			"germanynortheast",
			"",
			"AzureGermanCloud",
		},
		{
			"usgov123",
			"",
			"AzureUSGovernmentCloud",
		},
		{
			"usdod-123",
			"",
			"AzureUSGovernmentCloud",
		},
		{
			"sampleinput",
			"",
			"AzurePublicCloud",
		},
		{
			"azurestacklocation",
			"azurestackcloud",
			"AzureStackCloud",
		},
		{
			"azurestacklocation",
			"AzureStackcloud",
			"AzureStackCloud",
		},
		{
			"azurestacklocation",
			"azurestacklocation",
			"AzurePublicCloud",
		},
		{
			"akscustomlocation",
			"akscustom",
			"akscustom",
		},
	}

	for _, testcase := range testcases {
		actual := GetTargetEnv(testcase.location, testcase.clouldName)
		if testcase.expected != actual {
			t.Errorf("expected GetCloudTargetEnv to return %s, but got %s", testcase.expected, actual)
		}
	}
}
