package services

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
	"gorm.io/gorm"
)

// ── Word count ─────────────────────────────────────────────────────────────

// CountWords counts Chinese characters + English words in text without regex.
func CountWords(text string) int {
	if text == "" {
		return 0
	}
	// Fast path: single-pass counting
	chinese, english, inWord := 0, 0, false
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff {
			chinese++
			inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			if !inWord {
				english++
				inWord = true
			}
		} else {
			inWord = false
		}
	}
	return chinese + english
}

// ── Chapter CRUD ───────────────────────────────────────────────────────────

func GetChapters(novelID string, page, size int) ([]models.Chapter, int64, error) {
	var total int64
	database.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).Count(&total)

	var chapters []models.Chapter
	if err := database.DB.Where("novel_id = ?", novelID).
		Order("sort_order ASC").
		Offset((page - 1) * size).Limit(size).
		Find(&chapters).Error; err != nil {
		return nil, 0, err
	}
	return chapters, total, nil
}

func GetChapter(chapterID string) (*models.Chapter, error) {
	var ch models.Chapter
	if err := database.DB.Where("id = ?", chapterID).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

func GetChapterContent(ch *models.Chapter) (string, error) {
	if ch.ContentFile != "" {
		text, err := ReadContentFile(ch.ContentFile)
		if err == nil && text != "" {
			return text, nil
		}
	}
	return ch.Content, nil
}

func CreateChapter(novelID, title, content, sourceURL string, sortOrder int, isPublished bool) (*models.Chapter, error) {
	if novelID == "" || utf8.RuneCountInString(novelID) < 2 {
		return nil, fmt.Errorf("invalid novel_id")
	}
	if sortOrder == 0 {
		var maxOrder int
		database.DB.Model(&models.Chapter{}).
			Where("novel_id = ?", novelID).
			Select("COALESCE(MAX(sort_order), 0)").Scan(&maxOrder)
		sortOrder = maxOrder + 1
	}

	ch := &models.Chapter{
		NovelID:     novelID,
		Title:       title,
		SourceURL:   sourceURL,
		IsPublished: isPublished,
		SortOrder:   sortOrder,
		WordCount:   CountWords(content),
	}

	if err := database.DB.Create(ch).Error; err != nil {
		return nil, err
	}

	if content != "" {
		contentFile, err := WriteContentFile(novelID, ch.ID, content)
		if err == nil {
			database.DB.Model(ch).Update("content_file", contentFile)
			ch.ContentFile = contentFile
		}
	}

	recountNovelChapters(novelID, 1)
	return ch, nil
}

func UpdateChapter(chapterID string, updates map[string]interface{}) (*models.Chapter, error) {
	ch, err := GetChapter(chapterID)
	if err != nil {
		return nil, err
	}

	// Allowlist: only these fields can be updated
	safeUpdates := make(map[string]interface{})
	allowedKeys := map[string]bool{
		"title": true, "content": true, "source_url": true,
		"sort_order": true, "is_published": true, "volume": true,
	}
	for k, v := range updates {
		if allowedKeys[k] {
			safeUpdates[k] = v
		}
	}

	if content, ok := safeUpdates["content"].(string); ok {
		safeUpdates["word_count"] = CountWords(content)
		contentFile, err := WriteContentFile(ch.NovelID, ch.ID, content)
		if err == nil {
			safeUpdates["content_file"] = contentFile
		}
	}

	if len(safeUpdates) > 0 {
		if err := database.DB.Model(ch).Updates(safeUpdates).Error; err != nil {
			return nil, err
		}
	}
	return GetChapter(chapterID)
}

func DeleteChapter(chapterID string) error {
	ch, err := GetChapter(chapterID)
	if err != nil {
		return err
	}
	if err := database.DB.Delete(ch).Error; err != nil {
		return err
	}
	recountNovelChapters(ch.NovelID, -1)
	return nil
}

func BatchCreateChapters(novelID string, chaptersData []map[string]interface{}) ([]models.Chapter, error) {
	var maxOrder int
	database.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).
		Select("COALESCE(MAX(sort_order),0)").Scan(&maxOrder)

	chapters := make([]models.Chapter, 0, len(chaptersData))
	for i, data := range chaptersData {
		title, _ := data["title"].(string)
		content, _ := data["content"].(string)
		sourceURL, _ := data["source_url"].(string)
		volume, _ := data["volume"].(string)

		sortOrder := maxOrder + i + 1
		if so, ok := data["sort_order"].(float64); ok && int(so) > 0 {
			sortOrder = int(so)
		}

		ch := models.Chapter{
			NovelID:   novelID,
			Title:     title,
			SourceURL: sourceURL,
			Volume:    volume,
			SortOrder: sortOrder,
			WordCount: CountWords(content),
		}
		chapters = append(chapters, ch)
	}

	if err := database.DB.Create(&chapters).Error; err != nil {
		return nil, err
	}

	// Write content files asynchronously (each goroutine creates its own DB session)
	var wg sync.WaitGroup
	for i := range chapters {
		if content, ok := chaptersData[i]["content"].(string); ok && content != "" {
			wg.Add(1)
			go func(chapterID, novelID, txt string) {
				defer wg.Done()
				if cf, err := WriteContentFile(novelID, chapterID, txt); err == nil {
					// Each goroutine uses a fresh GORM session (safe for concurrent use)
					database.DB.Session(&gorm.Session{}).
						Model(&models.Chapter{}).Where("id = ?", chapterID).
						Update("content_file", cf)
				}
			}(chapters[i].ID, novelID, content)
		}
	}
	wg.Wait()

	recountNovelChapters(novelID, len(chapters))
	return chapters, nil
}

