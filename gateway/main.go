package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	harborctx "github.com/oseaitic/harbor/internal/context"
	"github.com/oseaitic/harbor/internal/pipeline"
	"github.com/oseaitic/harbor/internal/protocol"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"service": "harbor-gateway",
		})
	})

	mux.HandleFunc("POST /run", func(w http.ResponseWriter, r *http.Request) {
		var req protocol.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}

		if req.Connector == "" || req.Resource == "" {
			http.Error(w, `{"error":"connector and resource are required"}`, http.StatusBadRequest)
			return
		}

		result, err := pipeline.Execute(req.Connector, req.Resource, req.Params, nil, pipeline.Options{
			Compile: harborctx.DefaultOptions(),
		})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			errResp := protocol.NewErrorResponse(req.Connector, "", protocol.ErrExecution, err.Error())
			json.NewEncoder(w).Encode(errResp)
			return
		}

		respBytes, _ := json.Marshal(result.Response)

		w.Header().Set("Content-Type", "application/json")
		if result.FromMem {
			w.Header().Set("X-Harbor-Cache", "hit")
		} else {
			w.Header().Set("X-Harbor-Cache", "miss")
		}
		w.Write(respBytes)
	})

	log.Printf("Harbor Gateway listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatal(err)
	}
}
