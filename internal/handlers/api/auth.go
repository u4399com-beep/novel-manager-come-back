package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/handlers/middleware"
	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

func (r *Router) handleRegister(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	body.Email = strings.TrimSpace(body.Email)
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}
	if body.Email != "" && !strings.Contains(body.Email, "@") {
		writeError(w, http.StatusBadRequest, "invalid email format")
		return
	}

	user, err := services.RegisterUser(req.Context(), body.Username, body.Email, body.Password)
	if err != nil {
		switch err {
		case services.ErrUserExists:
			writeError(w, http.StatusConflict, "user already exists")
		case services.ErrPasswordTooShort:
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		default:
			writeError(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}

	token, err := services.CreateAccessToken(r.cfg, user.ID, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token creation failed")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"access_token": token, "token_type": "bearer", "user": user,
	})
}

func (r *Router) handleLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	user, err := services.AuthenticateUser(req.Context(), body.Username, body.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := services.CreateAccessToken(r.cfg, user.ID, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token creation failed")
		return
	}

	writeOK(w, map[string]interface{}{
		"access_token": token, "token_type": "bearer", "user": user,
	})
}

func (r *Router) handleMe(w http.ResponseWriter, req *http.Request) {
	userID, _ := req.Context().Value(middleware.UserIDKey).(string)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if req.Method == http.MethodGet {
		user, err := services.GetUserByID(req.Context(), userID)
		if err != nil {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeOK(w, user)
		return
	}
	if req.Method == http.MethodPut {
		var updates map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		user, err := services.UpdateUser(req.Context(), userID, updates)
		if err != nil {
			if err == services.ErrPasswordTooShort {
				writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			} else {
				writeError(w, http.StatusInternalServerError, "update failed")
			}
			return
		}
		writeOK(w, user)
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "GET or PUT required")
}

func (r *Router) handleSearch(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	q := req.URL.Query().Get("q")
	searchType := req.URL.Query().Get("type")
	if searchType == "" {
		searchType = "novel"
	}

	if q == "" {
		writeOK(w, map[string]interface{}{
			"items": []interface{}{}, "total": 0, "page": 1, "size": 20, "pages": 0,
		})
		return
	}

	if searchType == "novel" {
		result, err := services.ListNovels(req.Context(), services.NovelListParams{
			Page: 1, Size: 20, Search: q, SortBy: "updated_at", SortDir: "desc",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "search failed")
			return
		}
		writeOK(w, result)
		return
	}
	writeOK(w, map[string]interface{}{"items": []interface{}{}, "total": 0})
}
