package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func main() {
	int := make(chan os.Signal, 1)
	done := make(chan error)

	signal.Notify(int, os.Interrupt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)

	go func() { <-int; cancel() }()
	go func() { done <- run(os.Args[1]) }()

	select {
	case <-ctx.Done():
		log.Fatalln(fmt.Errorf("timed out generating node bootstrapping payload. %w", ctx.Err()))
	case err := <-done:
		if err != nil {
			log.Fatalln(fmt.Errorf("failed to generate node bootstrapping payload. %w", err))
		}
	}
	log.Println("OK")
}

func run(in string) (reterr error) {
	var config datamodel.NodeBootstrappingConfiguration

	dec := json.NewDecoder(bytes.NewBufferString(in))
	if err := dec.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode json. %w", err)
	}

	gen := agent.InitializeTemplateGenerator()
	csePayload := gen.GetNodeBootstrappingPayload(&config)
	cseCmd := gen.GetNodeBootstrappingCmd(&config)

	if err := ioutil.WriteFile("/opt/azure/containers/cse_payload.txt", []byte(csePayload), 0644); err != nil {
		return fmt.Errorf("failed to write cse payload. %w", err)
	}
	if err := ioutil.WriteFile("/opt/azure/containers/cse_regen.sh", []byte(cseCmd), 0544); err != nil {
		return fmt.Errorf("failed to write cse payload. %w", err)
	}

	return nil
}
