package parser

import (
	"bytes"
	_ "embed"
	"log"
	"text/template"
)

var (
	//go:embed cse_cmd.sh.gtpl
	bootstrapTrigger         string
	bootstrapTriggerTemplate = template.Must(template.New("triggerBootstrapScript").Parse(bootstrapTrigger))
)

func executeBootstrapTemplate() (string, error) {
	var buffer bytes.Buffer
	if err := bootstrapTriggerTemplate.Execute(&buffer, nil); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

// this function will eventually take a pointer to the bootstrap contract struct.
// it will then emplate out the variables into the final bootstrap trigger script.
func Parse() {
	triggerBootstrapScript, err := executeBootstrapTemplate()
	if err != nil {
		log.Printf("Failed to execute the template: %v", err)
	}

	log.Println(triggerBootstrapScript)
}
