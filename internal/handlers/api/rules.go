package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// rulesDir is where crawler rule JSON files are stored.
const rulesDir = "rules"

func init() { os.MkdirAll(rulesDir, 0755) }

func (r *Router) handleRulesList(w http.ResponseWriter, req *http.Request) {
	entries, _ := os.ReadDir(rulesDir)
	type RuleMeta struct {
		SourceName  string `json:"source_name"`
		BaseURL     string `json:"base_url"`
		Description string `json:"description"`
		Version     string `json:"version"`
	}
	rules := make([]RuleMeta, 0)
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			data, _ := os.ReadFile(filepath.Join(rulesDir, e.Name()))
			var rm RuleMeta
			json.Unmarshal(data, &rm)
			if rm.SourceName == "" {
				rm.SourceName = strings.TrimSuffix(e.Name(), ".json")
			}
			rules = append(rules, rm)
		}
	}
	if rules == nil { rules = []RuleMeta{} }
	writeOK(w, rules)
}

func (r *Router) handleRuleByID(w http.ResponseWriter, req *http.Request) {
	name := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/rules/")
	if name == "" { writeError(w, 400, "rule name required"); return }

	filePath := filepath.Join(rulesDir, name+".json")
	// Path traversal guard
	if strings.Contains(name, "..") || !strings.HasSuffix(filepath.Clean(filePath), ".json") {
		writeError(w, 400, "invalid rule name"); return
	}

	switch req.Method {
	case http.MethodGet:
		data, err := os.ReadFile(filePath)
		if err != nil { writeError(w, 404, "rule not found"); return }
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write(data)

	case http.MethodPut:
		var body map[string]interface{}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil { writeError(w, 400, "invalid JSON"); return }
		data, _ := json.MarshalIndent(body, "", "  ")
		if err := os.WriteFile(filePath, data, 0644); err != nil { writeError(w, 500, "save failed"); return }
		writeOK(w, map[string]string{"message": "saved"})

	case http.MethodDelete:
		if err := os.Remove(filePath); err != nil { writeError(w, 404, "rule not found"); return }
		w.WriteHeader(204)

	default:
		writeError(w, 405, "GET/PUT/DELETE required")
	}
}

func (r *Router) handleRuleTest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost { writeError(w, 405, "POST required"); return }
	var body struct {
		SourceName string `json:"source_name"`
		Section    string `json:"section"`
		TestURL    string `json:"test_url"`
		Keyword    string `json:"keyword"`
		BookID     string `json:"book_id"`
		ChapterURL string `json:"chapter_url"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil { writeError(w, 400, "invalid JSON"); return }
	// Rule test is a future feature — for now return placeholder
	writeOK(w, map[string]interface{}{
		"success": true, "url": body.TestURL, "total": 0, "results": []interface{}{},
	})
}
