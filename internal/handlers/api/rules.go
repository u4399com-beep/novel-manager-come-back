package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

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
	path := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/rules/")
	parts := strings.SplitN(path, "/", 2)
	name := parts[0]
	action := ""
	if len(parts) > 1 { action = parts[1] }

	// Export
	if action == "export" {
		r.exportRule(w, req, name)
		return
	}

	if name == "" { writeError(w, 400, "rule name required"); return }
	filePath := filepath.Join(rulesDir, name+".json")
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

// ── Export / Import ────────────────────────────────────────────────────────

func (r *Router) exportRule(w http.ResponseWriter, req *http.Request, name string) {
	filePath := filepath.Join(rulesDir, name+".json")
	data, err := os.ReadFile(filePath)
	if err != nil { writeError(w, 404, "rule not found"); return }
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", name))
	w.Write(data)
}

func (r *Router) handleRuleImport(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost { writeError(w, 405, "POST required"); return }

	// Parse multipart form (file upload) or raw JSON body
	contentType := req.Header.Get("Content-Type")
	if strings.Contains(contentType, "multipart") {
		req.ParseMultipartForm(10 << 20)
		file, header, err := req.FormFile("file")
		if err != nil { writeError(w, 400, "file required"); return }
		defer file.Close()
		data, _ := io.ReadAll(file)
		var body map[string]interface{}
		if err := json.Unmarshal(data, &body); err != nil { writeError(w, 400, "invalid JSON in file"); return }
		sourceName, _ := body["source_name"].(string)
		if sourceName == "" { sourceName = strings.TrimSuffix(header.Filename, ".json") }
		if strings.Contains(sourceName, "..") || strings.Contains(sourceName, "/") || strings.Contains(sourceName, "\\") { writeError(w, 400, "invalid source_name"); return }
		savePath := filepath.Join(rulesDir, sourceName+".json")
		formatted, _ := json.MarshalIndent(body, "", "  ")
		os.WriteFile(savePath, formatted, 0644)
		writeOK(w, map[string]string{"message": "imported " + sourceName})
		return
	}

	// Raw JSON body import
	var body map[string]interface{}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil { writeError(w, 400, "invalid JSON"); return }
	sourceName, _ := body["source_name"].(string)
	if sourceName == "" { writeError(w, 400, "source_name required"); return }
	if strings.Contains(sourceName, "..") || strings.Contains(sourceName, "/") || strings.Contains(sourceName, "\\") { writeError(w, 400, "invalid source_name"); return }
	savePath := filepath.Join(rulesDir, sourceName+".json")
	formatted, _ := json.MarshalIndent(body, "", "  ")
	os.WriteFile(savePath, formatted, 0644)
	writeOK(w, map[string]string{"message": "imported " + sourceName})
}

// ── Rule Test ──────────────────────────────────────────────────────────────

type ruleField struct {
	Selector     string `json:"selector"`
	Attr         string `json:"attr"`
	Fallback     string `json:"fallback"`
	FallbackAttr string `json:"fallback_attr"`
	FallbackText bool   `json:"fallback_text"`
	Transform    string `json:"transform"`
}

type ruleSection struct {
	Container      string               `json:"container"`
	Fields         map[string]ruleField `json:"fields"`
	RemoveElements []string             `json:"remove_elements"`
	ContentClean   bool                 `json:"content_clean"`
}

type testRule struct {
	Selectors map[string]ruleSection `json:"selectors"`
	BaseURL   string                 `json:"base_url"`
}

