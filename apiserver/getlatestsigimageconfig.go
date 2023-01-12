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
	// RoutePathLatestSIGImageConfig the route path to get node bootstrapping data.
	RoutePathLatestSIGImageConfig string = "/getlatestsigimageconfig"
)

// GetLatestSigImageConfig endpoint for getting latest sig image reference.
func (api *APIServer) GetLatestSigImageConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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

	latestSigConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, config.Region, config.Distro)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := json.Marshal(latestSigConfig)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
