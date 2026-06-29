// Package api provides the REST JSON API handlers under /api/v1.
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

	// Public (no auth required)
	mux.HandleFunc(p+"/register", r.handleRegister)
	mux.HandleFunc(p+"/login", r.handleLogin)
	mux.HandleFunc(p+"/search", r.handleSearch)

	// Protected by JWT
	authMW := middleware.AuthRequired(r.cfg)

	// User profile (GET + PUT on same path via method dispatch)
	mux.Handle(p+"/me", authMW(http.HandlerFunc(r.handleMe)))

	// Novels (REST + nested routes)
	mux.HandleFunc(p+"/novels", r.handleNovelsRouting)
	mux.HandleFunc(p+"/novels/", r.handleNovelsRouting)

	// Categories (auth required)
	mux.Handle(p+"/categories", authMW(http.HandlerFunc(r.handleCategories)))
	mux.Handle(p+"/categories/", authMW(http.HandlerFunc(r.handleCategoryByID)))

	// Crawler
	mux.Handle(p+"/crawler/sources", authMW(http.HandlerFunc(r.handleSources)))
	mux.Handle(p+"/crawler/trigger", authMW(http.HandlerFunc(r.handleCrawlTrigger)))
	mux.Handle(p+"/crawler/tasks", authMW(http.HandlerFunc(r.handleCrawlTasks)))
	mux.Handle(p+"/crawler/tasks/", authMW(http.HandlerFunc(r.handleCrawlTaskByID)))
	mux.Handle(p+"/crawler/stats", authMW(http.HandlerFunc(r.handleCrawlStats)))

	// Sites
	mux.Handle(p+"/sites", authMW(http.HandlerFunc(r.handleSites)))
	mux.Handle(p+"/sites/", authMW(http.HandlerFunc(r.handleSiteByID)))

	// Link rings
	mux.Handle(p+"/link-rings", authMW(http.HandlerFunc(r.handleLinkRings)))
	mux.Handle(p+"/link-rings/", authMW(http.HandlerFunc(r.handleLinkRingByID)))

	// Cache admin
	mux.Handle(p+"/cache/health", authMW(http.HandlerFunc(r.handleCacheHealth)))
	mux.Handle(p+"/cache/flush", authMW(http.HandlerFunc(r.handleCacheFlush)))

	// Repair
	mux.Handle(p+"/repair/status", authMW(http.HandlerFunc(r.handleRepairStatus)))
	mux.Handle(p+"/repair/chapters", authMW(http.HandlerFunc(r.handleRepairChapters)))
}

// handleNovelsRouting dispatches /api/v1/novels and /api/v1/novels/{id}/...
func (r *Router) handleNovelsRouting(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	prefix := r.cfg.APIPrefix + "/novels"

	// Exact match on /api/v1/novels or /api/v1/novels/
	if path == prefix || path == prefix+"/" {
		switch req.Method {
		case http.MethodGet:
			r.listNovels(w, req)
		case http.MethodPost:
			r.createNovel(w, req)
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
		}
		return
	}

	// Sub-path: /api/v1/novels/{id}/...
	r.routeNovelsPrefix(w, req)
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
