package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
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
	Addr    string
	Toggles toggles.Toggles
}

func (o *Options) validate() error {
	if o == nil {
		return errors.New("serviceexample options can not be nil")
	}

	if o.Addr == "" {
		return errors.New("addr must not be empty")
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
	svr := http.Server{
		Addr:              api.Options.Addr,
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: readHeaderTimeoutSeconds * time.Second,
	}

	errors := make(chan error)
	go func() {
		errors <- svr.ListenAndServe()
	}()

	log.Printf("Starting APIServer at %s\n", api.Options.Addr)
	select {
	case <-ctx.Done():
		return svr.Shutdown(context.Background())
	case err := <-errors:
		return err
	}
}
