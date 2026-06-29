// Package models defines domain structs independent of database driver.
package models

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

func NewUUID() string { return uuid.New().String() }

type User struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	HashedPassword string    `json:"-"`
	Role           string    `json:"role"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Category struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Novel struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	Author        string     `json:"author"`
	Description   string     `json:"description"`
	CoverImageURL string     `json:"cover_image_url"`
	SourceURL     string     `json:"source_url"`
	SourceName    string     `json:"source_name"`
	Status        string     `json:"status"`
	TotalChapters int        `json:"total_chapters"`
	Categories    []Category `json:"categories,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type Chapter struct {
	ID          string    `json:"id"`
	NovelID     string    `json:"novel_id"`
	Title       string    `json:"title"`
	Content     string    `json:"-"`
	ContentFile string    `json:"-"`
	Volume      string    `json:"volume"`
	SortOrder   int       `json:"sort_order"`
	WordCount   int       `json:"word_count"`
	SourceURL   string    `json:"source_url"`
	IsPublished bool      `json:"is_published"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CrawlerTask struct {
	ID            string         `json:"id"`
	NovelID       string         `json:"novel_id"`
	Status        string         `json:"status"`
	ChaptersFound int            `json:"chapters_found"`
	ChaptersAdded int            `json:"chapters_added"`
	ErrorMessage  sql.NullString `json:"error_message"`
	StartedAt     sql.NullTime   `json:"started_at"`
	FinishedAt    sql.NullTime   `json:"finished_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type Site struct {
	ID               string    `json:"id"`
	Domain           string    `json:"domain"`
	Name             string    `json:"name"`
	Template         string    `json:"template"`
	Offset           int       `json:"offset"`
	Description      string    `json:"description"`
	IsActive         bool      `json:"is_active"`
	TranslateEnabled bool      `json:"translate_enabled"`
	Language         string    `json:"language"`
	URLPatterns      string    `json:"url_patterns,omitempty"`
	ChapterPagination string   `json:"chapter_pagination,omitempty"`
	LinkWheel        string    `json:"link_wheel,omitempty"`
	RecommendModules string    `json:"recommend_modules,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type LinkRing struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	RingType       string           `json:"ring_type"`
	SiteID         string           `json:"site_id"`
	MaxLinks       int              `json:"max_links"`
	DisplayMode    string           `json:"display_mode"`
	LinkFormat     string           `json:"link_format"`
	OpenNewTab     bool             `json:"open_new_tab"`
	Nofollow       bool             `json:"nofollow"`
	SelectionRules string           `json:"selection_rules"`
	IsActive       bool             `json:"is_active"`
	Targets        []LinkRingTarget `json:"targets,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type LinkRingTarget struct {
	ID            string    `json:"id"`
	RingID        string    `json:"ring_id"`
	SourceSiteID  string    `json:"source_site_id"`
	SourceNovelID string    `json:"source_novel_id"`
	TargetSiteID  string    `json:"target_site_id"`
	TargetNovelID string    `json:"target_novel_id"`
	TargetURL     string    `json:"target_url"`
	AnchorText    string    `json:"anchor_text"`
	SortOrder     int       `json:"sort_order"`
	IsActive      bool      `json:"is_active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type TranslationCache struct {
	ID             string    `json:"id"`
	SourceHash     string    `json:"source_hash"`
	SourceLang     string    `json:"source_lang"`
	TargetLang     string    `json:"target_lang"`
	SourceText     string    `json:"source_text"`
	TranslatedText string    `json:"translated_text"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
