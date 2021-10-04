package apiserver

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"
)

// Route the route specifics
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// Routes list of routes to be added to the server
type Routes []Route

// NewRouter returns a new router with defaults
func (api *APIServer) NewRouter(ctx context.Context) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)

	// healthz should not require authentication
	router.
		Methods("POST").
		Path(RoutePathNodeBootstrapData).
		Name("GetNodeBootstrapData").
		HandlerFunc(api.GetNodeBootstrapData)

	return router
}
