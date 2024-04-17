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
	// RoutePathGetCachedComponentVersions the route path to get cached vhd images.
	RoutePathGetCachedComponentVersions string = "/getcachedcomponentversions"
)

type CachedOnVHD struct {
	CachedFromManifest   map[string]datamodel.ProcessedManifest
	CachedFromComponents map[string]datamodel.ProcessedComponents
}

// GetCachedComponentVersions endpoint for getting cached VHD images.
func (api *APIServer) GetCachedComponentVersions(w http.ResponseWriter, r *http.Request) {
	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cachedFromManifest, cachedFromComponents, err := agentBaker.GetCachedComponentVersions()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := CachedOnVHD{
		CachedFromManifest:   cachedFromManifest,
		CachedFromComponents: cachedFromComponents,
	}

	jsonResponse, err := json.Marshal(result)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(jsonResponse))
}
