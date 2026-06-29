package services

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

// ── Word count ─────────────────────────────────────────────────────────────

var (
	reChinese = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	reEnglish = regexp.MustCompile(`[a-zA-Z]+`)
	reHTML    = regexp.MustCompile(`<[^>]+>`)
)

// CountWords counts Chinese characters + English words in text.
func CountWords(text string) int {
	if text == "" {
		return 0
	}
	clean := reHTML.ReplaceAllString(text, "")
	return len(reChinese.FindAllString(clean, -1)) + len(reEnglish.FindAllString(clean, -1))
}

// ── Chapter CRUD ───────────────────────────────────────────────────────────

// GetChapters returns paginated chapters for a novel.
func GetChapters(novelID string, page, size int) ([]models.Chapter, int64, error) {
	var total int64
	database.DB.Model(&models.Chapter{}).Where("novel_id = ?", novelID).Count(&total)

	var chapters []models.Chapter
	if err := database.DB.Where("novel_id = ?", novelID).
		Order("sort_order ASC").
		Offset((page - 1) * size).
		Limit(size).
		Find(&chapters).Error; err != nil {
		return nil, 0, err
	}
	return chapters, total, nil
}

// GetChapter retrieves a single chapter by ID.
func GetChapter(chapterID string) (*models.Chapter, error) {
	var ch models.Chapter
	if err := database.DB.Where("id = ?", chapterID).First(&ch).Error; err != nil {
		return nil, err
	}
	return &ch, nil
}

// GetChapterContent reads chapter content from compressed file store (with DB fallback).
func GetChapterContent(ch *models.Chapter) (string, error) {
	if ch.ContentFile != "" {
		text, err := ReadContentFile(ch.NovelID, ch.ID, ch.ContentFile)
		if err == nil && text != "" {
			return text, nil
		}
	}
	return ch.Content, nil
}

// CreateChapter inserts a single chapter and updates novel's denormalized count.
func CreateChapter(novelID, title, content, sourceURL string, sortOrder int, isPublished bool) (*models.Chapter, error) {
	if sortOrder <= 0 {
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
			ch.ContentFile = contentFile
			database.DB.Model(ch).Update("content_file", contentFile)
		}
	}

	// Atomic increment of total_chapters
	recountNovelChapters(novelID, 1)

	return ch, nil
}

// UpdateChapter applies partial updates to a chapter.
func UpdateChapter(chapterID string, updates map[string]interface{}) (*models.Chapter, error) {
	ch, err := GetChapter(chapterID)
	if err != nil {
		return nil, err
	}
	if content, ok := updates["content"].(string); ok {
		updates["word_count"] = CountWords(content)
		contentFile, err := WriteContentFile(ch.NovelID, ch.ID, content)
		if err == nil {
			updates["content_file"] = contentFile
		}
	}
	if err := database.DB.Model(ch).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetChapter(chapterID)
}

// DeleteChapter removes a chapter and updates novel count.
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

// BatchCreateChapters inserts multiple chapters efficiently.
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
		if so, ok := data["sort_order"].(int); ok && so > 0 {
			sortOrder = so
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

	// Write content files concurrently
	var wg sync.WaitGroup
	for i := range chapters {
		if content, ok := chaptersData[i]["content"].(string); ok && content != "" {
			wg.Add(1)
			go func(idx int, txt string) {
				defer wg.Done()
				contentFile, err := WriteContentFile(novelID, chapters[idx].ID, txt)
				if err == nil {
					database.DB.Model(&chapters[idx]).Update("content_file", contentFile)
				}
			}(i, content)
		}
	}
	wg.Wait()

	recountNovelChapters(novelID, len(chapters))
	return chapters, nil
}

// BatchDeleteChapters removes multiple chapters atomically.
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

// ReorderChapters bulk-updates sort_order with a CASE expression.
func ReorderChapters(novelID string, orders map[string]int) error {
	if len(orders) == 0 {
		return nil
	}
	tx := database.DB.Begin()
	for chID, newOrder := range orders {
		if err := tx.Model(&models.Chapter{}).Where("id = ? AND novel_id = ?", chID, novelID).
			Update("sort_order", newOrder).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit().Error
}

// ── Content file store ─────────────────────────────────────────────────────

const contentDir = "data/content"

// WriteContentFile compresses and saves chapter content to disk.
func WriteContentFile(novelID, chapterID, content string) (string, error) {
	dir := filepath.Join(contentDir, novelID[:min(2, len(novelID))], novelID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	relPath := filepath.Join(novelID[:min(2, len(novelID))], novelID, chapterID+".gz")
	absPath := filepath.Join(contentDir, relPath)

	f, err := os.Create(absPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil {
		return "", err
	}
	gw.Close()

	return relPath, nil
}

// ReadContentFile reads and decompresses chapter content from disk.
func ReadContentFile(novelID, chapterID, contentFile string) (string, error) {
	path := filepath.Join(contentDir, contentFile)
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
	// Use database expression for atomic read-modify-write (prevents race conditions)
	database.DB.Model(&models.Novel{}).Where("id = ?", novelID).
		UpdateColumn("total_chapters", database.DB.Raw("total_chapters + ?", delta))
}
