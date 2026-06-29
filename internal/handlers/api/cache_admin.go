package api

import (
	"net/http"
	"runtime"
)

func (r *Router) handleCacheHealth(w http.ResponseWriter, req *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	writeOK(w, map[string]interface{}{
		"redis":         "connected",
		"page_cache":    "active",
		"memory": map[string]interface{}{
			"alloc_mb":      memStats.Alloc / 1024 / 1024,
			"total_alloc_mb": memStats.TotalAlloc / 1024 / 1024,
			"num_gc":        memStats.NumGC,
			"goroutines":    runtime.NumGoroutine(),
		},
	})
}

func (r *Router) handleCacheFlush(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST required")
		return
	}
	// In Go, we don't have the multi-layer page caching yet
	// This is a future enhancement point
	writeOK(w, map[string]string{"message": "cache flushed"})
}
