package apiserver

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/handlers"
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

	router.
		Methods("POST").
		Path(RoutePathNodeBootstrapData).
		Name("GetNodeBootstrapData").
		HandlerFunc(api.GetNodeBootstrapData)

	router.
		Methods("POST").
		Path(RoutePathLatestSIGImageConfig).
		Name("GetLatestSigImageConfig").
		HandlerFunc(api.GetLatestSigImageConfig)

	router.
		Methods("POST").
		Path(RoutePathDistroSIGImageConfig).
		Name("GetDistroSigImageConfig").
		HandlerFunc(api.GetDistroSigImageConfig)

	router.Methods("GET").Path("/healthz").Name("healthz").HandlerFunc(healthz)

	// global timeout and panic handlers.
	router.Use(timeoutHandler(), recoveryHandler())

	return router
}

func healthz(w http.ResponseWriter, r *http.Request) {
	handleOK(w, r)
}

func handleOK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func recoveryHandler() mux.MiddlewareFunc {
	return handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))
}

func timeoutHandler() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.TimeoutHandler(h, time.Second*30, "")
	}
}
