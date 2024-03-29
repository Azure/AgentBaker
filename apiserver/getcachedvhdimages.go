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
	// RoutePathGetCachedVHDImages the route path to get cached vhd images.
	RoutePathGetCachedVHDImages string = "/getcachedvhdimages"
)

// GetCachedVHDImages endpoint for getting cached VHD images.
func (api *APIServer) GetCachedVHDImages(w http.ResponseWriter, r *http.Request) {
	var config datamodel.GetLatestSigImageConfigRequest
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

	images, err := agentBaker.GetCachedVHDImages()
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
