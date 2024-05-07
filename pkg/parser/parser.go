package parser

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"log"
	"strings"
	"text/template"

	nbcontractv1 "github.com/Azure/agentbaker/pkg/proto/nbcontract/v1"
)

var (
	//go:embed cse_cmd.sh.gtpl
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
func Parse(inputJSON []byte) (string, error) {
	// Parse the JSON into a nbcontractv1.Configuration struct
	var nbc nbcontractv1.Configuration
	err := json.Unmarshal(inputJSON, &nbc)
	if err != nil {
		log.Printf("Failed to unmarshal the json to nbcontractv1: %v", err)
		return "", err
	}

	triggerBootstrapScript, err := executeBootstrapTemplate(&nbc)
	if err != nil {
		log.Printf("Failed to execute the template: %v", err)
		return "", err
	}

	log.Println("output env vars:")
	log.Println(triggerBootstrapScript)

	// Convert to one-liner
	return strings.ReplaceAll(triggerBootstrapScript, "\n", " "), nil
}
