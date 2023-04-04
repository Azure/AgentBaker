package apiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	agent "github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// RoutePathNodeBootstrapData the route path to get node bootstrapping data.
	RoutePathNodeBootstrapData string = "/getnodebootstrapdata"
)

// GetNodeBootstrapConfig endpoint for getting node bootstrapping data.
func (api *APIServer) GetNodeBootstrapData(w http.ResponseWriter, r *http.Request) {
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
	nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(&config)
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
