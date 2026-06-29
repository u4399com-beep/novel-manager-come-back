// Package site provides public-facing SSR HTML routes using Go html/template.
package site

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/u4399com-beep/novel-manager-come-back/internal/config"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
	"github.com/u4399com-beep/novel-manager-come-back/internal/services"
)

type Router struct {
	cfg       *config.Config
	templates *template.Template
}

func NewRouter(cfg *config.Config) *Router {
	r := &Router{cfg: cfg}
	tplDir := filepath.Join("web", "templates", "default")
	pattern := filepath.Join(tplDir, "pages", "*.html")

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"T": func(key string) string { return key },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"or": func(a, b string) string {
			if a != "" {
				return a
			}
			return b
		},
		"gt": func(a, b int) bool { return a > b },
		"eq": func(a, b interface{}) bool { return a == b },
		"truncate": func(s string, n int) string {
			r := []rune(s)
			if len(r) <= n {
				return s
			}
			return string(r[:n]) + "..."
		},
		"statusLabel": func(s string) string {
			return map[string]string{"ongoing": "连载中", "completed": "已完结", "hiatus": "暂停更新"}[s]
		},
		"splitParagraphs": func(s string) []string {
			parts := strings.Split(s, "\n")
			result := make([]string, 0, len(parts))
			for _, p := range parts {
				if p = strings.TrimSpace(p); p != "" {
					result = append(result, p)
				}
			}
			if len(result) == 0 && s != "" {
				result = append(result, s)
			}
			return result
		},
	}).ParseGlob(pattern)

	if err != nil {
		log.Fatalf("FATAL: Template loading failed: %v", err)
	}
	r.templates = tmpl
	return r
}

func (r *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", r.handleHome)
	mux.HandleFunc("/novels", r.handleBookLibrary)
	mux.HandleFunc("/novel/", r.handleNovelDetail)
	mux.HandleFunc("/chapter/", r.handleChapterRead)
	mux.HandleFunc("/search", r.handleSearch)
}

// ── Home ─────────────────────────────────────────────────────────────────────

func (r *Router) handleHome(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size := 24

	var novels []models.Novel
	db := database.DB.Preload("Categories").Order("updated_at DESC")
	db = db.Offset((page - 1) * size).Limit(size)
	db.Find(&novels)

	var total int64
	database.DB.Model(&models.Novel{}).Count(&total)

	var ranking []models.Novel
	database.DB.Preload("Categories").Order("total_chapters DESC").Limit(15).Find(&ranking)

	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	r.render(w, "home.html", map[string]interface{}{
		"Title":      "归来小说CMS - 首页",
		"Novels":     novels,
		"Ranking":    ranking,
		"Featured":   safeSlice(ranking, 5),
		"Categories": categories,
		"Page":       page, "Total": total,
		"Pages": pagesFromTotal(total, size),
	})
}

// ── Book Library ─────────────────────────────────────────────────────────────

func (r *Router) handleBookLibrary(w http.ResponseWriter, req *http.Request) {
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size := 30

	var total int64
	var novels []models.Novel
	db := database.DB.Model(&models.Novel{})

	if catID := req.URL.Query().Get("category"); catID != "" {
		db = db.Joins("JOIN novel_categories ON novel_categories.novel_id = novels.id").
			Where("novel_categories.category_id = ?", catID)
	}
	if status := req.URL.Query().Get("status"); status != "" {
		db = db.Where("status = ?", status)
	}
	db.Count(&total)
	db.Preload("Categories").Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&novels)

	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	r.render(w, "home.html", map[string]interface{}{
		"Title": "归来小说CMS - 书库", "Novels": novels,
		"Categories": categories, "Page": page, "Total": total,
		"Pages":    pagesFromTotal(total, size),
		"Featured": safeSlice(novels, 5), "Ranking": safeSlice(novels, 15),
	})
}

// ── Novel Detail ─────────────────────────────────────────────────────────────

func (r *Router) handleNovelDetail(w http.ResponseWriter, req *http.Request) {
	novelID := strings.TrimPrefix(req.URL.Path, "/novel/")
	if novelID == "" {
		http.NotFound(w, req)
		return
	}

	var novel models.Novel
	if err := database.DB.Preload("Categories").First(&novel, "id = ?", novelID).Error; err != nil {
		http.NotFound(w, req)
		return
	}

	var chapters []models.Chapter
	database.DB.Where("novel_id = ?", novelID).Order("sort_order DESC").Limit(15).Find(&chapters)

	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	r.render(w, "novel.html", map[string]interface{}{
		"Title":      novel.Title + " - 归来小说CMS",
		"Novel":      novel,
		"Chapters":   chapters,
		"Categories": categories,
	})
}

// ── Chapter Reader ───────────────────────────────────────────────────────────

func (r *Router) handleChapterRead(w http.ResponseWriter, req *http.Request) {
	chapterID := strings.TrimPrefix(req.URL.Path, "/chapter/")
	if chapterID == "" {
		http.NotFound(w, req)
		return
	}

	var chapter models.Chapter
	if err := database.DB.First(&chapter, "id = ?", chapterID).Error; err != nil {
		http.NotFound(w, req)
		return
	}

	// Eagerly load novel for template
	var novel models.Novel
	if err := database.DB.First(&novel, "id = ?", chapter.NovelID).Error; err != nil {
		novel = models.Novel{Title: "未知", Author: "未知"}
	}

	var prev, next models.Chapter
	database.DB.Where("novel_id = ? AND sort_order < ?", chapter.NovelID, chapter.SortOrder).
		Order("sort_order DESC").Limit(1).First(&prev)
	database.DB.Where("novel_id = ? AND sort_order > ?", chapter.NovelID, chapter.SortOrder).
		Order("sort_order ASC").Limit(1).First(&next)

	// Load content via file store with DB fallback
	content, _ := services.GetChapterContent(&chapter)
	chapter.Content = content

	r.render(w, "chapter.html", map[string]interface{}{
		"Title":   chapter.Title + " - " + novel.Title,
		"Novel":   novel,
		"Chapter": chapter,
		"PrevID":  prev.ID, "PrevT": prev.Title,
		"NextID": next.ID, "NextT": next.Title,
	})
}

// ── Search ───────────────────────────────────────────────────────────────────

func (r *Router) handleSearch(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query().Get("q")
	var results []models.Novel
	var total int64
	if q != "" {
		like := "%" + q + "%"
		db := database.DB.Preload("Categories").
			Where("LOWER(title) LIKE LOWER(?) OR LOWER(author) LIKE LOWER(?)", like, like)
		db.Model(&models.Novel{}).Count(&total)
		db.Order("updated_at DESC").Limit(20).Find(&results)
	}

	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	r.render(w, "search.html", map[string]interface{}{
		"Title":      "搜索: " + q + " - 归来小说CMS",
		"Query":      q,
		"Results":    results,
		"Total":      total,
		"Categories": categories,
	})
}

// ── Render ───────────────────────────────────────────────────────────────────

func (r *Router) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	if r.templates == nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	data["SiteName"] = "归来小说CMS"
	data["Lang"] = "zh"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Render error for %s: %v", name, err)
	}
}

func safeSlice(s []models.Novel, n int) []models.Novel {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func pagesFromTotal(total int64, size int) int {
	if total == 0 {
		return 0
	}
	p := int(total) / size
	if int(total)%size > 0 {
		p++
	}
	return p
}
