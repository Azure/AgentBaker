package apiserver

import (
	"fmt"
	"net/http"
	"runtime/debug"
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
func (api *APIServer) NewRouter() http.Handler {
	router := http.NewServeMux()
	// Each API route gets its own request trace with its name; unmatched requests get none.
	route := func(method, path, name string, h http.HandlerFunc) {
		router.Handle(method+" "+path, requestLogging(api.Options.RequestLogger, name, timeoutHandler(recoveryHandler(h))))
	}
	route(http.MethodPost, RoutePathNodeBootstrapData, "GetNodeBootstrapData", api.GetNodeBootstrapData)
	route(http.MethodPost, RoutePathLatestSIGImageConfig, "GetLatestSigImageConfig", api.GetLatestSigImageConfig)
	route(http.MethodPost, RoutePathDistroSIGImageConfig, "GetDistroSigImageConfig", api.GetDistroSigImageConfig)

	router.Handle("GET /healthz", timeoutHandler(recoveryHandler(http.HandlerFunc(healthz))))

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
	return router
}

func healthz(w http.ResponseWriter, r *http.Request) {
	handleOK(w, r)
}

func handleOK(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

// recoveryHandler logs panics with the request IDs, so that a stack trace can be joined to its
// request trace.
func recoveryHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				requestLogger(r).Error("recovered panic", "panic", fmt.Sprint(err), "stack", string(debug.Stack()))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func timeoutHandler(h http.Handler) http.Handler {
	return http.TimeoutHandler(h, defaultTimeout, "")
}
