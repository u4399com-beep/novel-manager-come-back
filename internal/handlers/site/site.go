// Package site provides the public-facing SSR HTML routes.
package site

import (
	"html/template"
	"log"
	"math"
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
	templates map[string]*template.Template
}

func NewRouter(cfg *config.Config) *Router {
	r := &Router{cfg: cfg, templates: make(map[string]*template.Template)}
	tplDir := filepath.Join("web", "templates", "default")
	pattern := filepath.Join(tplDir, "pages", "*.html")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"T":    func(key string) string { return key },
		"seq":  func(n int) []int { s := make([]int, n); for i := range s { s[i] = i + 1 }; return s },
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n { return s }
			return string(runes[:n]) + "..."
		},
		"statusLabel": func(s string) string {
			switch s {
			case "ongoing": return "连载中"
			case "completed": return "已完结"
			case "hiatus": return "暂停更新"
			default: return s
			}
		},
		"splitParagraphs": func(s string) []string {
			parts := strings.Split(s, "\n")
			result := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" { result = append(result, p) }
			}
			if len(result) == 0 && s != "" { result = append(result, s) }
			return result
		},
	}).ParseGlob(pattern)
	if err != nil {
		log.Fatalf("FATAL: Template loading failed: %v", err)
	}
	if tmpl != nil {
		r.templates["default"] = tmpl
	}
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
	if page < 1 { page = 1 }
	size := 24

	var total int64
	var novels []models.Novel
	database.DB.Model(&models.Novel{}).Count(&total)
	database.DB.Preload("Categories").Order("updated_at DESC").
		Offset((page - 1) * size).Limit(size).Find(&novels)

	var ranking []models.Novel
	database.DB.Preload("Categories").Order("total_chapters DESC").Limit(15).Find(&ranking)

	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	r.render(w, "home.html", map[string]interface{}{
		"Title": "归来小说CMS - 首页", "Novels": novels,
		"Ranking": ranking, "Featured": safeSlice(ranking, 5),
		"Categories": categories, "Page": page, "Total": total,
		"Pages": max(1, int(math.Ceil(float64(total)/float64(size)))),
	})
}

// ── Book Library ─────────────────────────────────────────────────────────────

func (r *Router) handleBookLibrary(w http.ResponseWriter, req *http.Request) {
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 { page = 1 }
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
		"Pages": max(1, int(math.Ceil(float64(total)/float64(size)))),
		"Featured": safeSlice(novels, 5), "Ranking": safeSlice(novels, 15),
	})
}

// ── Novel Detail ─────────────────────────────────────────────────────────────

func (r *Router) handleNovelDetail(w http.ResponseWriter, req *http.Request) {
	novelID := strings.TrimPrefix(req.URL.Path, "/novel/")
	if novelID == "" { http.NotFound(w, req); return }

	var novel models.Novel
	if err := database.DB.Preload("Categories").First(&novel, "id = ?", novelID).Error; err != nil {
		http.NotFound(w, req); return
	}

	var chapters []models.Chapter
	database.DB.Where("novel_id = ?", novelID).
		Order("sort_order DESC").Limit(15).Find(&chapters)

	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	r.render(w, "novel.html", map[string]interface{}{
		"Title": novel.Title + " - 归来小说CMS", "Novel": novel,
		"Chapters": chapters, "Categories": categories,
	})
}

// ── Chapter Reader ───────────────────────────────────────────────────────────

func (r *Router) handleChapterRead(w http.ResponseWriter, req *http.Request) {
	chapterID := strings.TrimPrefix(req.URL.Path, "/chapter/")
	if chapterID == "" { http.NotFound(w, req); return }

	var chapter models.Chapter
	if err := database.DB.First(&chapter, "id = ?", chapterID).Error; err != nil {
		http.NotFound(w, req); return
	}

	var novel models.Novel
	database.DB.First(&novel, "id = ?", chapter.NovelID)

	var prev, next models.Chapter
	database.DB.Where("novel_id = ? AND sort_order < ?", chapter.NovelID, chapter.SortOrder).
		Order("sort_order DESC").Limit(1).First(&prev)
	database.DB.Where("novel_id = ? AND sort_order > ?", chapter.NovelID, chapter.SortOrder).
		Order("sort_order ASC").Limit(1).First(&next)

	// Read content from gzip file store (fallback to DB column)
	content := chapter.Content
	if chapter.ContentFile != "" {
		if c, err := services.ReadContentFile(chapter.NovelID, chapter.ID, chapter.ContentFile); err == nil && c != "" {
			content = c
		}
	}
	chapter.Content = content

	r.render(w, "chapter.html", map[string]interface{}{
		"Title": chapter.Title + " - " + novel.Title,
		"Novel": novel, "Chapter": chapter,
		"PrevID": prev.ID, "PrevT": prev.Title,
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
		"Title": "搜索: " + q + " - 归来小说CMS", "Query": q,
		"Results": results, "Total": total, "Categories": categories,
	})
}

// ── Render ───────────────────────────────────────────────────────────────────

func (r *Router) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	tmpl := r.templates["default"]
	if tmpl == nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	data["SiteName"] = "归来小说CMS"
	data["Lang"] = "zh"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("Render error for %s: %v", name, err)
	}
}

func safeSlice(s []models.Novel, n int) []models.Novel {
	if len(s) <= n { return s }
	return s[:n]
}
