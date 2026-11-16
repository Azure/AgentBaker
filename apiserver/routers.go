package apiserver

import (
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// Route defines the route specifics.
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

// Routes list of routes to be added to the server.
type Routes []Route

// NewRouter returns a new router with defaults.
func (api *APIServer) NewRouter() *mux.Router {
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
	//
	// No authentication middleware (JWT/mTLS/API-key) is intentionally added here;
	// this is a risk-accepted By-Design decision, not an oversight.
	//
	// Rationale:
	//   - This APIServer is an internal sidecar invoked only by the AKS RP control
	//     plane over the cluster-internal network; it is not exposed to the public
	//     internet.
	//   - The enforced security boundary is network isolation (pod/cluster network
	//     policy), configured outside this repo in the RP deployment, not app-layer
	//     auth.
	//   - The handlers registered above are stateless template generators: they
	//     render node bootstrap data / image config from request input and do not
	//     read tenant secrets from a backend store.
	//   - Adding real authentication (mTLS or token-based) would require a
	//     cert/token issuance and rotation mechanism plus a coordinated change on
	//     the RP caller side (a different repo/system) - out of scope for a
	//     point fix here.
	//
	// If this router is ever exposed beyond the trusted RP-only network boundary,
	// authentication middleware must be added here.
	router.Use(timeoutHandler(), recoveryHandler())

	return router
}

func healthz(w http.ResponseWriter, r *http.Request) {
	handleOK(w, r)
}

func handleOK(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

func recoveryHandler() mux.MiddlewareFunc {
	return handlers.RecoveryHandler(handlers.PrintRecoveryStack(true))
}

func timeoutHandler() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.TimeoutHandler(h, defaultTimeout, "")
	}
}
