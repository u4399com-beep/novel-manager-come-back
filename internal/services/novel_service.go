package services

import (
	"crypto/rand"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

// ── Types ──────────────────────────────────────────────────────────────────

type NovelListParams struct {
	Page, Size      int
	Search          string
	CategoryID      *int
	Status          string
	SortBy, SortDir string
}

type NovelListResult struct {
	Items []models.Novel `json:"items"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
	Pages int            `json:"pages"`
}

// ── List / CRUD ────────────────────────────────────────────────────────────

func ListNovels(params NovelListParams) (*NovelListResult, error) {
	db := database.DB.Preload("Categories")

	if params.Search != "" {
		like := "%" + params.Search + "%"
		db = db.Where("LOWER(title) LIKE LOWER(?) OR LOWER(author) LIKE LOWER(?)", like, like)
	}
	if params.Status != "" {
		db = db.Where("status = ?", params.Status)
	}
	if params.CategoryID != nil {
		db = db.Joins("JOIN novel_categories nc ON nc.novel_id = novels.id").
			Where("nc.category_id = ?", *params.CategoryID)
	}

	var total int64
	if err := db.Model(&models.Novel{}).Count(&total).Error; err != nil {
		return nil, err
	}

	// Whitelist sortBy to prevent SQL injection
	sortCol := "updated_at"
	allowedCols := map[string]bool{
		"id": true, "title": true, "author": true, "status": true,
		"total_chapters": true, "created_at": true, "updated_at": true,
	}
	if params.SortBy != "" && allowedCols[params.SortBy] {
		sortCol = params.SortBy
	}
	sortDir := "DESC"
	if params.SortDir == "asc" {
		sortDir = "ASC"
	}
	db = db.Order(sortCol + " " + sortDir)
	db = db.Offset((params.Page - 1) * params.Size).Limit(params.Size)

	var novels []models.Novel
	if err := db.Find(&novels).Error; err != nil {
		return nil, err
	}

	return &NovelListResult{
		Items: novels, Total: total,
		Page: params.Page, Size: params.Size,
		Pages: max(1, int(math.Ceil(float64(total)/float64(params.Size)))),
	}, nil
}

func GetNovel(id string) (*models.Novel, error) {
	var novel models.Novel
	if err := database.DB.Preload("Categories").Where("id = ?", id).First(&novel).Error; err != nil {
		return nil, err
	}
	return &novel, nil
}

func CreateNovel(title, author, desc, sourceURL, sourceName, status string, categoryIDs []int) (*models.Novel, error) {
	novel := &models.Novel{
		Title: title, Author: author, Description: desc,
		SourceURL: sourceURL, SourceName: sourceName, Status: status,
	}
	if len(categoryIDs) > 0 {
		var cats []models.Category
		if err := database.DB.Where("id IN ?", categoryIDs).Find(&cats).Error; err != nil {
			return nil, err
		}
		novel.Categories = cats
	}
	if err := database.DB.Create(novel).Error; err != nil {
		return nil, err
	}
	return novel, nil
}

func UpdateNovel(id string, updates map[string]interface{}, categoryIDs []int) (*models.Novel, error) {
	// Allowlist: only these novel fields can be updated
	safeUpdates := make(map[string]interface{})
	allowedKeys := map[string]bool{
		"title": true, "author": true, "description": true,
		"source_url": true, "source_name": true, "status": true,
		"cover_image_url": true,
	}
	for k, v := range updates {
		if allowedKeys[k] {
			safeUpdates[k] = v
		}
	}

	if len(safeUpdates) > 0 {
		if err := database.DB.Model(&models.Novel{}).Where("id = ?", id).Updates(safeUpdates).Error; err != nil {
			return nil, err
		}
	}

	if categoryIDs != nil {
		novel, err := GetNovel(id)
		if err != nil {
			return nil, err
		}
		var cats []models.Category
		if len(categoryIDs) > 0 {
			if err := database.DB.Where("id IN ?", categoryIDs).Find(&cats).Error; err != nil {
				return nil, err
			}
		}
		if err := database.DB.Model(novel).Association("Categories").Replace(cats); err != nil {
			return nil, err
		}
	}

	return GetNovel(id)
}

func DeleteNovel(id string) error {
	return database.DB.Where("id = ?", id).Delete(&models.Novel{}).Error
}

func GetNovelStatistics(novelID string) map[string]interface{} {
	var totalCh, publishedCh, totalWords int64
	var lastUpdated string
	database.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).Count(&totalCh)
	database.DB.Model(&models.Chapter{}).Where("novel_id = ? AND is_published = ?", novelID, true).Count(&publishedCh)
	database.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).
		Select("COALESCE(SUM(word_count),0)").Scan(&totalWords)
	database.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).
		Select("MAX(updated_at)").Scan(&lastUpdated)
	return map[string]interface{}{
		"novel_id": novelID, "total_chapters": totalCh,
		"published_chapters": publishedCh, "total_words": totalWords,
		"last_updated": lastUpdated,
	}
}

func SaveCoverImage(cfg *config.Config, fileContent []byte, filename string) (string, error) {
	coversDir := filepath.Join(cfg.StaticDir, "covers")
	if err := os.MkdirAll(coversDir, 0755); err != nil {
		return "", err
	}
	ext := filepath.Ext(filename)
	if ext == "" || len(ext) > 10 {
		ext = ".jpg"
	}

	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random filename")
	}
	storedName := fmt.Sprintf("%x%s", b, ext)
	filePath := filepath.Join(coversDir, storedName)

	// Path traversal guard
	cleanDir := filepath.Clean(coversDir)
	absPath, err := filepath.Abs(filePath)
	if err != nil || !strings.HasPrefix(absPath, cleanDir) {
		return "", fmt.Errorf("invalid cover path")
	}

	if err := os.WriteFile(absPath, fileContent, 0644); err != nil {
		return "", err
	}
	return "/static/covers/" + storedName, nil
}
