package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"itemcosttracker/internal/handler"
	"itemcosttracker/internal/store"
)

//go:embed templates static
var embeddedFS embed.FS

const version = "1.2.0"

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	dataFile := dataDir + "/items.json"

	s, err := store.New(dataFile)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}

	// Use the embedded FS directly — template paths include the "templates/" prefix
	templateFS, err := fs.Sub(embeddedFS, ".")
	if err != nil {
		log.Fatalf("failed to create template FS: %v", err)
	}

	staticFS, err := fs.Sub(embeddedFS, "static")
	if err != nil {
		log.Fatalf("failed to create static FS: %v", err)
	}

	h, err := handler.New(s, templateFS, version)
	if err != nil {
		log.Fatalf("failed to initialize handlers: %v", err)
	}

	mux := http.NewServeMux()
	h.Register(mux, staticFS)

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("ItemCostTracker listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, logRequests(mux)))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
