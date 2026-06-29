package api

import (
	"net/http"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
)

func (r *Router) handleRepairStatus(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	var e, nc, nd, na, total int64
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM chapters WHERE content='' AND content_file=''").Scan(&e)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels WHERE cover_image_url='' OR cover_image_url IS NULL").Scan(&nc)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels WHERE description='' OR description IS NULL").Scan(&nd)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels WHERE author='' OR author IS NULL").Scan(&na)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels").Scan(&total)
	writeOK(w, map[string]interface{}{
		"empty_chapters":e,"no_cover":nc,"no_description":nd,"no_author":na,"total_novels":total,
		"tasks_running":map[string]bool{"repair_chapters":false,"repair_covers":false,"repair_info":false},
	})
}

func (r *Router) handleRepairChapters(w http.ResponseWriter, req *http.Request) {
	writeOK(w, map[string]interface{}{"message":"repair not yet implemented in Go version","success":false})
}
