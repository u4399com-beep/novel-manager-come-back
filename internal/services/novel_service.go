package services

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

type NovelListParams struct {
	Page, Size     int
	Search         string
	CategoryID     *int
	Status         string
	SortBy, SortDir string
}

type NovelListResult struct {
	Items []models.Novel `json:"items"`
	Total int64          `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
	Pages int            `json:"pages"`
}

func ListNovels(ctx context.Context, params NovelListParams) (*NovelListResult, error) {
	pool := database.Pool

	var total int64
	countSQL := "SELECT COUNT(*) FROM novels WHERE 1=1"
	args := []interface{}{}
	argN := 1

	if params.Search != "" {
		like := "%" + params.Search + "%"
		countSQL += fmt.Sprintf(" AND (LOWER(title) LIKE LOWER($%d) OR LOWER(author) LIKE LOWER($%d))", argN, argN+1)
		args = append(args, like, like)
		argN += 2
	}
	if params.Status != "" {
		countSQL += fmt.Sprintf(" AND status = $%d", argN)
		args = append(args, params.Status)
		argN++
	}
	if params.CategoryID != nil {
		countSQL += fmt.Sprintf(" AND id IN (SELECT novel_id FROM novel_categories WHERE category_id = $%d)", argN)
		args = append(args, *params.CategoryID)
		argN++
	}

	if err := pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}

	sortCol := "updated_at"
	allowed := map[string]bool{"id": true, "title": true, "author": true, "status": true, "total_chapters": true, "created_at": true, "updated_at": true}
	if allowed[params.SortBy] {
		sortCol = params.SortBy
	}
	sortDir := "DESC"
	if params.SortDir == "asc" {
		sortDir = "ASC"
	}

	querySQL := fmt.Sprintf("SELECT id, title, author, description, cover_image_url, source_url, source_name, status, total_chapters, created_at, updated_at FROM novels WHERE 1=1")
	// Rebuild args since we have a different query
	queryArgs := []interface{}{}
	qArgN := 1

	if params.Search != "" {
		querySQL += fmt.Sprintf(" AND (LOWER(title) LIKE LOWER($%d) OR LOWER(author) LIKE LOWER($%d))", qArgN, qArgN+1)
		queryArgs = append(queryArgs, "%"+params.Search+"%", "%"+params.Search+"%")
		qArgN += 2
	}
	if params.Status != "" {
		querySQL += fmt.Sprintf(" AND status = $%d", qArgN)
		queryArgs = append(queryArgs, params.Status)
		qArgN++
	}
	if params.CategoryID != nil {
		querySQL += fmt.Sprintf(" AND id IN (SELECT novel_id FROM novel_categories WHERE category_id = $%d)", qArgN)
		queryArgs = append(queryArgs, *params.CategoryID)
		qArgN++
	}

	querySQL += fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", sortCol, sortDir, qArgN, qArgN+1)
	queryArgs = append(queryArgs, params.Size, (params.Page-1)*params.Size)

	rows, err := pool.Query(ctx, querySQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	novels, err := pgx.CollectRows(rows, pgx.RowToStructByName[models.Novel])
	if err != nil {
		return nil, err
	}

	// Load categories for each novel
	for i := range novels {
		catRows, err := pool.Query(ctx,
			"SELECT c.id, c.name, c.slug, c.sort_order, c.created_at, c.updated_at FROM categories c JOIN novel_categories nc ON nc.category_id = c.id WHERE nc.novel_id = $1 ORDER BY c.sort_order",
			novels[i].ID)
		if err == nil {
			cats, _ := pgx.CollectRows(catRows, pgx.RowToStructByName[models.Category])
			novels[i].Categories = cats
			catRows.Close()
		}
	}

	return &NovelListResult{
		Items: novels, Total: total,
		Page: params.Page, Size: params.Size,
		Pages: max(1, int(math.Ceil(float64(total)/float64(params.Size)))),
	}, nil
}

func GetNovel(ctx context.Context, id string) (*models.Novel, error) {
	pool := database.Pool
	n := &models.Novel{}
	err := pool.QueryRow(ctx,
		"SELECT id, title, author, description, cover_image_url, source_url, source_name, status, total_chapters, created_at, updated_at FROM novels WHERE id = $1", id,
	).Scan(&n.ID, &n.Title, &n.Author, &n.Description, &n.CoverImageURL, &n.SourceURL, &n.SourceName, &n.Status, &n.TotalChapters, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}

	rows, _ := pool.Query(ctx, "SELECT c.id, c.name, c.slug, c.sort_order, c.created_at, c.updated_at FROM categories c JOIN novel_categories nc ON nc.category_id = c.id WHERE nc.novel_id = $1", id)
	if rows != nil {
		n.Categories, _ = pgx.CollectRows(rows, pgx.RowToStructByName[models.Category])
		rows.Close()
	}
	return n, nil
}

func CreateNovel(ctx context.Context, title, author, desc, sourceURL, sourceName, status string, categoryIDs []int) (*models.Novel, error) {
	pool := database.Pool
	n := &models.Novel{}
	err := pool.QueryRow(ctx,
		`INSERT INTO novels (title, author, description, source_url, source_name, status)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, title, author, description, cover_image_url, source_url, source_name, status, total_chapters, created_at, updated_at`,
		title, author, desc, sourceURL, sourceName, status,
	).Scan(&n.ID, &n.Title, &n.Author, &n.Description, &n.CoverImageURL, &n.SourceURL, &n.SourceName, &n.Status, &n.TotalChapters, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	for _, cid := range categoryIDs {
		pool.Exec(ctx, "INSERT INTO novel_categories (novel_id, category_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", n.ID, cid)
	}
	return GetNovel(ctx, n.ID)
}

func UpdateNovel(ctx context.Context, id string, updates map[string]interface{}, categoryIDs []int) (*models.Novel, error) {
	pool := database.Pool
	setClauses := []string{}
	args := []interface{}{}
	argN := 1

	allowed := map[string]bool{"title": true, "author": true, "description": true, "source_url": true, "source_name": true, "status": true, "cover_image_url": true}
	for k, v := range updates {
		if allowed[k] {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", k, argN))
			args = append(args, v)
			argN++
		}
	}
	if len(setClauses) > 0 {
		sql := fmt.Sprintf("UPDATE novels SET %s WHERE id = $%d", strings.Join(setClauses, ", "), argN)
		args = append(args, id)
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			return nil, err
		}
	}
	if categoryIDs != nil {
		pool.Exec(ctx, "DELETE FROM novel_categories WHERE novel_id = $1", id)
		for _, cid := range categoryIDs {
			pool.Exec(ctx, "INSERT INTO novel_categories (novel_id, category_id) VALUES ($1,$2) ON CONFLICT DO NOTHING", id, cid)
		}
	}
	return GetNovel(ctx, id)
}

func DeleteNovel(ctx context.Context, id string) error {
	_, err := database.Pool.Exec(ctx, "DELETE FROM novels WHERE id = $1", id)
	return err
}

func GetNovelStatistics(ctx context.Context, novelID string) map[string]interface{} {
	var totalCh, publishedCh, totalWords int64
	var lastUpdated time.Time
	pool := database.Pool
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM chapters WHERE novel_id = $1", novelID).Scan(&totalCh)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM chapters WHERE novel_id = $1 AND is_published = true", novelID).Scan(&publishedCh)
	pool.QueryRow(ctx, "SELECT COALESCE(SUM(word_count),0) FROM chapters WHERE novel_id = $1", novelID).Scan(&totalWords)
	pool.QueryRow(ctx, "SELECT MAX(updated_at) FROM chapters WHERE novel_id = $1", novelID).Scan(&lastUpdated)
	return map[string]interface{}{
		"novel_id": novelID, "total_chapters": totalCh,
		"published_chapters": publishedCh, "total_words": totalWords,
		"last_updated": lastUpdated.Format(time.RFC3339),
	}
}

func SaveCoverImage(cfg *config.Config, fileContent []byte, filename string) (string, error) {
	coversDir := filepath.Join(cfg.StaticDir, "covers")
	os.MkdirAll(coversDir, 0755)
	ext := filepath.Ext(filename)
	if ext == "" || len(ext) > 10 {
		ext = ".jpg"
	}
	storedName := models.NewUUID()[:12] + ext
	filePath := filepath.Join(coversDir, storedName)
	absPath, _ := filepath.Abs(filePath)
	if !strings.HasPrefix(absPath, filepath.Clean(coversDir)) {
		return "", fmt.Errorf("invalid path")
	}
	if err := os.WriteFile(absPath, fileContent, 0644); err != nil {
		return "", err
	}
	return "/static/covers/" + storedName, nil
}
