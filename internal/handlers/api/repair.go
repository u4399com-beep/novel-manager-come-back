package api

import (
	"net/http"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleRepairStatus(w http.ResponseWriter, req *http.Request) {
	var emptyCh, noCover, noDesc, noAuthor, total int64

	database.DB.Model(&models.Chapter{}).
		Where("content = '' AND content_file = ''").Count(&emptyCh)
	database.DB.Model(&models.Novel{}).
		Where("cover_image_url = '' OR cover_image_url IS NULL").Count(&noCover)
	database.DB.Model(&models.Novel{}).
		Where("description = '' OR description IS NULL").Count(&noDesc)
	database.DB.Model(&models.Novel{}).
		Where("author = '' OR author IS NULL").Count(&noAuthor)
	database.DB.Model(&models.Novel{}).Count(&total)

	writeOK(w, map[string]interface{}{
		"empty_chapters": emptyCh,
		"no_cover":       noCover,
		"no_description": noDesc,
		"no_author":      noAuthor,
		"total_novels":   total,
		"tasks_running": map[string]bool{
			"repair_chapters": false,
			"repair_covers":   false,
			"repair_info":     false,
		},
	})
}

func (r *Router) handleRepairChapters(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	// Background repair — for now, returns status
	writeOK(w, map[string]interface{}{
		"message": "Chapter repair would run in background (not yet implemented in Go version)",
		"success": false,
	})
}
