// Package middleware provides HTTP middleware for auth, rate limiting, CORS, and recovery.
package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

type contextKey string

const (
	UserIDKey   contextKey = "user_id"
	UserRoleKey contextKey = "user_role"
)

// MaxBodySize limits JSON request bodies to prevent memory exhaustion.
const MaxBodySize = 1 << 20 // 1 MB

// ── Auth ─────────────────────────────────────────────────────────────────────

// AuthRequired validates JWT Bearer tokens.
func AuthRequired(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := services.ParseAccessToken(cfg, tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			userID, _ := claims["sub"].(string)
			role, _ := claims["role"].(string)
			if userID == "" {
				http.Error(w, `{"error":"invalid token claims"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, UserRoleKey, role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ── Rate Limiter ─────────────────────────────────────────────────────────────

type RateLimit struct {
	maxRequests int
	window      time.Duration
	mu          sync.Mutex
	buckets     map[string]*bucket
	maxBuckets  int // hard cap to prevent memory exhaustion
}

type bucket struct {
	tokens   int
	lastSeen time.Time
}

func NewRateLimit(maxRequests int, window time.Duration) *RateLimit {
	rl := &RateLimit{
		maxRequests: maxRequests,
		window:      window,
		buckets:     make(map[string]*bucket),
		maxBuckets:  200000,
	}
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.cleanup()
		}
	}()
	return rl
}

func (rl *RateLimit) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-rl.window)
	for ip, b := range rl.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(rl.buckets, ip)
		}
	}
	// Hard eviction if under DDoS: remove oldest entries
	if len(rl.buckets) > rl.maxBuckets {
		excess := len(rl.buckets) - rl.maxBuckets/2
		for ip := range rl.buckets {
			if excess <= 0 {
				break
			}
			delete(rl.buckets, ip)
			excess--
		}
		log.Printf("RateLimit: hard eviction triggered (%d buckets → %d)", len(rl.buckets)+rl.maxBuckets/2, len(rl.buckets))
	}
}

func (rl *RateLimit) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		rl.mu.Lock()
		b, ok := rl.buckets[ip]
		if !ok || time.Since(b.lastSeen) > rl.window {
			b = &bucket{tokens: rl.maxRequests - 1, lastSeen: time.Now()}
			rl.buckets[ip] = b
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}
		b.lastSeen = time.Now()
		if b.tokens <= 0 {
			rl.mu.Unlock()
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		b.tokens--
		rl.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

// ── Body Size Limiter ──────────────────────────────────────────────────────

// LimitBodySize returns middleware that rejects oversized request bodies.
func LimitBodySize(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxSize {
				http.Error(w, `{"error":"request body too large"}`, http.StatusRequestEntityTooLarge)
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
			origin := r.Header.Get("Origin")
			if allowSet[origin] {
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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	return host
}
