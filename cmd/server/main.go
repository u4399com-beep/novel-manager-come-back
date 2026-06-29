// Package main is the entry point for Come Back Novel CMS.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/handlers/api"
	"github.com/u4399com-beep/novel-manager-come-back/internal/handlers/middleware"
	"github.com/u4399com-beep/novel-manager-come-back/internal/handlers/site"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Come Back Novel CMS v2.0.0 (pgx/PostgreSQL) starting...")

	cfg := config.Load()
	if cfg.IsDevelopment {
		log.Println("Running in DEVELOPMENT mode")
	}

	if err := database.Init(cfg); err != nil {
		log.Fatalf("Database init failed: %v", err)
	}

	mux := http.NewServeMux()
	api.NewRouter(cfg).Register(mux)
	site.NewRouter(cfg).Register(mux)

	// Static files
	if fi, err := os.Stat(cfg.StaticDir); err == nil && fi.IsDir() {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(cfg.StaticDir))))
	} else {
		log.Printf("Static directory not found: %s", cfg.StaticDir)
	}

	// Admin panel
	if fi, err := os.Stat("web/admin"); err == nil && fi.IsDir() {
		afs := http.FileServer(http.Dir("web/admin"))
		mux.Handle("/admin/", http.StripPrefix("/admin/", afs))
		mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
		})
		log.Printf("Admin panel at /admin/")
	}

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		dbOK := database.Pool != nil
		if dbOK {
			if err := database.Pool.Ping(r.Context()); err != nil { dbOK = false }
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   map[bool]string{true: "ok", false: "degraded"}[dbOK],
			"version":  cfg.AppVersion,
			"database": map[bool]string{true: "ok", false: "unreachable"}[dbOK],
		})
	})

	handler := middleware.Recoverer(mux)
	handler = middleware.RequestID(handler)
	handler = middleware.CORSMiddleware(cfg.CORSOrigins)(handler)
	handler = middleware.LimitBodySize(middleware.MaxBodySize)(handler)
	handler = middleware.NewRateLimit(100, 60).Handler(handler)

	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr: addr, Handler: handler,
		ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 120 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
		if database.Pool != nil { database.Pool.Close() }
		log.Println("Shutdown complete.")
		os.Exit(0)
	}()

	log.Printf("Server listening on http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
