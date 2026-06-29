// Package config provides typed application configuration from environment.
// Defaults are safe for development; production MUST override SECRET_KEY and DATABASE_URL.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application settings.
type Config struct {
	DatabaseURL   string
	DBPoolSize    int
	DBMaxOverflow int
	DBPoolRecycle time.Duration

	SecretKey            string
	AccessTokenExpireMin int

	RedisURL  string
	StaticDir string

	CORSOrigins []string

	AppTitle   string
	AppVersion string
	APIPrefix  string
	ServerPort string

	CrawlerRequestDelay float64
	CrawlerTimeout      int
	CrawlerConcurrency  int

	LibreTranslateURL string

	// Derived
	IsDevelopment bool
}

// Load reads configuration from environment with sensible defaults.
func Load() *Config {
	cfg := &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "mysql://root:password@localhost:3306/novel_come_back?charset=utf8mb4&parseTime=true"),
		DBPoolSize:    getEnvInt("DB_POOL_SIZE", 20),
		DBMaxOverflow: getEnvInt("DB_MAX_OVERFLOW", 10),
		DBPoolRecycle: time.Duration(getEnvInt("DB_POOL_RECYCLE", 3600)) * time.Second,

		SecretKey:            getEnv("SECRET_KEY", "change-me-in-production-use-a-strong-random-key"),
		AccessTokenExpireMin: getEnvInt("ACCESS_TOKEN_EXPIRE_MINUTES", 480),

		RedisURL:  getEnv("REDIS_URL", "redis://localhost:6380/0"),
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

	cfg.IsDevelopment = os.Getenv("APP_ENV") == "development" ||
		strings.Contains(cfg.DatabaseURL, "localhost") ||
		strings.Contains(cfg.DatabaseURL, "127.0.0.1")

	return cfg
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
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "[") {
		v = strings.Trim(v, "[]")
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "\"'")
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) > 0 {
		return result
	}
	return fallback
}
