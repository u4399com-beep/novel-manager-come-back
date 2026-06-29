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
		if sites == nil { sites = []models.Site{} }
		writeOK(w, sites)
	case http.MethodPost:
		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil { writeError(w, 400, "invalid JSON"); return }
		name, _ := body["name"].(string)
		domain, _ := body["domain"].(string)
		if name == "" || domain == "" { writeError(w, 400, "name and domain required"); return }

		// Marshal JSON fields
		urlP := marshalJSONField(body["url_patterns"])
		chP := marshalJSONField(body["chapter_pagination"])
		lw := marshalJSONField(body["link_wheel"])
		rm := marshalJSONField(body["recommend_modules"])

		template, _ := body["template"].(string)
		if template == "" { template = "default" }
		language, _ := body["language"].(string)
		if language == "" { language = "zh" }
		offsetVal, _ := body["offset_val"].(float64)
		desc, _ := body["description"].(string)
		isActive := toBool(body["is_active"], true)
		transEnabled := toBool(body["translate_enabled"], true)

		var s models.Site
		err := pool.QueryRow(ctx,
			`INSERT INTO sites (domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules)
			 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			 RETURNING id,domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules,created_at,updated_at`,
			domain, name, template, int(offsetVal), desc, isActive, transEnabled, language, urlP, chP, lw, rm,
		).Scan(&s.ID, &s.Domain, &s.Name, &s.Template, &s.Offset, &s.Description, &s.IsActive, &s.TranslateEnabled, &s.Language, &s.URLPatterns, &s.ChapterPagination, &s.LinkWheel, &s.RecommendModules, &s.CreatedAt, &s.UpdatedAt)
		if err != nil { writeError(w, 409, "domain exists or invalid data: "+err.Error()); return }
		writeJSON(w, 201, s)
	default:
		writeError(w, 405, "GET/POST required")
	}
}

func (r *Router) handleSiteByID(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	sid := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/sites/")
	switch req.Method {
	case http.MethodGet:
		var s models.Site
		err := pool.QueryRow(ctx, "SELECT id,domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules,created_at,updated_at FROM sites WHERE id=$1", sid).
			Scan(&s.ID, &s.Domain, &s.Name, &s.Template, &s.Offset, &s.Description, &s.IsActive, &s.TranslateEnabled, &s.Language, &s.URLPatterns, &s.ChapterPagination, &s.LinkWheel, &s.RecommendModules, &s.CreatedAt, &s.UpdatedAt)
		if err != nil { writeError(w, 404, "not found"); return }
		writeOK(w, s)
	case http.MethodPut:
		var body map[string]interface{}
		json.NewDecoder(req.Body).Decode(&body)
		for k, col := range map[string]string{"name": "name", "domain": "domain", "template": "template", "language": "language", "description": "description"} {
			if v, ok := body[k]; ok { pool.Exec(ctx, "UPDATE sites SET "+col+"=$1 WHERE id=$2", v, sid) }
		}
		if v, ok := body["offset_val"]; ok { pool.Exec(ctx, "UPDATE sites SET offset_val=$1 WHERE id=$2", int(toFloat(v)), sid) }
		if v, ok := body["is_active"]; ok { pool.Exec(ctx, "UPDATE sites SET is_active=$1 WHERE id=$2", toBool(v, true), sid) }
		if v, ok := body["translate_enabled"]; ok { pool.Exec(ctx, "UPDATE sites SET translate_enabled=$1 WHERE id=$2", toBool(v, true), sid) }
		for _, field := range []string{"url_patterns", "chapter_pagination", "link_wheel", "recommend_modules"} {
			if v, ok := body[field]; ok { pool.Exec(ctx, "UPDATE sites SET "+field+"=$1 WHERE id=$2", marshalJSONField(v), sid) }
		}
		var s models.Site
		pool.QueryRow(ctx, "SELECT id,domain,name,template,offset_val,description,is_active,translate_enabled,language,url_patterns,chapter_pagination,link_wheel,recommend_modules,created_at,updated_at FROM sites WHERE id=$1", sid).
			Scan(&s.ID, &s.Domain, &s.Name, &s.Template, &s.Offset, &s.Description, &s.IsActive, &s.TranslateEnabled, &s.Language, &s.URLPatterns, &s.ChapterPagination, &s.LinkWheel, &s.RecommendModules, &s.CreatedAt, &s.UpdatedAt)
		writeOK(w, s)
	case http.MethodDelete:
		pool.Exec(ctx, "DELETE FROM sites WHERE id=$1", sid)
		w.WriteHeader(204)
	default:
		writeError(w, 405, "GET/PUT/DELETE required")
	}
}

func marshalJSONField(v interface{}) string {
	if v == nil { return "{}" }
	switch val := v.(type) {
	case string: return val
	case map[string]interface{}: b, _ := json.Marshal(val); return string(b)
	default: b, _ := json.Marshal(val); return string(b)
	}
}

func toBool(v interface{}, defaultVal bool) bool {
	switch val := v.(type) {
	case bool: return val
	case float64: return val != 0
	case string: return val == "true" || val == "1"
	default: return defaultVal
	}
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64: return val
	case int: return float64(val)
	case int64: return float64(val)
	default: return 0
	}
}
