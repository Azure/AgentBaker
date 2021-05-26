package e2e

import (
	"fmt"
	"testing"
	"os"
	"io/ioutil"
	"encoding/json"
)

func TestBasic(t *testing.T) {
	entry := "Generating CustomData and cseCmd"
	fmt.Println(entry)

	fields, err := os.Open("fields.json")
	if err != nil {
		fmt.Println(err)
	}

	defer fields.Close()
	fieldsByteValue, _ := ioutil.ReadAll(fields)

	var values map[string]interface{}
	json.Unmarshal([]byte(fieldsByteValue), &values)
}