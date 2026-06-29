package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleSites(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	switch req.Method {
	case http.MethodGet:
		rows, _ := pool.Query(ctx, "SELECT id,domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules,created_at,updated_at FROM sites ORDER BY created_at DESC LIMIT 200")
		sites, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.Site])
		if rows != nil { rows.Close() }
		writeOK(w, sites)
	case http.MethodPost:
		var s models.Site
		if err := json.NewDecoder(req.Body).Decode(&s); err != nil { writeError(w, 400, "invalid JSON"); return }
		if s.Name == "" || s.Domain == "" { writeError(w, 400, "name and domain required"); return }
		err := pool.QueryRow(ctx, "INSERT INTO sites (domain,name,template,offset_val,description,is_active,translate_enabled,language) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,domain,name,template,offset_val,description,is_active,translate_enabled,language,created_at,updated_at",
			s.Domain, s.Name, s.Template, s.Offset, s.Description, s.IsActive, s.TranslateEnabled, s.Language).
			Scan(&s.ID,&s.Domain,&s.Name,&s.Template,&s.Offset,&s.Description,&s.IsActive,&s.TranslateEnabled,&s.Language,&s.CreatedAt,&s.UpdatedAt)
		if err != nil { writeError(w, 409, "domain exists"); return }
		writeJSON(w, 201, s)
	default: writeError(w, 405, "GET/POST required")
	}
}

func (r *Router) handleSiteByID(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	sid := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/sites/")
	switch req.Method {
	case http.MethodGet:
		var s models.Site
		if err := pool.QueryRow(ctx, "SELECT id,domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules,created_at,updated_at FROM sites WHERE id=$1", sid).
			Scan(&s.ID,&s.Domain,&s.Name,&s.Template,&s.Offset,&s.Description,&s.IsActive,&s.TranslateEnabled,&s.Language,&s.URLPatterns,&s.ChapterPagination,&s.LinkWheel,&s.RecommendModules,&s.CreatedAt,&s.UpdatedAt); err != nil { writeError(w, 404, "not found"); return }
		writeOK(w, s)
	case http.MethodPut:
		var u map[string]interface{}
		json.NewDecoder(req.Body).Decode(&u)
		for k, col := range map[string]string{"name":"name","domain":"domain","template":"template","language":"language"} {
			if v, ok := u[k]; ok { pool.Exec(ctx, "UPDATE sites SET "+col+"=$1 WHERE id=$2", v, sid) }
		}
		var s models.Site
		pool.QueryRow(ctx, "SELECT id,domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules,created_at,updated_at FROM sites WHERE id=$1", sid).
			Scan(&s.ID,&s.Domain,&s.Name,&s.Template,&s.Offset,&s.Description,&s.IsActive,&s.TranslateEnabled,&s.Language,&s.URLPatterns,&s.ChapterPagination,&s.LinkWheel,&s.RecommendModules,&s.CreatedAt,&s.UpdatedAt)
		writeOK(w, s)
	case http.MethodDelete:
		pool.Exec(ctx, "DELETE FROM sites WHERE id=$1", sid); w.WriteHeader(204)
	default: writeError(w, 405, "GET/PUT/DELETE required")
	}
}
