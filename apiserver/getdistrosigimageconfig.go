package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	agent "github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
	"github.com/sanity-io/litter"
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

	processResult := make(chan Result)
	go func() {
		var config datamodel.GetLatestSigImageConfigRequest

		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			handleError(err)
			return
		}

		litter.Dump("request config")
		litter.Dump(config)

		agentBaker, err := agent.NewAgentBaker()
		if err != nil {
			processResult <- handleError(err)
			return
		}

		allDistros, err := agentBaker.GetDistroSigImageConfig(config.SIGConfig, config.Region)
		if err != nil {
			processResult <- handleError(err)
			return
		}

		litter.Dump("allDistros")
		litter.Dump(allDistros)

		result, err := json.Marshal(allDistros)
		if err != nil {
			processResult <- handleError(err)
			return
		}
		processResult <- Result{string(result), nil}
	}()

	select {
	case <-ctx.Done():
		http.Error(w, "process timeout", http.StatusInternalServerError)
	case result := <-processResult:
		if result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, result.Message)
		}

	}
}
