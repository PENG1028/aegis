// Multi-service E2E test harness. Starts N service instances that register with Aegis
// and test the full ServiceAuth v2 chain: register → ticket → guard → groups → policies.
//
// Usage:
//
//	go run . -name=depotly -port=8081 -aegis=http://127.0.0.1:7380
//	go run . -name=aetherion -port=8082
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"aegis/pkg/serviceauth"
)

func main() {
	name := flag.String("name", "test-svc", "service name")
	port := flag.Int("port", 8080, "listen port")
	aegisURL := flag.String("aegis", "http://127.0.0.1:7380", "Aegis URL")
	flag.Parse()

	client, err := serviceauth.New(serviceauth.Config{
		ServiceName: *name,
		AegisURL:    *aegisURL,
	})
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	if err := client.Register(context.Background()); err != nil {
		log.Printf("WARNING: register failed (Aegis may not be running): %v", err)
	} else {
		log.Printf("registered as %s (pubkey=%s...)", *name, client.PublicKey()[:20])
	}
	defer client.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": *name})
	})

	mux.Handle("POST /ping", client.Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := serviceauth.CallerFromContext(r.Context())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"pong": true, "from": caller.ServiceName, "host": caller.CallerHost,
		})
	})))

	mux.Handle("GET /group-check", client.Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caller := serviceauth.CallerFromContext(r.Context())
		inCore := client.InGroup(caller.ServiceName, "core-services")
		inStorage := client.InGroup(caller.ServiceName, "storage-group")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"caller":           caller.ServiceName,
			"in_core_services":  inCore,
			"in_storage_group":  inStorage,
		})
	})))

	// Periodically call other services for integration testing
	go func() {
		time.Sleep(2 * time.Second)
		targets := []string{"monitor-svc", "aetherion"}
		for _, t := range targets {
			url := fmt.Sprintf("http://%s:8080/ping", t)
			resp, err := client.Post(context.Background(), url, nil)
			if err != nil {
				log.Printf("call %s: %v", t, err)
			} else {
				var result map[string]interface{}
				json.NewDecoder(resp.Body).Decode(&result)
				resp.Body.Close()
				log.Printf("%s → %s: %v", *name, t, result)
			}
		}
	}()

	log.Printf("%s listening on :%d", *name, *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), mux))
}
