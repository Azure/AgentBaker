// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package helpers

import (
	"testing"
)

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
