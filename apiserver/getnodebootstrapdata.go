package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

const (
	// RoutePathNodeBootstrapData the route path to get node bootstrapping data.
	RoutePathNodeBootstrapData                string = "/getnodebootstrapdata"
	defaultTimeout                                   = 30 * time.Second
	defaultMaxConcurrentNodeBootstrapRequests        = 30
	defaultOverloadRetryAfterSeconds                 = 3
)

// GetNodeBootstrapData endpoint for getting node bootstrapping data.
func (api *APIServer) GetNodeBootstrapData(w http.ResponseWriter, r *http.Request) {
	select {
	case api.nodeBootstrapLimiter <- struct{}{}:
		defer func() {
			<-api.nodeBootstrapLimiter
		}()
	default:
		w.Header().Set("Retry-After", api.overloadRetryAfterSeconds)
		http.Error(w, "server overloaded", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	var config datamodel.NodeBootstrappingConfiguration

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

	if api.Options != nil && api.Options.Toggles != nil {
		agentBaker = agentBaker.WithToggles(api.Options.Toggles)
	}

	nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(ctx, &config)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := json.Marshal(nodeBootStrapping)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logCustomDataSize(ctx, api.customDataLogger, &config, len(nodeBootStrapping.CustomData))

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}

func logCustomDataSize(ctx context.Context, logger *slog.Logger, config *datamodel.NodeBootstrappingConfiguration, length int) {
	if length < agent.MaxCustomDataLength*9/10 {
		return
	}

	level := slog.LevelWarn
	if length > agent.MaxCustomDataLength {
		level = slog.LevelError
	}

	logger.Log(ctx, level, fmt.Sprintf(
		"CustomData subscriptionID=%q resourceGroup=%q pool=%q os=%q distro=%q Length is %d",
		config.SubscriptionID, config.ResourceGroupName, config.AgentPoolProfile.Name,
		config.AgentPoolProfile.OSType, config.AgentPoolProfile.Distro, length),
		"source", "AgentBakerCustomData")
}
