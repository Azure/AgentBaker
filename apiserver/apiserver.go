package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/Azure/agentbaker/pkg/agent/toggles"
)

const (
	readHeaderTimeoutSeconds = 5

	maxConcurrentNodeBootstrapRequestsEnv = "MAX_CONCURRENT_NODE_BOOTSTRAP_REQUESTS"
	overloadRetryAfterSecondsEnv          = "OVERLOAD_RETRY_AFTER_SECONDS"
)

// OptionConfigurator is a function which can configure an Options object.
type OptionConfigurator func(opts *Options)

// Options holds the options for the api server.
type Options struct {
	Addr      string
	PProfAddr string
	Toggles   toggles.Toggles
}

func (o *Options) validate() error {
	if o == nil {
		return errors.New("serviceexample options can not be nil")
	}

	if o.Addr == "" {
		return errors.New("addr must not be empty")
	}
	if o.PProfAddr != "" && o.PProfAddr == o.Addr {
		return errors.New("pprof addr must differ from api addr")
	}
	return nil
}

// APIServer contains the connections details required to run the api.
type APIServer struct {
	Options                   *Options
	nodeBootstrapLimiter      chan struct{}
	overloadRetryAfterSeconds string
}

// NewAPIServer creates an APIServer object with defaults.
func NewAPIServer(o *Options) (*APIServer, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}

	maxConcurrentRequests, err := positiveIntFromEnv(
		maxConcurrentNodeBootstrapRequestsEnv,
		defaultMaxConcurrentNodeBootstrapRequests,
	)
	if err != nil {
		return nil, err
	}
	retryAfterSeconds, err := positiveIntFromEnv(overloadRetryAfterSecondsEnv, defaultOverloadRetryAfterSeconds)
	if err != nil {
		return nil, err
	}

	s := &APIServer{
		Options:                   o,
		nodeBootstrapLimiter:      make(chan struct{}, maxConcurrentRequests),
		overloadRetryAfterSeconds: strconv.Itoa(retryAfterSeconds),
	}

	return s, nil
}

func positiveIntFromEnv(name string, defaultValue int) (int, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

// ListenAndServe wraps http.Server and provides context-based cancelation.
func (api *APIServer) ListenAndServe(ctx context.Context) error {
	servers := []*http.Server{{
		Addr:              api.Options.Addr,
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: readHeaderTimeoutSeconds * time.Second,
	}}
	if api.Options.PProfAddr != "" {
		servers = append(servers, &http.Server{
			Addr:              api.Options.PProfAddr,
			Handler:           newPProfHandler(),
			ReadHeaderTimeout: readHeaderTimeoutSeconds * time.Second,
		})
		runtime.SetBlockProfileRate(1)
		previousMutexProfileFraction := runtime.SetMutexProfileFraction(1)
		defer func() {
			runtime.SetBlockProfileRate(0)
			runtime.SetMutexProfileFraction(previousMutexProfileFraction)
		}()
	}

	serverErrors := make(chan error, len(servers))
	for _, server := range servers {
		go func(server *http.Server) {
			serverErrors <- server.ListenAndServe()
		}(server)
		log.Printf("Starting APIServer at %s\n", server.Addr)
	}

	select {
	case <-ctx.Done():
		return shutdownServers(servers)
	case err := <-serverErrors:
		if shutdownErr := shutdownServers(servers); shutdownErr != nil {
			return fmt.Errorf("server failed: %w; shutdown failed: %w", err, shutdownErr)
		}
		return err
	}
}

func shutdownServers(servers []*http.Server) error {
	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, server := range servers {
		if err := server.Shutdown(shutdownContext); err != nil {
			return err
		}
	}
	return nil
}

func newPProfHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return mux
}
