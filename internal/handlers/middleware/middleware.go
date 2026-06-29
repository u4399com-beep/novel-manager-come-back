// Package middleware provides HTTP middleware for auth, rate limiting, CORS, and recovery.
package middleware

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
	MaxBodySize int64      = 1 << 20
)

const (
	maxRateBuckets    = 100000
	rateCleanInterval = 5 * time.Minute
	defaultRateLimit  = 100
	defaultRateWindow = 60
)

// ── Auth (with deactivated-user check) ─────────────────────────────────────

func AuthRequired(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ah := r.Header.Get("Authorization")
			if ah == "" || !strings.HasPrefix(ah, "Bearer ") {
				http.Error(w, `{"error":"auth required"}`, http.StatusUnauthorized)
				return
			}
			claims, err := services.ParseAccessToken(cfg, strings.TrimPrefix(ah, "Bearer "))
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			userID, _ := claims["sub"].(string)
			role, _ := claims["role"].(string)
			if userID == "" {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			// Verify user is still active (not deactivated after token issuance)
			var user models.User
			if err := database.DB.Where("id = ? AND is_active = ?", userID, true).First(&user).Error; err != nil {
				http.Error(w, `{"error":"user deactivated"}`, http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ── Rate Limiter (sharded token bucket) ───────────────────────────────────

const shardCount = 64

type RateLimit struct {
	rate    float64
	burst   int
	shards  [shardCount]*rateShard
	closeCh chan struct{}
}

type rateShard struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
}

type tokenBucket struct {
	tokens   float64
	lastSeen time.Time
}

func NewRateLimit(maxRequests, windowSecs int) *RateLimit {
	rl := &RateLimit{
		rate:    float64(maxRequests) / float64(windowSecs),
		burst:   maxRequests,
		closeCh: make(chan struct{}),
	}
	for i := range rl.shards {
		rl.shards[i] = &rateShard{buckets: make(map[string]*tokenBucket)}
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimit) cleanupLoop() {
	t := time.NewTicker(rateCleanInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			rl.cleanup()
		case <-rl.closeCh:
			return
		}
	}
}

func (rl *RateLimit) cleanup() {
	cutoff := time.Now().Add(-rateCleanInterval)
	for _, s := range rl.shards {
		s.mu.Lock()
		for ip, b := range s.buckets {
			if b.lastSeen.Before(cutoff) {
				delete(s.buckets, ip)
			}
		}
		// Hard eviction
		if len(s.buckets) > maxRateBuckets/shardCount {
			excess := len(s.buckets) - maxRateBuckets/shardCount/2
			for ip := range s.buckets {
				if excess <= 0 {
					break
				}
				delete(s.buckets, ip)
				excess--
			}
		}
		s.mu.Unlock()
	}
}

func (rl *RateLimit) Stop() {
	close(rl.closeCh)
}

func (rl *RateLimit) allow(ip string) bool {
	idx := fnvHash(ip) % shardCount
	s := rl.shards[idx]

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	b, ok := s.buckets[ip]
	if !ok {
		s.buckets[ip] = &tokenBucket{tokens: float64(rl.burst) - 1, lastSeen: now}
		return true
	}
	// Refill
	elapsed := now.Sub(b.lastSeen).Seconds()
	b.tokens += elapsed * rl.rate
	if b.tokens > float64(rl.burst) {
		b.tokens = float64(rl.burst)
	}
	b.lastSeen = now
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

func (rl *RateLimit) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if !rl.allow(clientIP(r)) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ── Body Size Limiter ──────────────────────────────────────────────────────

func LimitBodySize(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxSize {
				http.Error(w, `{"error":"body too large"}`, http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxSize)
			next.ServeHTTP(w, r)
		})
	}
}

// ── CORS ─────────────────────────────────────────────────────────────────────

func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowSet[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); allowSet[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Requested-With")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ── Recoverer ────────────────────────────────────────────────────────────────

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %v — %s %s", rec, r.Method, r.URL.Path)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ── RequestID ────────────────────────────────────────────────────────────────

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("%x", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", reqID)
		next.ServeHTTP(w, r)
	})
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// clientIP extracts the real client IP with IPv6 support.
// Uses net.SplitHostPort for proper address parsing.
// Proxy headers are only trusted when TRUST_PROXY=true is set.
func clientIP(r *http.Request) string {
	// Only trust proxy headers if explicitly enabled
	if trustProxyHeaders() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if idx := strings.IndexByte(xff, ','); idx != -1 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// Fallback for malformed RemoteAddr
		return r.RemoteAddr
	}
	return host
}

var proxyTrustOnce sync.Once
var proxyTrusted bool

func trustProxyHeaders() bool {
	proxyTrustOnce.Do(func() {
		v := strings.ToLower(strings.TrimSpace(os.Getenv("TRUST_PROXY")))
		proxyTrusted = v == "true" || v == "1"
	})
	return proxyTrusted
}

func fnvHash(s string) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	return h
}
