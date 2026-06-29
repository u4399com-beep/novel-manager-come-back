package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleCategories(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	switch req.Method {
	case http.MethodGet:
		rows, _ := pool.Query(ctx, "SELECT id,name,slug,sort_order,created_at,updated_at FROM categories ORDER BY sort_order")
		cats, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.Category])
		if rows != nil { rows.Close() }
		writeOK(w, cats)
	case http.MethodPost:
		var b struct{ Name, Slug string; SortOrder int }
		if err := json.NewDecoder(req.Body).Decode(&b); err != nil { writeError(w, 400, "invalid JSON"); return }
		if b.Name == "" || b.Slug == "" { writeError(w, 400, "name and slug required"); return }
		var c models.Category
		err := pool.QueryRow(ctx, "INSERT INTO categories (name,slug,sort_order) VALUES ($1,$2,$3) RETURNING id,name,slug,sort_order,created_at,updated_at", b.Name, b.Slug, b.SortOrder).
			Scan(&c.ID, &c.Name, &c.Slug, &c.SortOrder, &c.CreatedAt, &c.UpdatedAt)
		if err != nil { writeError(w, 409, "category exists"); return }
		writeJSON(w, 201, c)
	default: writeError(w, 405, "GET/POST required")
	}
}

func (r *Router) handleCategoryByID(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	idStr := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/categories/")
	switch req.Method {
	case http.MethodGet:
		var c models.Category
		if err := pool.QueryRow(ctx, "SELECT id,name,slug,sort_order,created_at,updated_at FROM categories WHERE id=$1", idStr).
			Scan(&c.ID,&c.Name,&c.Slug,&c.SortOrder,&c.CreatedAt,&c.UpdatedAt); err != nil { writeError(w, 404, "not found"); return }
		writeOK(w, c)
	case http.MethodPut:
		var u map[string]interface{}
		json.NewDecoder(req.Body).Decode(&u)
		for k, v := range map[string]string{"name":"name","slug":"slug","sort_order":"sort_order"} {
			if val, ok := u[k]; ok { pool.Exec(ctx, "UPDATE categories SET "+v+"=$1 WHERE id=$2", val, idStr) }
		}
		var c models.Category
		pool.QueryRow(ctx, "SELECT id,name,slug,sort_order,created_at,updated_at FROM categories WHERE id=$1", idStr).
			Scan(&c.ID,&c.Name,&c.Slug,&c.SortOrder,&c.CreatedAt,&c.UpdatedAt)
		writeOK(w, c)
	case http.MethodDelete:
		pool.Exec(ctx, "DELETE FROM categories WHERE id=$1", idStr)
		w.WriteHeader(204)
	default: writeError(w, 405, "GET/PUT/DELETE required")
	}
}
