package services

import (
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func CountWords(text string) int {
	if text == "" { return 0 }
	ch, en, inWord := 0, 0, false
	for _, r := range text {
		if r >= 0x4e00 && r <= 0x9fff { ch++; inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			if !inWord { en++; inWord = true }
		} else { inWord = false }
	}
	return ch + en
}

func GetChapters(ctx context.Context, novelID string, page, size int) ([]models.Chapter, int64, error) {
	pool := database.Pool
	var total int64
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM chapters WHERE novel_id = $1", novelID).Scan(&total)

	rows, err := pool.Query(ctx,
		"SELECT id, novel_id, title, content_file, volume, sort_order, word_count, source_url, is_published, created_at, updated_at FROM chapters WHERE novel_id = $1 ORDER BY sort_order ASC LIMIT $2 OFFSET $3",
		novelID, size, (page-1)*size)
	if err != nil { return nil, 0, err }
	defer rows.Close()

	chapters, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Chapter])
	return chapters, total, err
}

func GetChapter(ctx context.Context, chapterID string) (*models.Chapter, error) {
	ch := &models.Chapter{}
	err := database.Pool.QueryRow(ctx,
		"SELECT id, novel_id, title, content, content_file, volume, sort_order, word_count, source_url, is_published, created_at, updated_at FROM chapters WHERE id = $1", chapterID,
	).Scan(&ch.ID, &ch.NovelID, &ch.Title, &ch.Content, &ch.ContentFile, &ch.Volume, &ch.SortOrder, &ch.WordCount, &ch.SourceURL, &ch.IsPublished, &ch.CreatedAt, &ch.UpdatedAt)
	return ch, err
}

func GetChapterContent(ch *models.Chapter) (string, error) {
	if ch.ContentFile != "" {
		text, err := ReadContentFile(ch.ContentFile)
		if err == nil && text != "" { return text, nil }
	}
	return ch.Content, nil
}

func CreateChapter(ctx context.Context, novelID, title, content, sourceURL string, sortOrder int, isPublished bool) (*models.Chapter, error) {
	pool := database.Pool
	if sortOrder == 0 {
		pool.QueryRow(ctx, "SELECT COALESCE(MAX(sort_order),0)+1 FROM chapters WHERE novel_id = $1", novelID).Scan(&sortOrder)
	}
	ch := &models.Chapter{}
	err := pool.QueryRow(ctx,
		`INSERT INTO chapters (novel_id, title, source_url, is_published, sort_order, word_count)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, novel_id, title, content, content_file, volume, sort_order, word_count, source_url, is_published, created_at, updated_at`,
		novelID, title, sourceURL, isPublished, sortOrder, CountWords(content),
	).Scan(&ch.ID, &ch.NovelID, &ch.Title, &ch.Content, &ch.ContentFile, &ch.Volume, &ch.SortOrder, &ch.WordCount, &ch.SourceURL, &ch.IsPublished, &ch.CreatedAt, &ch.UpdatedAt)
	if err != nil { return nil, err }

	if content != "" {
		if cf, err := WriteContentFile(novelID, ch.ID, content); err == nil {
			pool.Exec(ctx, "UPDATE chapters SET content_file = $1 WHERE id = $2", cf, ch.ID)
			ch.ContentFile = cf
		}
	}
	recountNovelChapters(ctx, novelID, 1)
	return ch, nil
}

func UpdateChapter(ctx context.Context, chapterID string, updates map[string]interface{}) (*models.Chapter, error) {
	ch, err := GetChapter(ctx, chapterID)
	if err != nil { return nil, err }

	pool := database.Pool
	safe := make(map[string]interface{})
	allowed := map[string]bool{"title": true, "content": true, "source_url": true, "sort_order": true, "is_published": true, "volume": true}
	for k, v := range updates {
		if allowed[k] { safe[k] = v }
	}
	if content, ok := safe["content"].(string); ok {
		safe["word_count"] = CountWords(content)
		if cf, err := WriteContentFile(ch.NovelID, ch.ID, content); err == nil {
			safe["content_file"] = cf
		}
	}
	if len(safe) > 0 {
		setClauses := []string{}
		args := []interface{}{}
		n := 1
		for k, v := range safe {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, n))
			args = append(args, v)
			n++
		}
		sql := fmt.Sprintf("UPDATE chapters SET %s WHERE id = $%d", strings.Join(setClauses, ", "), n)
		args = append(args, chapterID)
		if _, err := pool.Exec(ctx, sql, args...); err != nil { return nil, err }
	}
	return GetChapter(ctx, chapterID)
}

func DeleteChapter(ctx context.Context, chapterID string) error {
	ch, err := GetChapter(ctx, chapterID)
	if err != nil { return err }
	if _, err := database.Pool.Exec(ctx, "DELETE FROM chapters WHERE id = $1", chapterID); err != nil {
		return err
	}
	recountNovelChapters(ctx, ch.NovelID, -1)
	return nil
}

