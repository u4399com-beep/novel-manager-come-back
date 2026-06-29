package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

// routeChaptersPrefix handles /api/v1/novels/{novelId}/chapters/...
func (r *Router) routeChaptersPrefix(w http.ResponseWriter, req *http.Request, novelID string, parts []string) {
	// parts[0]=novelID, parts[1]="chapters", parts[2]=rest (may be empty or sub-path)

	// /api/v1/novels/{id}/chapters (no chapter ID)
	if len(parts) < 3 || parts[2] == "" {
		switch req.Method {
		case http.MethodGet:
			r.listChapters(w, req, novelID)
		case http.MethodPost:
			r.createChapter(w, req, novelID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
		}
		return
	}

	remaining := parts[2]
	// /api/v1/novels/{id}/chapters/batch
	if remaining == "batch" {
		switch req.Method {
		case http.MethodPost:
			r.batchCreateChapters(w, req, novelID)
		case http.MethodDelete:
			r.batchDeleteChapters(w, req, novelID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "POST/DELETE required")
		}
		return
	}
	// /api/v1/novels/{id}/chapters/reorder
	if remaining == "reorder" {
		if req.Method == http.MethodPut {
			r.reorderChapters(w, req, novelID)
		} else {
			writeError(w, http.StatusMethodNotAllowed, "PUT required")
		}
		return
	}

	// /api/v1/novels/{id}/chapters/{chapter_id}
	chapterID := remaining
	switch req.Method {
	case http.MethodGet:
		r.getChapter(w, req, chapterID)
	case http.MethodPut:
		r.updateChapter(w, req, chapterID)
	case http.MethodDelete:
		r.deleteChapter(w, req, chapterID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/PUT/DELETE required")
	}
}

func (r *Router) listChapters(w http.ResponseWriter, req *http.Request, novelID string) {
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(req.URL.Query().Get("size"))
	if size < 1 || size > 200 {
		size = 50
	}

	chapters, total, err := services.GetChapters(req.Context(), novelID, page, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chapters")
		return
	}
	pages := total / int64(size)
	if total%int64(size) > 0 {
		pages++
	}
	writeOK(w, map[string]interface{}{
		"items": chapters, "total": total, "page": page, "size": size, "pages": pages,
	})
}

func (r *Router) createChapter(w http.ResponseWriter, req *http.Request, novelID string) {
	var body struct {
		Title       string `json:"title"`
		Content     string `json:"content"`
		SourceURL   string `json:"source_url"`
		SortOrder   int    `json:"sort_order"`
		IsPublished *bool  `json:"is_published"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	published := true
	if body.IsPublished != nil {
		published = *body.IsPublished
	}

	ch, err := services.CreateChapter(req.Context(), novelID, body.Title, body.Content, body.SourceURL, body.SortOrder, published)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chapter")
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (r *Router) getChapter(w http.ResponseWriter, req *http.Request, chapterID string) {
	ch, err := services.GetChapter(req.Context(), chapterID)
	if err != nil {
		writeError(w, http.StatusNotFound, "chapter not found")
		return
	}
	content, _ := services.GetChapterContent(ch)
	writeOK(w, map[string]interface{}{
		"id": ch.ID, "novel_id": ch.NovelID, "title": ch.Title,
		"content": content, "content_file": ch.ContentFile,
		"volume": ch.Volume, "sort_order": ch.SortOrder,
		"word_count": ch.WordCount, "source_url": ch.SourceURL,
		"is_published": ch.IsPublished,
		"created_at":   ch.CreatedAt, "updated_at": ch.UpdatedAt,
	})
}

func (r *Router) updateChapter(w http.ResponseWriter, req *http.Request, chapterID string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	ch, err := services.UpdateChapter(req.Context(), chapterID, updates)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update chapter")
		return
	}
	writeOK(w, ch)
}

func (r *Router) deleteChapter(w http.ResponseWriter, req *http.Request, chapterID string) {
	if err := services.DeleteChapter(req.Context(), chapterID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete chapter")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Router) batchCreateChapters(w http.ResponseWriter, req *http.Request, novelID string) {
	var body struct {
		Chapters []map[string]interface{} `json:"chapters"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(body.Chapters) == 0 || len(body.Chapters) > 500 {
		writeError(w, http.StatusBadRequest, "chapters must have 1-500 items")
		return
	}
	chapters, err := services.BatchCreateChapters(req.Context(), novelID, body.Chapters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to batch create chapters")
		return
	}
	writeJSON(w, http.StatusCreated, chapters)
}

func (r *Router) batchDeleteChapters(w http.ResponseWriter, req *http.Request, novelID string) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	deleted, err := services.BatchDeleteChapters(req.Context(), novelID, body.IDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to batch delete chapters")
		return
	}
	writeOK(w, map[string]interface{}{"deleted": deleted})
}

func (r *Router) reorderChapters(w http.ResponseWriter, req *http.Request, novelID string) {
	var body struct {
		Orders map[string]int `json:"orders"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := services.ReorderChapters(req.Context(), novelID, body.Orders); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reorder chapters")
		return
	}
	writeOK(w, map[string]string{"message": "reordered"})
}
