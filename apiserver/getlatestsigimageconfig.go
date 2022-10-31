package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
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

	processResult := make(chan Result)
	go func() {
		var config datamodel.GetLatestSigImageConfigRequest

		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			handleError(err)
			return
		}

		agentBaker, err := agent.NewAgentBaker()
		if err != nil {
			processResult <- handleError(err)
			return
		}

		latestSigConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, config.Region, config.Distro)
		if err != nil {
			processResult <- handleError(err)
			return
		}

		result, err := json.Marshal(latestSigConfig)
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
