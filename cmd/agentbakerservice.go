package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	agent "github.com/Azure/agentbaker/pkg/agent"
	"github.com/Azure/agentbaker/pkg/agent/datamodel"
)

func handleError(err error, w http.ResponseWriter) {
	log.Println(err.Error())
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func main() {

	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "0.0.1")
	})

	http.HandleFunc("/nodebootstrapping", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var config datamodel.NodeBootstrappingConfiguration

		err := json.NewDecoder(r.Body).Decode(&config)
		if err != nil {
			handleError(err, w)
			return
		}

		agentBaker, err := agent.NewAgentBaker()
		if err != nil {
			handleError(err, w)
			return
		}
		nodeBootStrapping, err := agentBaker.GetNodeBootstrapping(ctx, &config)
		if err != nil {
			handleError(err, w)
			return
		}
		result, err := json.Marshal(nodeBootStrapping)
		if err != nil {
			handleError(err, w)
			return
		}
		fmt.Fprint(w, string(result))
	})

	fmt.Printf("Starting server at port 8081\n")
	log.Fatal(http.ListenAndServe(":8081", nil))
}
