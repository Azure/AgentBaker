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
	// RoutePathGetCachedVersionsOnVHD the route path to get cached vhd images.
	RoutePathGetCachedVersionsOnVHD string = "/getcachedversionsonvhd"
)

type CachedOnVHD struct {
	CachedFromManifest   map[string]datamodel.ProcessedManifest
	CachedFromComponents map[string]datamodel.ProcessedComponents
}

// GetCachedVersionsOnVHD endpoint for getting cached VHD versions.
func (api *APIServer) GetCachedVersionsOnVHD(w http.ResponseWriter, r *http.Request) {
	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cachedFromManifest, cachedFromComponents, err := agentBaker.GetCachedVersionsOnVHD()
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
