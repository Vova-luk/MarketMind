package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: stub bybit|telegram")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	switch os.Args[1] {
	case "bybit":
		registerBybit(mux)
	case "telegram":
		token := os.Getenv("BOT_TOKEN")
		if token == "" {
			log.Fatal("BOT_TOKEN is required for the Telegram stub")
		}
		registerTelegram(mux, token)
	default:
		log.Fatalf("unknown stub %q", os.Args[1])
	}
	server := &http.Server{
		Addr: ":8080", Handler: mux,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}
