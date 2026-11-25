package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// RoutePathLatestSIGImageConfig the route path to get node bootstrapping data.
	RoutePathLatestSIGImageConfig string = "/getlatestsigimageconfig"
)

// GetLatestSigImageConfig endpoint for getting latest sig image reference.
func (api *APIServer) GetLatestSigImageConfig(w http.ResponseWriter, r *http.Request) {
	var config datamodel.GetLatestSigImageConfigRequest

	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		requestLogger(r).Error("decode request body", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	agentBaker, err := agent.NewAgentBaker()
	if err != nil {
		requestLogger(r).Error("create AgentBaker", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if api.Options != nil && api.Options.Toggles != nil {
		agentBaker = agentBaker.WithToggles(api.Options.Toggles)
	}

	latestSigConfig, err := agentBaker.GetLatestSigImageConfig(config.SIGConfig, config.Distro, &datamodel.EnvironmentInfo{
		SubscriptionID: config.SubscriptionID,
		TenantID:       config.TenantID,
		Region:         config.Region,
	})
	if err != nil {
		requestLogger(r).Error("get latest SIG image config", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := json.Marshal(latestSigConfig)
	if err != nil {
		requestLogger(r).Error("marshal response", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
