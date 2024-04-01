package apiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	agent "github.com/Azure/agentbaker/pkg/agent"
)

const (
	// RoutePathGetCachedK8sVersions the route path to get cached vhd images.
	RoutePathGetCachedK8sVersions string = "/getcachedk8sversions"
)

// GetCachedK8sVersions endpoint for getting cached VHD images.
func (api *APIServer) GetCachedK8sVersions(w http.ResponseWriter, r *http.Request) {
	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	images, err := agentBaker.GetCachedK8sVersions()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := json.Marshal(images)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
