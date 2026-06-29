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
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("Come Back Novel CMS v2.0.0 starting...")

	cfg := config.Load()
	if cfg.IsDevelopment {
		log.Println("Running in DEVELOPMENT mode")
	}
	if cfg.SecretKey == "change-me-in-production-use-a-strong-random-key" && !cfg.IsDevelopment {
		log.Println("WARNING: Using default SECRET_KEY — JWT tokens are forgeable!")
	}

	// Database
	if err := database.Init(cfg); err != nil {
		log.Fatalf("Database init failed: %v", err)
	}

	// Auto-migrate all models
	if err := database.DB.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Novel{},
		&models.Chapter{},
		&models.CrawlerTask{},
		&models.Site{},
		&models.LinkRing{},
		&models.LinkRingTarget{},
		&models.TranslationCache{},
	); err != nil {
		log.Printf("AutoMigrate warning: %v", err)
	}

	// Router
	mux := http.NewServeMux()

	apiRouter := api.NewRouter(cfg)
	apiRouter.Register(mux)

	siteRouter := site.NewRouter(cfg)
	siteRouter.Register(mux)

	// Static files
	if fi, err := os.Stat(cfg.StaticDir); err == nil && fi.IsDir() {
		fs := http.FileServer(http.Dir(cfg.StaticDir))
		mux.Handle("/static/", http.StripPrefix("/static/", fs))
	} else {
		log.Printf("Static directory not found: %s (serving without static files)", cfg.StaticDir)
	}

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		dbOK := true
		if database.DB == nil {
			dbOK = false
		} else if sqlDB, err := database.DB.DB(); err != nil || sqlDB.Ping() != nil {
			dbOK = false
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   map[bool]string{true: "ok", false: "degraded"}[dbOK],
			"version":  cfg.AppVersion,
			"database": map[bool]string{true: "ok", false: "unreachable"}[dbOK],
		})
	})

	// Middleware stack (outermost first)
	handler := middleware.Recoverer(mux)
	handler = middleware.RequestID(handler)
	handler = middleware.CORSMiddleware(cfg.CORSOrigins)(handler)
	handler = middleware.LimitBodySize(middleware.MaxBodySize)(handler)
	handler = middleware.NewRateLimit(100, 60).Handler(handler)

	// Server
	addr := ":" + cfg.ServerPort
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down gracefully...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
		if database.DB != nil {
			if sqlDB, err := database.DB.DB(); err == nil && sqlDB != nil {
				sqlDB.Close()
			}
		}
		log.Println("Shutdown complete.")
		os.Exit(0)
	}()

	log.Printf("Server listening on http://localhost%s", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
