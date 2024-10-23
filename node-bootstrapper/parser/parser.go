package parser

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/Azure/agentbaker/node-bootstrapper/utils"
	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

var (
	//go:embed templates/cse_cmd.sh.gtpl
	bootstrapTrigger         string
	bootstrapTriggerTemplate = template.Must(template.New("triggerBootstrapScript").Funcs(getFuncMap()).Parse(bootstrapTrigger)) //nolint:gochecknoglobals

)

func executeBootstrapTemplate(inputContract *nbcontractv1.Configuration) (string, error) {
	var buffer bytes.Buffer
	if err := bootstrapTriggerTemplate.Execute(&buffer, inputContract); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

// this function will eventually take a pointer to the bootstrap contract struct.
// it will then template out the variables into the final bootstrap trigger script.
func Parse(inputJSON []byte) (utils.SensitiveString, error) {
	// Parse the JSON into a nbcontractv1.Configuration struct
	var nbc nbcontractv1.Configuration
	err := json.Unmarshal(inputJSON, &nbc)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal the json to nbcontractv1: %w", err)
	}

	if nbc.Version != "v0" {
		return "", fmt.Errorf("unsupported version: %s", nbc.Version)
	}

	triggerBootstrapScript, err := executeBootstrapTemplate(&nbc)
	if err != nil {
		return "", fmt.Errorf("failed to execute the template: %w", err)
	}

	// Convert to one-liner
	return utils.SensitiveString(strings.ReplaceAll(triggerBootstrapScript, "\n", " ")), nil
}
