package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	agent "github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// RoutePathNodeBootstrapping the route path to get node bootstrapping data.
	RoutePathNodeBootstrapData string = "/getnodebootstrapdata"
)

func handleError(err error) Result {
	log.Println(err.Error())
	return Result{"", err}
}

// GetNodeBootstrapConfig endpoint for getting node bootstrapping data.
func (api *APIServer) GetNodeBootstrapData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	processResult := make(chan Result)
	go func() {
		var config datamodel.NodeBootstrappingConfiguration

		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			handleError(err)
			return
		}

		agentBaker, err := agent.NewAgentBaker()
		if err != nil {
			processResult <- handleError(err)
			return
		}
		nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(ctx, &config)
		if err != nil {
			processResult <- handleError(err)
			return
		}
		result, err := json.Marshal(nodeBootStrapping)
		if err != nil {
			processResult <- handleError(err)
			return
		}
		processResult <- Result{string(result), nil}
	}()

	select {
	case <-ctx.Done():
		http.Error(w, "process timeout", http.StatusInternalServerError)
	case result := <-processResult:
		if result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, result.Message)
		}

	}
}

type Result struct {
	Message string
	Error   error
}
