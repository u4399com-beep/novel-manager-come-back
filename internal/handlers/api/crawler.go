package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleSources(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	writeOK(w, []map[string]string{
		{"source_name": "23qb", "base_url": "https://www.23qb.net", "description": "铅笔小说 (23qb.net)"},
	})
}

func (r *Router) handleCrawlTrigger(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var body struct {
		NovelID    string `json:"novel_id"`
		SourceName string `json:"source_name"`
		Mode       string `json:"mode"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.NovelID == "" {
		writeError(w, http.StatusBadRequest, "novel_id required")
		return
	}
	if body.Mode == "" {
		body.Mode = "direct"
	}

	var novel models.Novel
	if err := database.DB.First(&novel, "id = ?", body.NovelID).Error; err != nil {
		writeError(w, http.StatusNotFound, "novel not found")
		return
	}

	task := models.CrawlerTask{NovelID: body.NovelID, Status: "pending"}
	if err := database.DB.Create(&task).Error; err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	writeJSON(w, http.StatusAccepted, task)
}

func (r *Router) handleCrawlTasks(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	q := req.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("size"))
	if size < 1 || size > 100 {
		size = 20
	}

	var tasks []models.CrawlerTask
	var total int64
	db := database.DB.Model(&models.CrawlerTask{})
	if novelID := q.Get("novel_id"); novelID != "" {
		db = db.Where("novel_id = ?", novelID)
	}
	if status := q.Get("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	db.Count(&total)
	db.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&tasks)

	pages := calcPages(total, int64(size))
	writeOK(w, map[string]interface{}{
		"items": tasks, "total": total, "page": page, "size": size, "pages": pages,
	})
}

func (r *Router) handleCrawlTaskByID(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/crawler/tasks/")
	parts := strings.SplitN(path, "/", 2)
	taskID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	var task models.CrawlerTask
	if err := database.DB.First(&task, "id = ?", taskID).Error; err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	switch req.Method {
	case http.MethodGet:
		writeOK(w, task)

	case http.MethodDelete:
		if task.Status == "running" {
			writeError(w, http.StatusConflict, "cannot delete running task")
			return
		}
		if err := database.DB.Delete(&task).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)

	case http.MethodPost:
		switch action {
		case "start":
			if task.Status == "running" {
				writeError(w, http.StatusConflict, "task already running")
				return
			}
			if err := database.DB.Model(&task).Updates(map[string]interface{}{
				"status": "running",
			}).Error; err != nil {
				writeError(w, http.StatusInternalServerError, "start failed")
				return
			}
			task.Status = "running"
			writeJSON(w, http.StatusAccepted, task)

		case "stop":
			if err := database.DB.Model(&task).Updates(map[string]interface{}{
				"status": "failed", "error_message": "manually stopped",
			}).Error; err != nil {
				writeError(w, http.StatusInternalServerError, "stop failed")
				return
			}
			task.Status = "failed"
			writeJSON(w, http.StatusAccepted, task)

		case "retry":
			if err := database.DB.Model(&task).Updates(map[string]interface{}{
				"status": "pending", "error_message": nil,
			}).Error; err != nil {
				writeError(w, http.StatusInternalServerError, "retry failed")
				return
			}
			task.Status = "pending"
			writeJSON(w, http.StatusAccepted, task)

		default:
			writeError(w, http.StatusNotFound, "unknown action: "+action)
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/DELETE/POST required")
	}
}

func (r *Router) handleCrawlStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	var totalN, totalCh, totalTasks, pending int64
	database.DB.Model(&models.Novel{}).Count(&totalN)
	database.DB.Model(&models.Chapter{}).Count(&totalCh)
	database.DB.Model(&models.CrawlerTask{}).Count(&totalTasks)
	database.DB.Model(&models.CrawlerTask{}).Where("status = ?", "pending").Count(&pending)

	writeOK(w, map[string]interface{}{
		"novels":        totalN,
		"chapters":      totalCh,
		"tasks_total":   totalTasks,
		"tasks_pending": pending,
	})
}

func calcPages(total, size int64) int {
	if total == 0 {
		return 0
	}
	p := int(total / size)
	if total%size > 0 {
		p++
	}
	return p
}
