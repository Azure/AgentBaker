package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// RoutePathNodeBootstrapData the route path to get node bootstrapping data.
	RoutePathNodeBootstrapData string = "/getnodebootstrapdata"
	defaultTimeout                    = 30 * time.Second
)

// GetNodeBootstrapData endpoint for getting node bootstrapping data.
func (api *APIServer) GetNodeBootstrapData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var config datamodel.NodeBootstrappingConfiguration

	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if api.Options != nil && api.Options.Toggles != nil {
		agentBaker = agentBaker.WithToggles(api.Options.Toggles)
	}

	nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(ctx, &config)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := json.Marshal(nodeBootStrapping)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
