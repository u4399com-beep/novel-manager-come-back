// Package models defines GORM entity structs for novel_come_back schema.
package models

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

// TimestampMixin provides auto-managed created_at / updated_at.
type TimestampMixin struct {
	CreatedAt time.Time `gorm:"autoCreateTime:milli" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime:milli" json:"updated_at"`
}

// ── User ───────────────────────────────────────────────────────────────────

type User struct {
	ID             string `gorm:"type:varchar(36);primaryKey" json:"id"`
	Username       string `gorm:"type:varchar(100);uniqueIndex;not null" json:"username"`
	Email          string `gorm:"type:varchar(255);uniqueIndex" json:"email"`
	HashedPassword string `gorm:"type:varchar(255);not null" json:"-"`
	Role           string `gorm:"type:varchar(20);default:user" json:"role"`
	IsActive       bool   `gorm:"default:true" json:"is_active"`
	TimestampMixin
}

// ── Category ───────────────────────────────────────────────────────────────

type Category struct {
	ID        int    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string `gorm:"type:varchar(50);uniqueIndex;not null" json:"name"`
	Slug      string `gorm:"type:varchar(50);uniqueIndex;not null" json:"slug"`
	SortOrder int    `gorm:"default:0" json:"sort_order"`
	TimestampMixin
}

// ── Novel ──────────────────────────────────────────────────────────────────

type Novel struct {
	ID            string     `gorm:"type:varchar(36);primaryKey" json:"id"`
	Title         string     `gorm:"type:varchar(255);not null;index" json:"title"`
	Author        string     `gorm:"type:varchar(100)" json:"author"`
	Description   string     `gorm:"type:text" json:"description"`
	CoverImageURL string     `gorm:"type:varchar(500)" json:"cover_image_url"`
	SourceURL     string     `gorm:"type:varchar(500)" json:"source_url"`
	SourceName    string     `gorm:"type:varchar(50)" json:"source_name"`
	Status        string     `gorm:"type:varchar(20);default:ongoing" json:"status"`
	TotalChapters int        `gorm:"default:0" json:"total_chapters"`
	Categories    []Category `gorm:"many2many:novel_categories" json:"categories,omitempty"`
	Chapters      []Chapter  `gorm:"foreignKey:NovelID;constraint:OnDelete:CASCADE" json:"chapters,omitempty"`
	TimestampMixin
}

// ── Chapter ────────────────────────────────────────────────────────────────

type Chapter struct {
	ID          string `gorm:"type:varchar(36);primaryKey" json:"id"`
	NovelID     string `gorm:"type:varchar(36);not null;index:idx_novel_sort,priority:1" json:"novel_id"`
	Title       string `gorm:"type:varchar(500);not null" json:"title"`
	Content     string `gorm:"type:longtext" json:"-"`
	ContentFile string `gorm:"type:varchar(255)" json:"-"`
	Volume      string `gorm:"type:varchar(100)" json:"volume"`
	SortOrder   int    `gorm:"not null;index:idx_novel_sort,priority:2" json:"sort_order"`
	WordCount   int    `gorm:"default:0" json:"word_count"`
	SourceURL   string `gorm:"type:varchar(500);index" json:"source_url"`
	IsPublished bool   `gorm:"default:true" json:"is_published"`
	TimestampMixin
}

// ── CrawlerTask ────────────────────────────────────────────────────────────

type CrawlerTask struct {
	ID            string         `gorm:"type:varchar(36);primaryKey" json:"id"`
	NovelID       string         `gorm:"type:varchar(36);not null;index" json:"novel_id"`
	Status        string         `gorm:"type:varchar(20);default:pending;index" json:"status"`
	ChaptersFound int            `gorm:"default:0" json:"chapters_found"`
	ChaptersAdded int            `gorm:"default:0" json:"chapters_added"`
	ErrorMessage  sql.NullString `gorm:"type:text" json:"error_message"`
	StartedAt     sql.NullTime   `json:"started_at"`
	FinishedAt    sql.NullTime   `json:"finished_at"`
	TimestampMixin
}

// ── Site ───────────────────────────────────────────────────────────────────

type Site struct {
	ID               string `gorm:"type:varchar(36);primaryKey" json:"id"`
	Domain           string `gorm:"type:varchar(255);uniqueIndex;not null" json:"domain"`
	Name             string `gorm:"type:varchar(100);not null" json:"name"`
	Template         string `gorm:"type:varchar(50);default:default" json:"template"`
	Offset           int    `gorm:"column:offset_val;default:0" json:"offset"`
	Description      string `gorm:"type:text" json:"description"`
	IsActive         bool   `gorm:"default:true" json:"is_active"`
	TranslateEnabled bool   `gorm:"default:true" json:"translate_enabled"`
	Language         string `gorm:"type:varchar(10);default:zh" json:"language"`
	// JSON columns stored as byte slices for proper serialization
	URLPatterns       []byte `gorm:"type:json" json:"url_patterns,omitempty"`
	ChapterPagination []byte `gorm:"type:json" json:"chapter_pagination,omitempty"`
	LinkWheel         []byte `gorm:"type:json" json:"link_wheel,omitempty"`
	RecommendModules  []byte `gorm:"type:json" json:"recommend_modules,omitempty"`
	TimestampMixin
}

