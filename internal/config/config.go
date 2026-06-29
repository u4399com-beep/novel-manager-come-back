// Package config provides typed application configuration loaded from environment.
// All defaults are safe for development; production MUST override SECRET_KEY and DATABASE_URL.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application settings.
type Config struct {
	// Database
	DatabaseURL      string
	DBPoolSize       int
	DBMaxOverflow    int
	DBPoolRecycle    time.Duration

	// Auth
	SecretKey               string
	AccessTokenExpireMin    int

	// Redis
	RedisURL string

	// Static files
	StaticDir string

	// CORS
	CORSOrigins []string

	// App metadata
	AppTitle   string
	AppVersion string
	APIPrefix  string
	ServerPort string

	// Crawler
	CrawlerRequestDelay float64
	CrawlerTimeout      int
	CrawlerConcurrency  int

	// Translation
	LibreTranslateURL string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		DatabaseURL:      getEnv("DATABASE_URL", "mysql://root:password@localhost:3306/novel_come_back?charset=utf8mb4&parseTime=true"),
		DBPoolSize:        getEnvInt("DB_POOL_SIZE", 20),
		DBMaxOverflow:     getEnvInt("DB_MAX_OVERFLOW", 10),
		DBPoolRecycle:     getEnvDuration("DB_POOL_RECYCLE", 3600*time.Second),

		SecretKey:            getEnv("SECRET_KEY", "change-me-in-production-use-a-strong-random-key"),
		AccessTokenExpireMin: getEnvInt("ACCESS_TOKEN_EXPIRE_MINUTES", 480),

		RedisURL: getEnv("REDIS_URL", "redis://localhost:6380/0"),

		StaticDir: getEnv("STATIC_DIR", "web/static"),

		CORSOrigins: getEnvSlice("CORS_ORIGINS", []string{
			"http://localhost:5173",
			"http://localhost:3000",
		}),

		AppTitle:   getEnv("APP_TITLE", "Come Back Novel CMS"),
		AppVersion: getEnv("APP_VERSION", "2.0.0"),
		APIPrefix:  getEnv("API_PREFIX", "/api/v1"),
		ServerPort: getEnv("SERVER_PORT", "8008"),

		CrawlerRequestDelay: getEnvFloat("CRAWLER_REQUEST_DELAY", 0.2),
		CrawlerTimeout:      getEnvInt("CRAWLER_TIMEOUT", 60),
		CrawlerConcurrency:  getEnvInt("CRAWLER_CONCURRENCY", 10),

		LibreTranslateURL: getEnv("LIBRETRANSLATE_URL", "http://localhost:5001/translate"),
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v + "s"); err == nil {
			return d
		}
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		// Supports JSON array format: ["a","b"] or comma-separated: a,b
		v = strings.TrimSpace(v)
		if strings.HasPrefix(v, "[") {
			v = strings.Trim(v, "[]")
		}
		parts := strings.Split(v, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.Trim(strings.TrimSpace(p), "\"")
			if p != "" {
				result = append(result, p)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return fallback
}
