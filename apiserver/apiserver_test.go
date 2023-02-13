package apiserver_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Azure/agentbaker/apiserver"
)

func Test_RecoveryHandler(t *testing.T) {
	srv, err := apiserver.NewAPIServer(&apiserver.Options{Addr: ":8080", Timeout: time.Second * 5})
	router := srv.NewRouter(context.Background())

	ok(t, err)
	rr := execute("{}", "POST", "/getnodebootstrapdata", router, t)

	status := rr.Code

	equals(t, http.StatusInternalServerError, status)
}

func Test_TimeoutHandler(t *testing.T) {
	srv, err := apiserver.NewAPIServer(&apiserver.Options{Addr: ":8080", Timeout: time.Second * 0})
	router := srv.NewRouter(context.Background())
	done := make(chan struct{})

	router.Methods("GET").Path("/timeout").Name("timeout").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-done
		w.WriteHeader(http.StatusOK)
	})

	ok(t, err)
	rr := execute("", "GET", "/timeout", router, t)

	status := rr.Code
	equals(t, http.StatusServiceUnavailable, status)
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// execute assists generating HTTP requests for testing purposes.
func execute(body string, method string, endpoint string, h http.Handler, t *testing.T) *httptest.ResponseRecorder {
	// Read data, create a request manually, instantiate recording apparatus.
	data := strings.NewReader(body)
	req, err := http.NewRequest(method, endpoint, data)
	ok(t, err)
	rr := httptest.NewRecorder()

	// Create handler and process request
	h.ServeHTTP(rr, req)

	return rr
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}
