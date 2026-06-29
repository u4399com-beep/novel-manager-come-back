package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleLinkRings(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	switch req.Method {
	case http.MethodGet:
		rows, _ := pool.Query(ctx, "SELECT id,name,ring_type,site_id,max_links,display_mode,link_format,open_new_tab,nofollow,selection_rules,is_active,created_at,updated_at FROM link_rings ORDER BY created_at DESC LIMIT 100")
		rings, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.LinkRing])
		if rows != nil { rows.Close() }
		writeOK(w, rings)
	case http.MethodPost:
		var r models.LinkRing
		if err := json.NewDecoder(req.Body).Decode(&r); err != nil { writeError(w, 400, "invalid JSON"); return }
		if r.Name == "" { writeError(w, 400, "name required"); return }
		err := pool.QueryRow(ctx, "INSERT INTO link_rings (name,ring_type,site_id,max_links,display_mode,link_format,open_new_tab,nofollow,is_active) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id,name,ring_type,site_id,max_links,display_mode,link_format,open_new_tab,nofollow,is_active,created_at,updated_at",
			r.Name, r.RingType, r.SiteID, r.MaxLinks, r.DisplayMode, r.LinkFormat, r.OpenNewTab, r.Nofollow, r.IsActive).
			Scan(&r.ID,&r.Name,&r.RingType,&r.SiteID,&r.MaxLinks,&r.DisplayMode,&r.LinkFormat,&r.OpenNewTab,&r.Nofollow,&r.IsActive,&r.CreatedAt,&r.UpdatedAt)
		if err != nil { writeError(w, 500, "create failed"); return }
		writeJSON(w, 201, r)
	default: writeError(w, 405, "GET/POST required")
	}
}

func (r *Router) handleLinkRingByID(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	rid := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/link-rings/")
	switch req.Method {
	case http.MethodGet:
		var r models.LinkRing
		if err := pool.QueryRow(ctx, "SELECT id,name,ring_type,site_id,max_links,display_mode,link_format,open_new_tab,nofollow,selection_rules,is_active,created_at,updated_at FROM link_rings WHERE id=$1", rid).
			Scan(&r.ID,&r.Name,&r.RingType,&r.SiteID,&r.MaxLinks,&r.DisplayMode,&r.LinkFormat,&r.OpenNewTab,&r.Nofollow,&r.SelectionRules,&r.IsActive,&r.CreatedAt,&r.UpdatedAt); err != nil { writeError(w, 404, "not found"); return }
		writeOK(w, r)
	case http.MethodDelete:
		pool.Exec(ctx, "DELETE FROM link_rings WHERE id=$1", rid); w.WriteHeader(204)
	default: writeError(w, 405, "GET/DELETE required")
	}
}
