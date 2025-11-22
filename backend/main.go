package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
)

// main only wires dependencies and starts the HTTP server.
func main() {
	// Initialize repository
	var repo Repository
	dsn := os.Getenv("DATABASE_URL")
	if dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pg, err := NewPGRepo(ctx, dsn)
		if err != nil {
			log.Fatalf("failed to init db: %v", err)
		}
		repo = pg
		log.Println("connected to Postgres")
	} else {
		log.Println("DATABASE_URL not set; server will start but DB-backed endpoints will fail")
		repo = &noopRepo{}
	}

	srv := NewServer(repo)
	addr := ":8080"
	if v := os.Getenv("ADDR"); v != "" {
		addr = v
	}
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		log.Fatal(err)
	}
}
