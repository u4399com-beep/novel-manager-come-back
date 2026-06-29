// Package database provides connection pool management for MySQL and SQLite.
package database

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
)

var (
	DB   *gorm.DB
	once sync.Once
	mu   sync.RWMutex
)

// Init initializes the database connection pool. Thread-safe via sync.Once.
func Init(cfg *config.Config) error {
	var initErr error
	once.Do(func() {
		initErr = initDB(cfg)
	})
	return initErr
}

func initDB(cfg *config.Config) error {
	isSQLite := false
	var dialector gorm.Dialector

	switch {
	case strings.Contains(cfg.DatabaseURL, "sqlite"), strings.Contains(cfg.DatabaseURL, "sqlite3"),
		!strings.Contains(cfg.DatabaseURL, "://"):
		// SQLite: plain file path, file: prefix, or sqlite:// scheme
		dsn := cfg.DatabaseURL
		dsn = strings.TrimPrefix(dsn, "sqlite://")
		dsn = strings.TrimPrefix(dsn, "sqlite3://")
		dialector = sqlite.Open(dsn)
		isSQLite = true
	case strings.Contains(cfg.DatabaseURL, "mysql"), strings.Contains(cfg.DatabaseURL, "mariadb"):
		dialector = mysql.Open(cfg.DatabaseURL)
	case strings.Contains(cfg.DatabaseURL, "postgres"):
		return fmt.Errorf("database: PostgreSQL not yet supported")
	default:
		dialector = mysql.Open(cfg.DatabaseURL)
	}

	gormCfg := &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Warn),
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	}

	var db *gorm.DB
	var err error
	for retry := 0; retry < 3; retry++ {
		db, err = gorm.Open(dialector, gormCfg)
		if err == nil {
			break
		}
		log.Printf("DB connection attempt %d failed: %v", retry+1, err)
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return fmt.Errorf("database: failed after 3 retries: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBPoolSize + cfg.DBMaxOverflow)
	sqlDB.SetMaxIdleConns(cfg.DBPoolSize)
	sqlDB.SetConnMaxLifetime(cfg.DBPoolRecycle)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// SQLite pragmas
	if isSQLite {
		for _, pragma := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA busy_timeout=20000",
			"PRAGMA foreign_keys=ON",
		} {
			if err := db.Exec(pragma).Error; err != nil {
				log.Printf("SQLite pragma %s failed: %v", pragma, err)
			}
		}
	}

	mu.Lock()
	DB = db
	mu.Unlock()

	log.Println("Database connected successfully")
	return nil
}

// Ctx returns *gorm.DB with context (nil-safe).
func Ctx(ctx context.Context) *gorm.DB {
	mu.RLock()
	defer mu.RUnlock()
	if DB == nil {
		return nil
	}
	return DB.WithContext(ctx)
}
