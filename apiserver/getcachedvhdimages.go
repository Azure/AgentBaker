package apiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Azure/agentbaker/pkg/agent"
)

const (
	// RoutePathGetCachedVersionsOnVHD the route path to get cached vhd images.
	RoutePathGetCachedVersionsOnVHD string = "/getcachedversionsonvhd"
)

// GetCachedVersionsOnVHD endpoint for getting the current versions of components cached on the vhd.
func (api *APIServer) GetCachedVersionsOnVHD(w http.ResponseWriter, r *http.Request) {
	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cachedOnVHD := agentBaker.GetCachedVersionsOnVHD()

	jsonResponse, err := json.Marshal(cachedOnVHD)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(jsonResponse))
}
