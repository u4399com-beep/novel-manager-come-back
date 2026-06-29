// Package database provides MySQL connection pool management.
package database

import (
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

// Init opens the database connection pool. Safe to call multiple times
// (subsequent calls are no-ops thanks to sync.Once).
func Init(cfg *config.Config) error {
	var initErr error
	once.Do(func() {
		initErr = initDB(cfg)
	})
	return initErr
}

func initDB(cfg *config.Config) error {
	var dialector gorm.Dialector

	if strings.Contains(cfg.DatabaseURL, "sqlite") {
		dialector = sqlite.Open(cfg.DatabaseURL)
	} else {
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

	// Connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.DBPoolSize + cfg.DBMaxOverflow)
	sqlDB.SetMaxIdleConns(cfg.DBPoolSize)
	sqlDB.SetConnMaxLifetime(cfg.DBPoolRecycle)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)

	// SQLite pragmas
	if strings.Contains(cfg.DatabaseURL, "sqlite") {
		for _, pragma := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA busy_timeout=20000",
			"PRAGMA foreign_keys=ON",
		} {
			db.Exec(pragma)
		}
	}

	mu.Lock()
	DB = db
	mu.Unlock()

	log.Println("Database connected successfully")
	return nil
}

// WithContext returns *gorm.DB with context (with nil guard).
func WithContext(ctx interface{}) *gorm.DB {
	mu.RLock()
	defer mu.RUnlock()
	if DB == nil {
		log.Println("WARNING: database.DB is nil — Init() not called or failed")
		return nil
	}
	return DB
}
