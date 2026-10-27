// Package cloudfixture contains services used only by cloud initialization tests.
package cloudfixture

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"time"
)

const StatsPath = "/anc-integration/stats"

const ubuntuPrefix = "/ubuntu"
const microsoftPrefix = "/microsoft"
const repositoryHTTPScheme = "https"

type RepositoryStats struct {
	Keys     int `json:"keys"`
	Metadata int `json:"metadata"`
	Failures int `json:"failures"`
}

// RepositoryProxy preserves vendor signatures and package contents. In particular,
// it must not replace real repository metadata with a successful empty response.
type RepositoryProxy struct {
	ubuntu, microsoft *url.URL
	mu                sync.Mutex
	stats             RepositoryStats
	failMetadata      bool
}

func NewRepositoryProxy() *RepositoryProxy {
	ubuntu := &url.URL{Scheme: repositoryHTTPScheme, Host: "archive.ubuntu.com", Path: ubuntuPrefix}
	if runtime.GOARCH == "arm64" {
		ubuntu = &url.URL{Scheme: repositoryHTTPScheme, Host: "ports.ubuntu.com", Path: "/ubuntu-ports"}
	}
	return &RepositoryProxy{
		ubuntu:    ubuntu,
		microsoft: &url.URL{Scheme: repositoryHTTPScheme, Host: "packages.microsoft.com"},
	}
}

func (p *RepositoryProxy) Stats() RepositoryStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stats
}

func (p *RepositoryProxy) SetMetadataFailure(fail bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failMetadata = fail
}

func (p *RepositoryProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "only GET and HEAD are supported", http.StatusMethodNotAllowed)
		return
	}
	p.mu.Lock()
	fail := p.failMetadata && isRepositoryMetadata(r.URL.Path)
	p.mu.Unlock()
	if fail {
		p.record(r.URL.Path, http.StatusServiceUnavailable)
		http.Error(w, "injected repository metadata outage", http.StatusServiceUnavailable)
		return
	}
	if r.URL.Path == StatsPath {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(p.Stats()); err != nil {
			return // The caller has disconnected.
		}
		return
	}
	target, prefix := p.microsoft, ""
	switch {
	case strings.HasPrefix(r.URL.Path, ubuntuPrefix+"/"):
		target, prefix = p.ubuntu, ubuntuPrefix
	case strings.HasPrefix(r.URL.Path, microsoftPrefix+"/"):
		prefix = microsoftPrefix
	case strings.HasPrefix(r.URL.Path, "/keys/"), strings.HasPrefix(r.URL.Path, "/azurelinux/"):
	default:
		http.NotFound(w, r)
		return
	}
	requestPath := r.URL.Path
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Path = strings.TrimPrefix(pr.In.URL.Path, prefix)
			pr.Out.URL.RawPath = ""
			pr.SetURL(target)
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Del("Cookie")
		},
		ModifyResponse: func(response *http.Response) error {
			if err := rewriteRepositoryRedirect(response, target, prefix); err != nil {
				return err
			}
			p.record(requestPath, response.StatusCode)
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			p.record(requestPath, http.StatusBadGateway)
			http.Error(w, "repository upstream failed: "+err.Error(), http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func rewriteRepositoryRedirect(response *http.Response, target *url.URL, prefix string) error {
	location := response.Header.Get("Location")
	if location == "" {
		return nil
	}
	redirect, err := url.Parse(location)
	if err != nil {
		return err
	}
	if redirect.Host != "" && redirect.Host != target.Host {
		return nil
	}
	redirect.Scheme, redirect.Host = "", ""
	switch prefix {
	case microsoftPrefix:
		redirect.Path = prefix + redirect.Path
	case ubuntuPrefix:
		redirect.Path = prefix + strings.TrimPrefix(redirect.Path, target.Path)
	}
	response.Header.Set("Location", redirect.String())
	return nil
}

func (p *RepositoryProxy) record(path string, status int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if status >= 400 {
		p.stats.Failures++
		return
	}
	if status < 200 || status >= 300 {
		return
	}
	if strings.HasPrefix(path, "/keys/") {
		p.stats.Keys++
	}
	if isRepositoryMetadata(path) {
		p.stats.Metadata++
	}
}

func isRepositoryMetadata(path string) bool {
	return strings.HasSuffix(path, "/InRelease") || strings.HasSuffix(path, "/Release") ||
		strings.HasSuffix(path, "/repodata/repomd.xml")
}

func NewHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}
