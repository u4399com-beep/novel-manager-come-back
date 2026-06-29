// Package api provides the REST JSON API handlers.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/handlers/middleware"
)

type Router struct {
	cfg *config.Config
}

func NewRouter(cfg *config.Config) *Router {
	return &Router{cfg: cfg}
}

func (r *Router) Register(mux *http.ServeMux) {
	p := r.cfg.APIPrefix // /api/v1

	// Public endpoints (no auth)
	mux.HandleFunc(p+"/register", r.handleRegister)
	mux.HandleFunc(p+"/login", r.handleLogin)
	mux.HandleFunc(p+"/search", r.handleSearch)

	// Protected
	auth := middleware.AuthRequired(r.cfg)

	// Auth
	mux.Handle(p+"/me", auth(http.HandlerFunc(r.handleMe)))
	mux.Handle(p+"/me", auth(http.HandlerFunc(r.handleUpdateMe)).(http.HandlerFunc)) // PUT

	// Novels list/create
	mux.HandleFunc(p+"/novels", func(w http.ResponseWriter, req *http.Request) {
		// Route: /api/v1/novels (exact) → list or create
		if req.URL.Path == p+"/novels" || req.URL.Path == p+"/novels/" {
			if req.Method == http.MethodGet {
				r.listNovels(w, req)
			} else if req.Method == http.MethodPost {
				r.createNovel(w, req)
			} else {
				writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
			}
			return
		}
		// Route: /api/v1/novels/{id}/... → single novel, chapters, cover, stats
		r.routeNovelsPrefix(w, req)
	})

	// Categories
	mux.HandleFunc(p+"/categories", r.handleCategories)
	mux.HandleFunc(p+"/categories/", r.handleCategoryByID)

	// Crawler
	mux.HandleFunc(p+"/crawler/sources", r.handleSources)
	mux.HandleFunc(p+"/crawler/trigger", r.handleCrawlTrigger)
	mux.HandleFunc(p+"/crawler/tasks", r.handleCrawlTasks)
	mux.HandleFunc(p+"/crawler/tasks/", r.handleCrawlTaskByID)
	mux.HandleFunc(p+"/crawler/stats", r.handleCrawlStats)

	// Sites
	mux.HandleFunc(p+"/sites", r.handleSites)
	mux.HandleFunc(p+"/sites/", r.handleSiteByID)

	// Link rings
	mux.HandleFunc(p+"/link-rings", r.handleLinkRings)
	mux.HandleFunc(p+"/link-rings/", r.handleLinkRingByID)

	// Cache admin
	mux.HandleFunc(p+"/cache/health", r.handleCacheHealth)
	mux.HandleFunc(p+"/cache/flush", r.handleCacheFlush)

	// Repair
	mux.HandleFunc(p+"/repair/status", r.handleRepairStatus)
	mux.HandleFunc(p+"/repair/chapters", r.handleRepairChapters)
}

// ── JSON helpers ───────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeOK(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, data)
}