func (r *Router) handleRuleTest(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost { writeError(w, 405, "POST required"); return }
	var body struct {
		SourceName string `json:"source_name"`
		Section    string `json:"section"`
		TestURL    string `json:"test_url"`
		Keyword    string `json:"keyword"`
		BookID     string `json:"book_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil { writeError(w, 400, "invalid JSON"); return }
	if body.SourceName == "" { writeError(w, 400, "source_name required"); return }

	// Load rule
	filePath := filepath.Join(rulesDir, body.SourceName+".json")
	ruleData, err := os.ReadFile(filePath)
	if err != nil { writeError(w, 404, "rule not found"); return }

	var rule testRule
	if err := json.Unmarshal(ruleData, &rule); err != nil { writeError(w, 400, "invalid rule JSON: "+err.Error()); return }

	// Build URL
	testURL := body.TestURL
	if testURL == "" && body.Section == "catalog" && body.BookID != "" {
		testURL = fmt.Sprintf("%s/book/%s/catalog", rule.BaseURL, body.BookID)
	}
	if testURL == "" && body.Section == "search" && body.Keyword != "" {
		sel, ok := rule.Selectors["search"]
		if ok && sel.Container != "" {
			testURL = rule.BaseURL + "/search?keyword=" + body.Keyword
		}
	}
	if testURL == "" {
		writeError(w, 400, "test_url required (or book_id for catalog, keyword for search)")
		return
	}

	// Fetch page
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(testURL)
	if err != nil { writeError(w, 500, "fetch failed: "+err.Error()); return }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { writeError(w, 500, fmt.Sprintf("HTTP %d from source", resp.StatusCode)); return }

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil { writeError(w, 500, "parse HTML failed: "+err.Error()); return }

	// Apply rule
	sel, ok := rule.Selectors[body.Section]
	if !ok {
		// List available sections
		sections := make([]string, 0, len(rule.Selectors))
		for k := range rule.Selectors { sections = append(sections, k) }
		writeOK(w, map[string]interface{}{
			"success": false, "error": fmt.Sprintf("section '%s' not found. Available: %v", body.Section, sections),
			"url": testURL,
		})
		return
	}

	results := make([]map[string]string, 0)

	// Find container elements
	containerSel := sel.Container
	if containerSel == "" {
		// Single-item mode (novel_info)
		item := make(map[string]string)
		for fieldName, field := range sel.Fields {
			item[fieldName] = extractField(doc.Selection, field)
		}
		if len(item) > 0 { results = append(results, item) }
	} else {
		// Repeating container mode (catalog, search)
		// Split by comma for multiple selectors
		containers := strings.Split(containerSel, ",")
		for _, cont := range containers {
			cont = strings.TrimSpace(cont)
			doc.Find(cont).Each(func(i int, s *goquery.Selection) {
				item := make(map[string]string)
				for fieldName, field := range sel.Fields {
					item[fieldName] = extractField(s, field)
				}
				if len(item) > 0 {
					results = append(results, item)
				}
			})
		}
	}

	writeOK(w, map[string]interface{}{
		"success": true,
		"url":     testURL,
		"section": body.Section,
		"total":   len(results),
		"results": results,
	})
}

func extractField(s *goquery.Selection, field ruleField) string {
	var val string

	// Try primary selector
	if field.Selector != "" {
		sel := s.Find(field.Selector)
		if sel.Length() > 0 {
			if field.Attr != "" {
				val, _ = sel.Attr(field.Attr)
			} else {
				val = strings.TrimSpace(sel.First().Text())
			}
		}
	}

	// Fallback
	if val == "" && field.Fallback != "" {
		fbSel := s.Find(field.Fallback)
		if fbSel.Length() > 0 {
			if field.FallbackAttr != "" {
				val, _ = fbSel.Attr(field.FallbackAttr)
			} else {
				val = strings.TrimSpace(fbSel.First().Text())
			}
		}
	}

	// Fallback text
	if val == "" && field.FallbackText {
		val = strings.TrimSpace(s.Text())
	}

	// Apply transform
	if val != "" && field.Transform != "" {
		if strings.HasPrefix(field.Transform, "regex:") {
			pattern := strings.TrimPrefix(field.Transform, "regex:")
			if re, err := regexp.Compile(pattern); err == nil {
				if matches := re.FindStringSubmatch(val); len(matches) > 1 {
					val = matches[1]
				}
			}
		}
		if field.Transform == "absolute_url" {
			// handled by caller
		}
	}

	return val
}
