package parser

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"log"
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
func Parse() {
	inputJSON, err := json.Marshal(getBaseTemplate())
	if err != nil {
		log.Printf("Failed to marshal the nbcontractv1 to json: %v", err)
	}

	log.Println("Input Json: ")
	log.Println(string(inputJSON))

	// inputJson above will be provided by bootstrappers. We are using getBaseTemplate() for dev/test purpose for now.
	// We can further move it to other file for unit tests later.

	inputContract := nbcontractv1.Configuration{}
	err = json.Unmarshal(inputJSON, &inputContract)
	if err != nil {
		log.Printf("Failed to unmarshal the json to nbcontractv1: %v", err)
	}

	triggerBootstrapScript, err := executeBootstrapTemplate(&inputContract)
	if err != nil {
		log.Printf("Failed to execute the template: %v", err)
	}

	log.Println("output env vars:")
	log.Println(triggerBootstrapScript)
}
