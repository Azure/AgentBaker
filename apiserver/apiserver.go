package apiserver

import (
	"context"
	"errors"
	"log"
	"net/http"
)

// Options holds the options for the api server
type Options struct {
	Addr string
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

// APIServer contains the connections details required to run the api
type APIServer struct {
	Options *Options
}

// NewAPIServer creates an APIServer object with defaults
func NewAPIServer(o *Options) (*APIServer, error) {
	if err := o.validate(); err != nil {
		return nil, err
	}

	s := &APIServer{
		Options: o,
	}

	return s, nil
}

// ListenAndServe wraps http.Server and provides context-based cancelation.
func (api *APIServer) ListenAndServe(ctx context.Context) error {
	svr := http.Server{
		Addr:    api.Options.Addr,
		Handler: api.NewRouter(ctx),
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
