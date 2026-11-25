package apiserver

import (
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// Log sources and field names follow the AKS Kusto schema. The AKS log collector routes
// JSON stdout lines to tables by the "source" field.
const (
	// IncomingRequestTraceSource is the source for incoming request traces.
	IncomingRequestTraceSource = "IncomingRequestTraceLog"
	// ContextlessTraceSource is the source for logs that are not tied to a request.
	ContextlessTraceSource = "AcsContextlessTraceLog"

	operationIDHeader     = "x-ms-acs-operation-id"
	correlationIDHeader   = "x-ms-correlation-request-id"
	clientRequestIDHeader = "x-ms-client-request-id"

	healthzRouteName = "healthz"
)

// NewJSONLogger returns a logger that writes JSON lines with the given source.
func NewJSONLogger(w io.Writer, source string) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil)).With("source", source)
}

func requestLogging(logger *slog.Logger) mux.MiddlewareFunc {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeName := ""
			if route := mux.CurrentRoute(r); route != nil {
				routeName = route.GetName()
			}
			if routeName == healthzRouteName {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			reqLogger := logger.With(requestIDAttrs(r)...).With(
				"httpMethod", r.Method,
				"targetURI", r.URL.EscapedPath(),
				"routeName", routeName,
				"operationName", routeName,
				"userAgent", r.UserAgent(),
				"clientRemoteAddr", r.RemoteAddr,
			)
			reqLogger.Info("AgentBakerHttpRequestStart")

			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			reqLogger.Info("AgentBakerHttpRequestEnd",
				"StatusCode", strconv.Itoa(rec.statusCode),
				"contentLength", strconv.Itoa(rec.contentLength),
				"durationInMilliseconds", strconv.FormatInt(time.Since(start).Milliseconds(), 10),
			)
		})
	}
}

// requestLogger returns the default logger with the request IDs, so that handler logs can be
// joined to the request trace.
func requestLogger(r *http.Request) *slog.Logger {
	return slog.Default().With(requestIDAttrs(r)...)
}

func requestIDAttrs(r *http.Request) []any {
	return []any{
		"operationID", r.Header.Get(operationIDHeader),
		"correlationID", r.Header.Get(correlationIDHeader),
		"clientRequestID", r.Header.Get(clientRequestIDHeader),
	}
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode    int
	contentLength int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	if r.statusCode == 0 {
		r.statusCode = statusCode
	}
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.contentLength += n
	return n, err
}
