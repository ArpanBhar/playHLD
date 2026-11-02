package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"playhld/backend/internal/api"
	"playhld/backend/internal/storage"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	options := api.Options{SecureCookies: os.Getenv("COOKIE_SECURE") == "1", EnableNativeSimulation: os.Getenv("ENABLE_NATIVE_SIMULATION") == "1"}
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		pool, err := storage.Open(context.Background(), databaseURL)
		if err != nil {
			log.Fatalf("database connection failed: %v", err)
		}
		defer pool.Close()
		if err := storage.Migrate(context.Background(), pool); err != nil {
			log.Fatalf("database migration failed: %v", err)
		}
		options.DB = pool
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.NewServerWithOptions(options),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("playHLD API listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