// ── LinkRing ───────────────────────────────────────────────────────────────

type LinkRing struct {
	ID             string           `gorm:"type:varchar(36);primaryKey" json:"id"`
	Name           string           `gorm:"type:varchar(100);not null" json:"name"`
	RingType       string           `gorm:"type:varchar(30);default:cross_site" json:"ring_type"`
	SiteID         string           `gorm:"type:varchar(36);index" json:"site_id"`
	MaxLinks       int              `gorm:"default:10" json:"max_links"`
	DisplayMode    string           `gorm:"type:varchar(20);default:sidebar" json:"display_mode"`
	LinkFormat     string           `gorm:"type:varchar(500)" json:"link_format"`
	OpenNewTab     bool             `gorm:"default:true" json:"open_new_tab"`
	Nofollow       bool             `gorm:"default:false" json:"nofollow"`
	SelectionRules []byte           `gorm:"type:json" json:"selection_rules"`
	IsActive       bool             `gorm:"default:true" json:"is_active"`
	Targets        []LinkRingTarget `gorm:"foreignKey:RingID;constraint:OnDelete:CASCADE" json:"targets,omitempty"`
	TimestampMixin
}

type LinkRingTarget struct {
	ID            string `gorm:"type:varchar(36);primaryKey" json:"id"`
	RingID        string `gorm:"type:varchar(36);not null;index" json:"ring_id"`
	SourceSiteID  string `gorm:"type:varchar(36)" json:"source_site_id"`
	SourceNovelID string `gorm:"type:varchar(36)" json:"source_novel_id"`
	TargetSiteID  string `gorm:"type:varchar(36)" json:"target_site_id"`
	TargetNovelID string `gorm:"type:varchar(36)" json:"target_novel_id"`
	TargetURL     string `gorm:"type:varchar(500)" json:"target_url"`
	AnchorText    string `gorm:"type:varchar(255)" json:"anchor_text"`
	SortOrder     int    `gorm:"default:0" json:"sort_order"`
	IsActive      bool   `gorm:"default:true" json:"is_active"`
	TimestampMixin
}

// ── TranslationCache ───────────────────────────────────────────────────────

type TranslationCache struct {
	ID             string `gorm:"type:varchar(36);primaryKey" json:"id"`
	SourceHash     string `gorm:"type:varchar(64);uniqueIndex:idx_trans_hash_lang,priority:1;not null" json:"source_hash"`
	SourceLang     string `gorm:"type:varchar(10)" json:"source_lang"`
	TargetLang     string `gorm:"type:varchar(10);uniqueIndex:idx_trans_hash_lang,priority:2;not null" json:"target_lang"`
	SourceText     string `gorm:"type:text" json:"source_text"`
	TranslatedText string `gorm:"type:text;not null" json:"translated_text"`
	TimestampMixin
}

// ── GORM Hooks ─────────────────────────────────────────────────────────────

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = newUUID()
	}
	if u.Role == "" {
		u.Role = "user"
	}
	return nil
}

func (n *Novel) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = newUUID()
	}
	if n.Status == "" {
		n.Status = "ongoing"
	}
	return nil
}

func (c *Chapter) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = newUUID()
	}
	return nil
}

func (ct *CrawlerTask) BeforeCreate(tx *gorm.DB) error {
	if ct.ID == "" {
		ct.ID = newUUID()
	}
	if ct.Status == "" {
		ct.Status = "pending"
	}
	return nil
}

func (s *Site) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = newUUID()
	}
	if s.Template == "" {
		s.Template = "default"
	}
	if s.Language == "" {
		s.Language = "zh"
	}
	return nil
}

func (lr *LinkRing) BeforeCreate(tx *gorm.DB) error {
	if lr.ID == "" {
		lr.ID = newUUID()
	}
	return nil
}

func (lrt *LinkRingTarget) BeforeCreate(tx *gorm.DB) error {
	if lrt.ID == "" {
		lrt.ID = newUUID()
	}
	return nil
}

func (tc *TranslationCache) BeforeCreate(tx *gorm.DB) error {
	if tc.ID == "" {
		tc.ID = newUUID()
	}
	return nil
}
