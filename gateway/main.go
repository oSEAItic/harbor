package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/oseaitic/harbor/internal/cache"
	"github.com/oseaitic/harbor/internal/connector"
	"github.com/oseaitic/harbor/internal/protocol"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	responseCache := cache.New(5 * time.Minute)

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

		// Check cache
		cacheKey := cache.Key(req.Connector, req.Resource, req.Params)
		if cached := responseCache.Get(cacheKey); cached != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Harbor-Cache", "hit")
			w.Write(cached)
			return
		}

		// Execute connector
		resp, err := connector.Execute(req)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			errResp := protocol.NewErrorResponse(req.Connector, "", protocol.ErrExecution, err.Error())
			json.NewEncoder(w).Encode(errResp)
			return
		}

		// Cache the response
		respBytes, _ := json.Marshal(resp)
		responseCache.Set(cacheKey, respBytes)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Harbor-Cache", "miss")
		w.Write(respBytes)
	})

	log.Printf("Harbor Gateway listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), mux); err != nil {
		log.Fatal(err)
	}
}
