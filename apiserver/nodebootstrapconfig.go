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
	RoutePathNodeBootstrapConfig string = "/nodebootstrapconfig"
)

func handleError(err error, w http.ResponseWriter) {
	log.Println(err.Error())
	http.Error(w, err.Error(), http.StatusBadRequest)
}

// GetNodeBootstrapConfig endpoint for getting node bootstrapping data.
func (api *APIServer) GetNodeBootstrappingConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	processResult := make(chan string)
	go func() {
		var config datamodel.NodeBootstrappingConfiguration

		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			handleError(err, w)
			return
		}

		agentBaker, err := agent.NewAgentBaker()
		if err != nil {
			handleError(err, w)
			return
		}
		nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(ctx, &config)
		if err != nil {
			handleError(err, w)
			return
		}
		result, err := json.Marshal(nodeBootStrapping)
		if err != nil {
			handleError(err, w)
			return
		}
		processResult <- string(result)
	}()

	select {
	case <-ctx.Done():
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "process timeout"}`))
	case result := <-processResult:
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(result))
	}
}
