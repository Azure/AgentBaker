package apiserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// RoutePathGetCachedVersionsOnVHD the route path to get cached vhd images.
	RoutePathGetCachedVersionsOnVHD string = "/getcachedversionsonvhd"
)

type CachedOnVHD struct {
	cachedFromManifest   map[string]datamodel.ProcessedManifest `json:"cached_from_manifest"`
	cachedFromComponents map[string]datamodel.ContainerImage    `json:"cached_from_components"`
}

// GetCachedVersionsOnVHD endpoint for getting the current versions of components cached on the vhd.
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
		cachedFromManifest:   cachedFromManifest,
		cachedFromComponents: cachedFromComponents,
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
