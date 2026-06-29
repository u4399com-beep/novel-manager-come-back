package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleLinkRings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		var rings []models.LinkRing
		database.DB.Preload("Targets").Order("created_at DESC").Limit(100).Find(&rings)
		writeOK(w, rings)
	case http.MethodPost:
		var ring models.LinkRing
		if err := json.NewDecoder(req.Body).Decode(&ring); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if ring.Name == "" {
			writeError(w, http.StatusBadRequest, "name required")
			return
		}
		if err := database.DB.Create(&ring).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "create failed")
			return
		}
		writeJSON(w, http.StatusCreated, ring)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/POST required")
	}
}

func (r *Router) handleLinkRingByID(w http.ResponseWriter, req *http.Request) {
	ringID := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/link-rings/")

	switch req.Method {
	case http.MethodGet:
		var ring models.LinkRing
		if err := database.DB.Preload("Targets").First(&ring, "id = ?", ringID).Error; err != nil {
			writeError(w, http.StatusNotFound, "link ring not found")
			return
		}
		writeOK(w, ring)
	case http.MethodPut:
		var updates map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := database.DB.Model(&models.LinkRing{}).Where("id = ?", ringID).Updates(updates).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "update failed")
			return
		}
		var ring models.LinkRing
		database.DB.Preload("Targets").First(&ring, "id = ?", ringID)
		writeOK(w, ring)
	case http.MethodDelete:
		if err := database.DB.Delete(&models.LinkRing{}, "id = ?", ringID).Error; err != nil {
			writeError(w, http.StatusInternalServerError, "delete failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "GET/PUT/DELETE required")
	}
}
