package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

// ── Sources ──────────────────────────────────────────────────────────────────

func (r *Router) handleSources(w http.ResponseWriter, req *http.Request) {
	writeOK(w, []map[string]string{
		{"source_name": "23qb", "base_url": "https://www.23qb.net", "description": "铅笔小说 (23qb.net)"},
	})
}

// ── Crawl trigger ────────────────────────────────────────────────────────────

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
	if body.Mode == "" {
		body.Mode = "direct"
	}

	// Verify novel exists
	var novel models.Novel
	if err := database.DB.First(&novel, "id = ?", body.NovelID).Error; err != nil {
		writeError(w, http.StatusNotFound, "novel not found")
		return
	}

	task := models.CrawlerTask{NovelID: body.NovelID, Status: "pending"}
	if err := database.DB.Create(&task).Error; err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusAccepted, task)
}

// ── Tasks list ───────────────────────────────────────────────────────────────

func (r *Router) handleCrawlTasks(w http.ResponseWriter, req *http.Request) {
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

	pages := 0
	if total > 0 {
		pages = int(total) / size
		if int(total)%size > 0 {
			pages++
		}
	}
	writeOK(w, map[string]interface{}{
		"items": tasks, "total": total, "page": page, "size": size, "pages": pages,
	})
}

func (r *Router) handleCrawlTaskByID(w http.ResponseWriter, req *http.Request) {
	taskID := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/crawler/tasks/")
	// Strip any trailing path
	if idx := strings.Index(taskID, "/"); idx != -1 {
		taskID = taskID[:idx]
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
		database.DB.Delete(&task)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost:
		// task actions: /tasks/{id}/start, /tasks/{id}/stop, /tasks/{id}/retry
		action := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/crawler/tasks/"+taskID+"/")
		switch action {
		case "start":
			task.Status = "running"
			database.DB.Save(&task)
			writeJSON(w, http.StatusAccepted, task)
		case "stop":
			task.Status = "failed"
			task.ErrorMessage.Scan("manually stopped")
			database.DB.Save(&task)
			writeJSON(w, http.StatusAccepted, task)
		case "retry":
			task.Status = "pending"
			task.ErrorMessage.Valid = false
			database.DB.Save(&task)
			writeJSON(w, http.StatusAccepted, task)
		default:
			writeError(w, http.StatusNotFound, "unknown action")
		}
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/DELETE/POST required")
	}
}

// ── Stats ────────────────────────────────────────────────────────────────────

func (r *Router) handleCrawlStats(w http.ResponseWriter, req *http.Request) {
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
		"words_total":   0,
	})
}