func BatchCreateChapters(ctx context.Context, novelID string, chaptersData []map[string]interface{}) ([]models.Chapter, error) {
	pool := database.Pool
	var maxOrder int
	pool.QueryRow(ctx, "SELECT COALESCE(MAX(sort_order),0) FROM chapters WHERE novel_id = $1", novelID).Scan(&maxOrder)

	chapters := make([]models.Chapter, 0, len(chaptersData))
	for i, data := range chaptersData {
		title, _ := data["title"].(string)
		content, _ := data["content"].(string)
		sourceURL, _ := data["source_url"].(string)
		volume, _ := data["volume"].(string)

		so := maxOrder + i + 1
		if v, ok := data["sort_order"].(float64); ok && int(v) > 0 { so = int(v) }

		ch := models.Chapter{}
		err := pool.QueryRow(ctx,
			`INSERT INTO chapters (novel_id, title, source_url, volume, sort_order, word_count)
			 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, novel_id, title, content, content_file, volume, sort_order, word_count, source_url, is_published, created_at, updated_at`,
			novelID, title, sourceURL, volume, so, CountWords(content),
		).Scan(&ch.ID, &ch.NovelID, &ch.Title, &ch.Content, &ch.ContentFile, &ch.Volume, &ch.SortOrder, &ch.WordCount, &ch.SourceURL, &ch.IsPublished, &ch.CreatedAt, &ch.UpdatedAt)
		if err != nil { return nil, err }
		chapters = append(chapters, ch)
	}

	var wg sync.WaitGroup
	for i := range chapters {
		if content, ok := chaptersData[i]["content"].(string); ok && content != "" {
			wg.Add(1)
			go func(chID, nID, txt string) {
				defer wg.Done()
				if cf, err := WriteContentFile(nID, chID, txt); err == nil {
					pool.Exec(ctx, "UPDATE chapters SET content_file = $1 WHERE id = $2", cf, chID)
				}
			}(chapters[i].ID, novelID, content)
		}
	}
	wg.Wait()
	recountNovelChapters(ctx, novelID, len(chapters))
	return chapters, nil
}

func BatchDeleteChapters(ctx context.Context, novelID string, chapterIDs []string) (int64, error) {
	if len(chapterIDs) == 0 { return 0, nil }
	tag, err := database.Pool.Exec(ctx, "DELETE FROM chapters WHERE id = ANY($1) AND novel_id = $2", chapterIDs, novelID)
	if err != nil { return 0, err }
	recountNovelChapters(ctx, novelID, -int(tag.RowsAffected()))
	return tag.RowsAffected(), nil
}

func ReorderChapters(ctx context.Context, novelID string, orders map[string]int) error {
	if len(orders) == 0 { return nil }
	tx, err := database.Pool.Begin(ctx)
	if err != nil { return err }
	defer tx.Rollback(ctx)
	for chID, so := range orders {
		if _, err := tx.Exec(ctx, "UPDATE chapters SET sort_order = $1 WHERE id = $2 AND novel_id = $3", so, chID, novelID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func recountNovelChapters(ctx context.Context, novelID string, delta int) {
	database.Pool.Exec(ctx, "UPDATE novels SET total_chapters = total_chapters + $1 WHERE id = $2", delta, novelID)
}

// ── Content file store ─────────────────────────────────────────────────────

var contentDir = func() string {
	d, _ := filepath.Abs("data/content")
	return d
}()

func WriteContentFile(novelID, chapterID, content string) (string, error) {
	if novelID == "" || chapterID == "" { return "", fmt.Errorf("invalid IDs") }
	prefix := novelID[:min(2, len(novelID))]
	dir := filepath.Join(contentDir, prefix, novelID)
	os.MkdirAll(dir, 0755)

	relPath := filepath.Join(prefix, novelID, chapterID+".gz")
	absPath := filepath.Join(contentDir, relPath)
	if !strings.HasPrefix(filepath.Clean(absPath), filepath.Clean(contentDir)) {
		return "", fmt.Errorf("path traversal")
	}
	f, _ := os.Create(absPath)
	defer f.Close()
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil { gw.Close(); return "", err }
	gw.Close()
	return relPath, nil
}

func ReadContentFile(contentFile string) (string, error) {
	contentFile = strings.TrimPrefix(contentFile, "content/")
	path := filepath.Join(contentDir, contentFile)
	if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(contentDir)) { return "", fmt.Errorf("path traversal") }
	f, _ := os.Open(path)
	defer f.Close()
	gr, _ := gzip.NewReader(f)
	defer gr.Close()
	var sb strings.Builder
	buf := make([]byte, 32*1024)
	for { n, err := gr.Read(buf); if n > 0 { sb.Write(buf[:n]) }; if err != nil { break } }
	return sb.String(), nil
}

func min(a, b int) int { if a < b { return a }; return b }
