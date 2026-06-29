package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleSites(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		var sites []models.Site
		database.DB.Order("created_at DESC").Limit(200).Find(&sites)
		writeOK(w, sites)
	case http.MethodPost:
		var site models.Site
		if err := json.NewDecoder(req.Body).Decode(&site); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if site.Name == "" || site.Domain == "" {
			writeError(w, http.StatusBadRequest, "name and domain required")
			return
		}
		if err := database.DB.Create(&site).Error; err != nil {
			writeError(w, http.StatusConflict, "site domain likely exists")
			return
		}
		writeJSON(w, http.StatusCreated, site)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
	}
}

func (r *Router) handleSiteByID(w http.ResponseWriter, req *http.Request) {
	siteID := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/sites/")

	switch req.Method {
	case http.MethodGet:
		var site models.Site
		if err := database.DB.First(&site, "id = ?", siteID).Error; err != nil {
			writeError(w, http.StatusNotFound, "site not found")
			return
		}
		writeOK(w, site)
	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := database.DB.Model(&models.Site{}).Where("id = ?", siteID).Updates(updates).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		var site models.Site
		database.DB.First(&site, "id = ?", siteID)
		writeOK(w, site)
	case http.MethodDelete:
		if err := database.DB.Delete(&models.Site{}, "id = ?", siteID).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/PUT/DELETE required")
	}
}