func BatchDeleteChapters(novelID string, chapterIDs []string) (int64, error) {
	if len(chapterIDs) == 0 {
		return 0, nil
	}
	result := database.DB.Where("id IN ? AND novel_id = ?", chapterIDs, novelID).Delete(&models.Chapter{})
	if result.Error != nil {
		return 0, result.Error
	}
	recountNovelChapters(novelID, -int(result.RowsAffected))
	return result.RowsAffected, nil
}

func ReorderChapters(novelID string, orders map[string]int) error {
	if len(orders) == 0 {
		return nil
	}
	// Bulk update using a single transaction with CASE WHEN
	return database.DB.Transaction(func(tx *gorm.DB) error {
		for chID, newOrder := range orders {
			if err := tx.Model(&models.Chapter{}).
				Where("id = ? AND novel_id = ?", chID, novelID).
				Update("sort_order", newOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ── Content file store ─────────────────────────────────────────────────────

// contentDir is resolved to absolute path at init for path traversal safety.
var contentDir = func() string {
	dir := "data/content"
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}()

func WriteContentFile(novelID, chapterID, content string) (string, error) {
	if novelID == "" || chapterID == "" {
		return "", fmt.Errorf("invalid IDs")
	}
	prefix := novelID[:min(2, len(novelID))]
	dir := filepath.Join(contentDir, prefix, novelID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	relPath := filepath.Join(prefix, novelID, chapterID+".gz")
	absPath := filepath.Join(contentDir, relPath)

	// Validate path stays within contentDir
	if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(contentDir)) {
		return "", fmt.Errorf("path traversal detected")
	}

	f, err := os.Create(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil {
		gw.Close()
		return "", err
	}
	if err := gw.Close(); err != nil {
		return "", err
	}
	return relPath, nil
}

func ReadContentFile(contentFile string) (string, error) {
	path := filepath.Join(contentDir, contentFile)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(contentDir)) {
		return "", fmt.Errorf("path traversal detected")
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	var sb strings.Builder
	buf := make([]byte, 32*1024)
	for {
		n, err := gr.Read(buf)
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String(), nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

func recountNovelChapters(novelID string, delta int) {
	database.DB.Model(&models.Novel{}).Where("id = ?", novelID).
		UpdateColumn("total_chapters", gorm.Expr("total_chapters + ?", delta))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
