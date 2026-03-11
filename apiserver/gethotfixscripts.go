package apiserver

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Azure/agentbaker/parts"
)

const (
	// RoutePathHotfixScripts is the route path for retrieving hotfixed provisioning scripts.
	RoutePathHotfixScripts string = "/gethotfixscripts"
)

// GetHotfixScriptsRequest is the request body for the hotfix scripts endpoint.
type GetHotfixScriptsRequest struct {
	OsSku      string `json:"osSku"`
	VhdVersion string `json:"vhdVersion"`
}

// HotfixFile represents a single hotfixed script to be delivered to the node.
type HotfixFile struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	Permissions string `json:"permissions"`
}

// HotfixEntry maps an embedded source script to its on-disk destination.
type HotfixEntry struct {
	SourcePath      string
	DestinationPath string
	Permissions     string
}

// hotfixRegistry maps "osSku:vhdVersion" to the list of scripts that need hotfixing.
// Engineers populate this map when creating a hotfix for specific VHD versions.
// When no hotfixes are active, this map is empty.
var hotfixRegistry = map[string][]HotfixEntry{
	// Example (commented out — uncomment and populate when a hotfix is needed):
	//
	// "AKSUbuntu2204:202502.15.0": {
	//   {
	//     SourcePath:      "linux/cloud-init/artifacts/cse_helpers.sh",
	//     DestinationPath: "/opt/azure/containers/provision_source.sh",
	//     Permissions:     "0744",
	//   },
	// },
}

// GetHotfixScripts returns the hotfixed script files for a given osSku and vhdVersion.
func (api *APIServer) GetHotfixScripts(w http.ResponseWriter, r *http.Request) {
	var req GetHotfixScriptsRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.OsSku == "" || req.VhdVersion == "" {
		http.Error(w, "osSku and vhdVersion are required", http.StatusBadRequest)
		return
	}

	key := req.OsSku + ":" + req.VhdVersion
	entries, ok := hotfixRegistry[key]
	if !ok {
		// No hotfix registered for this osSku:vhdVersion — return empty list.
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "[]")
		return
	}

	files := make([]HotfixFile, 0, len(entries))
	for _, entry := range entries {
		content, err := parts.Templates.ReadFile(entry.SourcePath)
		if err != nil {
			log.Printf("failed to read hotfix source %s: %s", entry.SourcePath, err.Error())
			http.Error(w, fmt.Sprintf("failed to read hotfix source %s: %s", entry.SourcePath, err.Error()), http.StatusInternalServerError)
			return
		}

		files = append(files, HotfixFile{
			Path:        entry.DestinationPath,
			Content:     base64.StdEncoding.EncodeToString(content),
			Permissions: entry.Permissions,
		})
	}

	result, err := json.Marshal(files)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, string(result))
}
