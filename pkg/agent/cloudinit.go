// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT license.

package agent

import (
	"fmt"
	"strings"

	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"gopkg.in/yaml.v3"
)

func getYAMLQuotedCustomScript(path string, config *datamodel.NodeBootstrappingConfiguration) string {
	content, err := quoteCloudConfigFileContent(getRenderedCustomScript(path, config))
	if err != nil {
		panic(fmt.Sprintf("BUG: encode cloud-config file %s: %v", path, err))
	}
	return content
}

// Quoting preserves whitespace and trailing newlines without indentation or
// nested gzip streams. YAML emits !!binary automatically for non-UTF-8 data.
func quoteCloudConfigFileContent(content string) (string, error) {
	node := yaml.Node{Kind: yaml.ScalarNode, Style: yaml.DoubleQuotedStyle, Value: content}
	encoded, err := yaml.Marshal(&node)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(encoded), "\n"), nil
}
