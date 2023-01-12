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
	// RoutePathDistroSIGImageConfig the route path to get node bootstrapping data.
	RoutePathDistroSIGImageConfig string = "/getdistrosigimageconfig"
)

// GetDistroSigImageConfig endpoint for sig config for all distros in one shot.
func (api *APIServer) GetDistroSigImageConfig(w http.ResponseWriter, r *http.Request) {
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

	allDistros, err := agentBaker.GetDistroSigImageConfig(config.SIGConfig, config.Region)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := json.Marshal(allDistros)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
