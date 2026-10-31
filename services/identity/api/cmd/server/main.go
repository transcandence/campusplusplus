// Command server runs the identity service HTTP API.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/transcandence/campusplusplus/services/identity/api/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error loading config:", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello world")
	})

	log.Printf("listening on %s (env=%s)", cfg.HTTPAddr, cfg.Env)
	if err := http.ListenAndServe(cfg.HTTPAddr, mux); err != nil {
		log.Fatal(err)
	}
}
