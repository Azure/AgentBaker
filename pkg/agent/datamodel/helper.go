// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package datamodel

import (
	"bufio"
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

const (
	minSizeNamePartCount = 2
)

// ValidateDNSPrefix is a helper function to check that a DNS Prefix is valid.
func ValidateDNSPrefix(dnsName string) error {
	dnsNameRegex := `^([A-Za-z][A-Za-z0-9-]{1,43}[A-Za-z0-9])$`

	re, err := regexp.Compile(dnsNameRegex)
	if err != nil {
		return err
	}
	if !re.MatchString(dnsName) {
		return errors.Errorf("DNSPrefix '%s' is invalid. The DNSPrefix must contain between 3 and 45 characters"+
			" and can contain only letters, numbers, and hyphens.  It must start with a letter and must end with a"+
			" letter or a number. (length was %d)", dnsName, len(dnsName))
	}
	return nil
}

// IsSgxEnabledSKU determines if an VM SKU has SGX driver support.
func IsSgxEnabledSKU(vmSize string) bool {
	switch vmSize {
	case "Standard_DC2s", "Standard_DC4s":
		return true
	}
	return false
}

// GetStorageAccountType returns the support managed disk storage tier for a give VM size.
func GetStorageAccountType(sizeName string) (string, error) {
	spl := strings.Split(sizeName, "_")
	if len(spl) < minSizeNamePartCount {
		return "", errors.Errorf("Invalid sizeName: %s", sizeName)
	}
	capability := spl[1]
	if strings.Contains(strings.ToLower(capability), "s") {
		return "Premium_LRS", nil
	}
	return "Standard_LRS", nil
}

// GetOrderedEscapedKeyValsString returns an ordered string of escaped, quoted key=val.
func GetOrderedEscapedKeyValsString(config map[string]string) string {
	keys := []string{}
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		buf.WriteString(fmt.Sprintf("\"%s=%s\", ", key, config[key]))
	}
	return strings.TrimSuffix(buf.String(), ", ")
}

// SliceIntIsNonEmpty is a simple convenience to determine if a []int is non-empty.
func SliceIntIsNonEmpty(s []int) bool {
	return len(s) > 0
}

// WrapAsVerbatim formats a string for inserting a literal string into an ARM expression.
func WrapAsVerbatim(s string) string {
	return fmt.Sprintf("',%s,'", s)
}

// IndentString pads each line of an original string with N spaces and returns the new value.
func IndentString(original string, spaces int) string {
	out := bytes.NewBuffer(nil)
	scanner := bufio.NewScanner(strings.NewReader(original))
	for scanner.Scan() {
		for i := 0; i < spaces; i++ {
			out.WriteString(" ")
		}
		out.WriteString(scanner.Text())
		out.WriteString("\n")
	}
	return out.String()
}

func getContainerImageNameFromURL(downloadURL string) (string, error) {
	// example URL "downloadURL": "mcr.microsoft.com/oss/kubernetes/autoscaler/addon-resizer:*",
	// getting the data between the last / and the last :
	parts := strings.Split(downloadURL, "/")
	if len(parts) == 0 || len(parts[len(parts)-1]) == 0 {
		return "", fmt.Errorf("container image component URL is not in the expected format: %s", downloadURL)
	}
	lastPart := parts[len(parts)-1]
	component := strings.TrimSuffix(lastPart, ":*")
	return component, nil
}

func getComponentNameFromURL(downloadURL string) (string, error) {
	// example URL "downloadURL": "https://acs-mirror.azureedge.net/cni-plugins/v*/binaries",
	url, err := url.Parse(downloadURL) // /cni-plugins/v*/binaries
	if err != nil {
		return "", fmt.Errorf("download file image URL is not in the expected format: %s", downloadURL)
	}
	urlSplit := strings.Split(url.Path, "/") // ["", cni-plugins, v*, binaries]
	componentIndx, minURLSplit := 1, 2
	if len(urlSplit) < minURLSplit {
		return "", fmt.Errorf("download file image URL is not in the expected format: %s", downloadURL)
	}
	componentName := urlSplit[componentIndx]
	if componentName == "" {
		return "", fmt.Errorf("component name is empty in the URL: %s", downloadURL)
	}
	return componentName, nil
}

// IsMIGNode check if the node should be partitioned.
func IsMIGNode(gpuInstanceProfile string) bool {
	return gpuInstanceProfile != ""
}
