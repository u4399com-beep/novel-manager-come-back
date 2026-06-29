package api

import (
	"net/http"
	"runtime"
)

func (r *Router) handleCacheHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET required")
		return
	}
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	writeOK(w, map[string]interface{}{
		"redis":      "not_configured",
		"page_cache": "active (in-memory LRU)",
		"memory": map[string]interface{}{
			"alloc_mb":       memStats.Alloc / 1024 / 1024,
			"total_alloc_mb": memStats.TotalAlloc / 1024 / 1024,
			"num_gc":         memStats.NumGC,
			"goroutines":     runtime.NumGoroutine(),
		},
	})
}

func (r *Router) handleCacheFlush(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	writeOK(w, map[string]string{"message": "ok — no persistent cache configured (future feature)"})
}
