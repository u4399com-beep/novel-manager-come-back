// Package api provides the REST JSON API handlers under /api/v1.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/handlers/middleware"
	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

type Router struct {
	cfg *config.Config
}

func NewRouter(cfg *config.Config) *Router {
	return &Router{cfg: cfg}
}

func (r *Router) Register(mux *http.ServeMux) {
	p := r.cfg.APIPrefix // /api/v1

	// Public
	mux.HandleFunc(p+"/register", r.handleRegister)
	mux.HandleFunc(p+"/login", r.handleLogin)
	mux.HandleFunc(p+"/search", r.handleSearch)
	mux.HandleFunc(p+"/novels", r.handleNovelsRouting)
	mux.HandleFunc(p+"/novels/", r.handleNovelsRouting)

	// Protected
	auth := middleware.AuthRequired(r.cfg)
	mux.Handle(p+"/me", auth(http.HandlerFunc(r.handleMe)))
	mux.Handle(p+"/categories", auth(http.HandlerFunc(r.handleCategories)))
	mux.Handle(p+"/categories/", auth(http.HandlerFunc(r.handleCategoryByID)))
	mux.Handle(p+"/crawler/sources", auth(http.HandlerFunc(r.handleSources)))
	mux.Handle(p+"/crawler/trigger", auth(http.HandlerFunc(r.handleCrawlTrigger)))
	mux.Handle(p+"/crawler/tasks", auth(http.HandlerFunc(r.handleCrawlTasks)))
	mux.Handle(p+"/crawler/tasks/", auth(http.HandlerFunc(r.handleCrawlTaskByID)))
	mux.Handle(p+"/crawler/stats", auth(http.HandlerFunc(r.handleCrawlStats)))
	mux.Handle(p+"/sites", auth(http.HandlerFunc(r.handleSites)))
	mux.Handle(p+"/sites/", auth(http.HandlerFunc(r.handleSiteByID)))
	mux.Handle(p+"/link-rings", auth(http.HandlerFunc(r.handleLinkRings)))
	mux.Handle(p+"/link-rings/", auth(http.HandlerFunc(r.handleLinkRingByID)))
	mux.HandleFunc(p+"/rules", func(w http.ResponseWriter, req *http.Request) {
		// Check token first (simple auth)
		if !r.requireAuthBool(w, req) { return }
		r.handleRulesList(w, req)
	})
	mux.HandleFunc(p+"/rules/", func(w http.ResponseWriter, req *http.Request) {
		if !r.requireAuthBool(w, req) { return }
		path := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/rules/")
		if path == "test" { r.handleRuleTest(w, req); return }
		if path == "import" { r.handleRuleImport(w, req); return }
		r.handleRuleByID(w, req)
	})
	mux.Handle(p+"/cache/health", auth(http.HandlerFunc(r.handleCacheHealth)))
	mux.Handle(p+"/cache/flush", auth(http.HandlerFunc(r.handleCacheFlush)))
	mux.Handle(p+"/repair/status", auth(http.HandlerFunc(r.handleRepairStatus)))
	mux.Handle(p+"/repair/chapters", auth(http.HandlerFunc(r.handleRepairChapters)))
}

func (r *Router) handleNovelsRouting(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	prefix := r.cfg.APIPrefix + "/novels"
	isList := path == prefix || path == prefix+"/"

	// POST/PUT/DELETE require auth; GET is public
	if !isList || req.Method != http.MethodGet {
		ctx, ok := r.requireAuth(w, req)
		if !ok {
			return
		}
		req = req.WithContext(ctx)
	}

	if isList {
		switch req.Method {
		case http.MethodGet:
			r.listNovels(w, req)
		case http.MethodPost:
			r.createNovel(w, req)
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET or POST required")
		}
		return
	}
	r.routeNovelsPrefix(w, req)
}

// requireAuthBool checks JWT and returns true if valid (for HandleFunc wrappers).
func (r *Router) requireAuthBool(w http.ResponseWriter, req *http.Request) bool {
	_, ok := r.requireAuth(w, req)
	return ok
}

// requireAuth validates JWT and injects user info into context (mirrors middleware.AuthRequired).
func (r *Router) requireAuth(w http.ResponseWriter, req *http.Request) (context.Context, bool) {
	ah := req.Header.Get("Authorization")
	if ah == "" || !strings.HasPrefix(ah, "Bearer ") {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	claims, err := services.ParseAccessToken(r.cfg, strings.TrimPrefix(ah, "Bearer "))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired token")
		return nil, false
	}
	userID, _ := claims["sub"].(string)
	role, _ := claims["role"].(string)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return nil, false
	}
	ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.UserRoleKey, role)
	return ctx, true
}

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
