package apiserver

import (
	"context"
	"net/http"

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

	// healthz should not require authentication
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

	router.Methods("GET").Path("/panic").Name("PanicTester").HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("fudge") })
	router.Methods("GET").Path("/panic2").Name("PanicTester2").HandlerFunc(handlePanic)

	router.HandleFunc("/healthz", healthz)

	router.Use(handlers.RecoveryHandler(handlers.PrintRecoveryStack(true)))

	return router
}

func healthz(w http.ResponseWriter, r *http.Request) {
	handleOK(w, r)
}

func setHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func handleOK(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	w.WriteHeader(http.StatusOK)
}

func handlePanic(w http.ResponseWriter, r *http.Request) {
	setHeaders(w)
	w.WriteHeader(http.StatusOK)
	panic("fudge")
}
