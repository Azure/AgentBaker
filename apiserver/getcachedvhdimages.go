package apiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	agent "github.com/Azure/agentbaker/pkg/agent"
)

const (
	// RoutePathGetCachedComponentVersions the route path to get cached vhd images.
	RoutePathGetCachedComponentVersions string = "/getcachedcomponentversions"
)

// GetCachedComponentVersions endpoint for getting cached VHD images.
func (api *APIServer) GetCachedComponentVersions(w http.ResponseWriter, r *http.Request) {
	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cachedVersions := agentBaker.GetCachedComponentVersions()

	result, err := json.Marshal(cachedVersions)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
