package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

const maxCoverSize = 10 << 20 // 10 MB

// ── Novel list/create ───────────────────────────────────────────────────────

func (r *Router) listNovels(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(q.Get("size"))
	if size < 1 || size > 100 {
		size = 20
	}

	var catID *int
	if cidStr := q.Get("category_id"); cidStr != "" {
		if cid, err := strconv.Atoi(cidStr); err == nil {
			catID = &cid
		}
	}

	result, err := services.ListNovels(req.Context(), services.NovelListParams{
		Page: page, Size: size,
		Search:     q.Get("search"),
		CategoryID: catID,
		Status:     q.Get("status"),
		SortBy:     q.Get("sort_by"),
		SortDir:    q.Get("sort_dir"),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list novels")
		return
	}
	writeOK(w, result)
}

func (r *Router) createNovel(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Author      string `json:"author"`
		Description string `json:"description"`
		SourceURL   string `json:"source_url"`
		SourceName  string `json:"source_name"`
		Status      string `json:"status"`
		CategoryIDs []int  `json:"category_ids"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	if body.Status == "" {
		body.Status = "ongoing"
	}

	novel, err := services.CreateNovel(req.Context(), body.Title, body.Author, body.Description,
		body.SourceURL, body.SourceName, body.Status, body.CategoryIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create novel")
		return
	}
	writeJSON(w, http.StatusCreated, novel)
}

// ── Route /api/v1/novels/{id}/... ───────────────────────────────────────────

func (r *Router) routeNovelsPrefix(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/novels/")
	if path == "" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	parts := strings.SplitN(path, "/", 3)
	novelID := parts[0]

	// /api/v1/novels/{id} (no sub-path)
	if len(parts) == 1 || parts[1] == "" {
		switch req.Method {
		case http.MethodGet:
			r.getNovel(w, req, novelID)
		case http.MethodPut:
			r.updateNovel(w, req, novelID)
		case http.MethodDelete:
			r.deleteNovel(w, req, novelID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "GET/PUT/DELETE required")
		}
		return
	}

	// /api/v1/novels/{id}/chapters/...
	if parts[1] == "chapters" {
		r.routeChaptersPrefix(w, req, novelID, parts)
		return
	}

	// /api/v1/novels/{id}/cover
	if parts[1] == "cover" {
		r.handleCover(w, req, novelID)
		return
	}
	// /api/v1/novels/{id}/statistics
	if parts[1] == "statistics" {
		r.handleNovelStatistics(w, req, novelID)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

func (r *Router) getNovel(w http.ResponseWriter, req *http.Request, novelID string) {
	novel, err := services.GetNovel(req.Context(), novelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "novel not found")
		return
	}
	writeOK(w, novel)
}

func (r *Router) updateNovel(w http.ResponseWriter, req *http.Request, novelID string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	var catIDs []int
	if raw, ok := updates["category_ids"]; ok {
		if arr, ok := raw.([]interface{}); ok {
			for _, v := range arr {
				if n, ok := v.(float64); ok {
					catIDs = append(catIDs, int(n))
				}
			}
		}
		delete(updates, "category_ids")
	}
	novel, err := services.UpdateNovel(req.Context(), novelID, updates, catIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update novel")
		return
	}
	writeOK(w, novel)
}

func (r *Router) deleteNovel(w http.ResponseWriter, req *http.Request, novelID string) {
	if err := services.DeleteNovel(req.Context(), novelID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete novel")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Cover upload ────────────────────────────────────────────────────────────

func (r *Router) handleCover(w http.ResponseWriter, req *http.Request, novelID string) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	if err := req.ParseMultipartForm(maxCoverSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 10MB)")
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	// Validate by magic bytes not Content-Type header
	buf := make([]byte, min(header.Size, 512))
	n, _ := file.Read(buf)
	contentType := http.DetectContentType(buf[:n])
	allowedTypes := map[string]bool{
		"image/jpeg": true, "image/png": true, "image/webp": true, "image/gif": true,
	}
	if !allowedTypes[contentType] {
		writeError(w, http.StatusBadRequest, "unsupported image type: "+contentType)
		return
	}

	// Read full content safely using io.ReadAll
	fullBuf, err := io.ReadAll(io.MultiReader(bytes.NewReader(buf[:n]), file))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read cover")
		return
	}

	url, err := services.SaveCoverImage(r.cfg, fullBuf, header.Filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save cover")
		return
	}

	if _, err := services.UpdateNovel(req.Context(), novelID, map[string]interface{}{"cover_image_url": url}, nil); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update novel cover")
		return
	}
	writeOK(w, map[string]string{"cover_image_url": url})
}

// ── Statistics ──────────────────────────────────────────────────────────────

func (r *Router) handleNovelStatistics(w http.ResponseWriter, req *http.Request, novelID string) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	stats := services.GetNovelStatistics(req.Context(), novelID)
	writeOK(w, stats)
}
