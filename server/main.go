package main

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"

	"tablic/server/internal/room"
	"tablic/server/internal/storage"
	"tablic/server/internal/ws"
)

//go:embed all:static
var staticFiles embed.FS

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3579"
	}
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "tablic.db"
	}

	st, err := storage.Open(dbPath)
	if err != nil {
		log.Printf("storage: failed to open database: %v (continuing without recording)", err)
	}
	manager := room.NewManager(st)
	wsHandler := ws.NewHandler(manager)

	mux := http.NewServeMux()
	mux.Handle("/ws", wsHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("/api/rooms", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(manager.List())
	})
	mux.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if st == nil {
			w.Write([]byte("[]"))
			return
		}
		records, err := st.QueryHistory(30)
		if err != nil {
			log.Printf("history query error: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(records)
	})
	stripped, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", spaHandler(stripped))

	log.Printf("Tablić server listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// spaHandler serves static files and falls back to index.html for unknown paths.
func spaHandler(root fs.FS) http.Handler {
	fsh := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := root.Open(path); errors.Is(err, fs.ErrNotExist) {
			r.URL.Path = "/"
		}
		fsh.ServeHTTP(w, r)
	})
}
