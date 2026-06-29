// Package database provides PostgreSQL connection pool via pgx.
package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
)

var (
	Pool *pgxpool.Pool
	once sync.Once
)

// Init creates the pgx connection pool. Thread-safe via sync.Once.
func Init(cfg *config.Config) error {
	var initErr error
	once.Do(func() {
		initErr = initPool(cfg)
	})
	return initErr
}

func initPool(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: parse config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.DBPoolSize)
	poolCfg.MinConns = int32(max(2, cfg.DBPoolSize/4))
	poolCfg.MaxConnLifetime = cfg.DBPoolRecycle
	poolCfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("database: create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("database: ping: %w", err)
	}

	// Auto-create tables
	if err := migrate(ctx, pool); err != nil {
		pool.Close()
		return fmt.Errorf("database: migrate: %w", err)
	}

	Pool = pool
	log.Println("PostgreSQL connected successfully")
	return nil
}

// migrate creates tables if they don't exist.
func migrate(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	return err
}

// Ctx is a convenience alias (kept for backward compat).
func Ctx(ctx context.Context) *pgxpool.Pool {
	if Pool == nil {
		return nil
	}
	return Pool
}

// ── Schema ─────────────────────────────────────────────────────────────────

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE DEFAULT '',
    hashed_password VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'user',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    slug VARCHAR(50) UNIQUE NOT NULL,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS novels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    author VARCHAR(100) DEFAULT '',
    description TEXT DEFAULT '',
    cover_image_url VARCHAR(500) DEFAULT '',
    source_url VARCHAR(500) DEFAULT '',
    source_name VARCHAR(50) DEFAULT '',
    status VARCHAR(20) DEFAULT 'ongoing',
    total_chapters INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_novels_title ON novels(title);
CREATE INDEX IF NOT EXISTS idx_novels_updated ON novels(updated_at DESC);

CREATE TABLE IF NOT EXISTS novel_categories (
    novel_id UUID REFERENCES novels(id) ON DELETE CASCADE,
    category_id INT REFERENCES categories(id) ON DELETE CASCADE,
    PRIMARY KEY (novel_id, category_id)
);

CREATE TABLE IF NOT EXISTS chapters (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    novel_id UUID NOT NULL REFERENCES novels(id) ON DELETE CASCADE,
    title VARCHAR(500) NOT NULL,
    content TEXT DEFAULT '',
    content_file VARCHAR(255) DEFAULT '',
    volume VARCHAR(100) DEFAULT '',
    sort_order INT NOT NULL DEFAULT 0,
    word_count INT DEFAULT 0,
    source_url VARCHAR(500) DEFAULT '',
    is_published BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_chapters_novel_sort ON chapters(novel_id, sort_order);
CREATE INDEX IF NOT EXISTS idx_chapters_source ON chapters(source_url);

CREATE TABLE IF NOT EXISTS crawler_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    novel_id UUID NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    chapters_found INT DEFAULT 0,
    chapters_added INT DEFAULT 0,
    error_message TEXT,
    rule_name VARCHAR(50) DEFAULT '',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_crawler_status ON crawler_tasks(status);

CREATE TABLE IF NOT EXISTS sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    template VARCHAR(50) DEFAULT 'default',
    offset_val INT DEFAULT 0,
    description TEXT DEFAULT '',
    is_active BOOLEAN DEFAULT true,
    translate_enabled BOOLEAN DEFAULT true,
    language VARCHAR(10) DEFAULT 'zh',
    url_patterns JSONB DEFAULT '{}',
    chapter_pagination JSONB DEFAULT '{}',
    link_wheel JSONB DEFAULT '{}',
    recommend_modules JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS link_rings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    ring_type VARCHAR(30) DEFAULT 'cross_site',
    site_id UUID,
    max_links INT DEFAULT 10,
    display_mode VARCHAR(20) DEFAULT 'sidebar',
    link_format VARCHAR(500) DEFAULT '',
    open_new_tab BOOLEAN DEFAULT true,
    nofollow BOOLEAN DEFAULT false,
    selection_rules JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS link_ring_targets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ring_id UUID NOT NULL REFERENCES link_rings(id) ON DELETE CASCADE,
    source_site_id UUID,
    source_novel_id UUID,
    target_site_id UUID,
    target_novel_id UUID,
    target_url VARCHAR(500) DEFAULT '',
    anchor_text VARCHAR(255) DEFAULT '',
    sort_order INT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS translation_caches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_hash VARCHAR(64) NOT NULL,
    source_lang VARCHAR(10) DEFAULT '',
    target_lang VARCHAR(10) NOT NULL,
    source_text TEXT DEFAULT '',
    translated_text TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE(source_hash, target_lang)
);
`
