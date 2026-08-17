// Command taawun-relay runs an independently deployable, zero-storage signaling relay.
package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"taawun/pkg/primitives"
)

func main() {
	secret := []byte(os.Getenv("RELAY_SHARED_SECRET"))
	if len(secret) < 32 {
		log.Fatal("RELAY_SHARED_SECRET must contain at least 32 bytes")
	}
	config := primitives.RelayConfig{
		SharedSecret:   secret,
		AllowedOrigins: splitCSV(os.Getenv("RELAY_ALLOWED_ORIGINS")),
	}
	if len(config.AllowedOrigins) == 0 {
		log.Fatal("RELAY_ALLOWED_ORIGINS must list at least one browser origin")
	}
	hub, err := primitives.NewP2PRelayHubWithConfig(config)
	if err != nil {
		log.Fatalf("configure relay: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","service":"taawun-relay"}`))
	})
	mux.HandleFunc("/api/p2p/stream", hub.HandleP2PStream)
	address := os.Getenv("RELAY_LISTEN_ADDR")
	if address == "" {
		address = ":8090"
	}
	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Taawun relay listening on %s", address)
	log.Fatal(server.ListenAndServe())
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	values := strings.Split(value, ",")
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
