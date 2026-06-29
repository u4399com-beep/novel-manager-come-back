package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleCategories(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		var cats []models.Category
		database.DB.Order("sort_order ASC").Find(&cats)
		writeOK(w, cats)
	case http.MethodPost:
		var body struct {
			Name      string `json:"name"`
			Slug      string `json:"slug"`
			SortOrder int    `json:"sort_order"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if body.Name == "" || body.Slug == "" {
			writeError(w, http.StatusBadRequest, "name and slug required")
			return
		}
		cat := models.Category{Name: body.Name, Slug: body.Slug, SortOrder: body.SortOrder}
		if err := database.DB.Create(&cat).Error; err != nil {
			writeError(w, http.StatusConflict, "category exists")
			return
		}
		writeJSON(w, http.StatusCreated, cat)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
	}
}

func (r *Router) handleCategoryByID(w http.ResponseWriter, req *http.Request) {
	idStr := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/categories/")

	switch req.Method {
	case http.MethodGet:
		var cat models.Category
		if err := database.DB.First(&cat, idStr).Error; err != nil {
			writeError(w, http.StatusNotFound, "category not found")
			return
		}
		writeOK(w, cat)
	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := database.DB.Model(&models.Category{}).Where("id = ?", idStr).Updates(updates).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		var cat models.Category
		database.DB.First(&cat, idStr)
		writeOK(w, cat)
	case http.MethodDelete:
		if err := database.DB.Delete(&models.Category{}, idStr).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/PUT/DELETE required")
	}
}
