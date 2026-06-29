// Package site provides public-facing SSR HTML routes using Go html/template.
package site

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
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
		"add": func(a, b int) int { return a + b },
		"gt":  func(a, b int) bool { return a > b },
		"lt":  func(a, b int) bool { return a < b },
		"stripHTML": func(s string) string {
			s = strings.ReplaceAll(s, "<br>", "\n")
			s = strings.ReplaceAll(s, "<br/>", "\n")
			s = strings.ReplaceAll(s, "<br />", "\n")
			re := regexp.MustCompile(`<[^>]*>`)
			return strings.TrimSpace(re.ReplaceAllString(s, ""))
		},
		"eq":  func(a, b interface{}) bool { return a == b },
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

// homeNovelItem enriches a novel with its latest chapter and primary category.
type homeNovelItem struct {
	Novel           models.Novel
	CategoryName    string
	LatestChapter   string
	LatestChapterID string
	UpdatedMMDD     string
}

// catGroup holds novels for a category recommendation block.
type catGroup struct {
	Category models.Category
	Novels   []models.Novel
}

func (r *Router) handleHome(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req); return
	}

	// ── Categories ──
	var categories []models.Category
	database.DB.Order("sort_order ASC").Find(&categories)

	// ── Category Recommendations: top 6 categories, 4 novels each ──
	catRecs := make([]catGroup, 0)
	for i, cat := range categories {
		if i >= 6 {
			break
		}
		var novels []models.Novel
		database.DB.Preload("Categories").
			Joins("JOIN novel_categories nc ON nc.novel_id = novels.id").
			Where("nc.category_id = ?", cat.ID).
			Order("novels.total_chapters DESC").
			Limit(4).
			Find(&novels)
		if len(novels) > 0 {
			catRecs = append(catRecs, catGroup{Category: cat, Novels: novels})
		}
	}

	// ── Latest Updates: 30 novels + latest chapters ──
	var latestList []models.Novel
	database.DB.Preload("Categories").Order("updated_at DESC").Limit(30).Find(&latestList)

	type chInfo struct {
		NovelID string
		ID      string
		Title   string
	}
	latestChMap := make(map[string]chInfo)
	if len(latestList) > 0 {
		ids := make([]string, len(latestList))
		for i, n := range latestList {
			ids[i] = n.ID
		}
		var rows []chInfo
		database.DB.Raw(`
			SELECT c.novel_id, c.id, c.title FROM chapters c
			INNER JOIN (
				SELECT novel_id, MAX(sort_order) AS max_so FROM chapters
				WHERE novel_id IN (?)
				GROUP BY novel_id
			) latest ON c.novel_id = latest.novel_id AND c.sort_order = latest.max_so
		`, ids).Scan(&rows)
		for _, row := range rows {
			latestChMap[row.NovelID] = row
		}
	}

	listItems := make([]homeNovelItem, 0, len(latestList))
	for _, n := range latestList {
		catName := ""
		if len(n.Categories) > 0 {
			catName = n.Categories[0].Name
		}
		mmdd := ""
		if !n.UpdatedAt.IsZero() {
			mmdd = n.UpdatedAt.Format("01-02")
		}
		ch := latestChMap[n.ID]
		listItems = append(listItems, homeNovelItem{
			Novel: n, CategoryName: catName,
			LatestChapter: ch.Title, LatestChapterID: ch.ID,
			UpdatedMMDD: mmdd,
		})
	}

	// ── Ranking + Featured ──
	var ranking []models.Novel
	database.DB.Preload("Categories").Order("total_chapters DESC").Limit(15).Find(&ranking)

	var total int64
	database.DB.Model(&models.Novel{}).Count(&total)

	r.render(w, "home.html", map[string]interface{}{
		"Title":      "归来小说CMS - 首页",
		"CatRecs":    catRecs,
		"LatestList": listItems,
		"Ranking":    ranking,
		"Featured":   safeSlice(ranking, 5),
		"Categories": categories,
		"Total":      total,
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
	path := strings.TrimPrefix(req.URL.Path, "/novel/")
	if path == "" {
		http.NotFound(w, req); return
	}
	parts := strings.SplitN(path, "/", 2)
	novelID := parts[0]
	isChapterList := len(parts) == 2 && parts[1] == "chapters"

	var novel models.Novel
	if err := database.DB.Preload("Categories").First(&novel, "id = ?", novelID).Error; err != nil {
		http.NotFound(w, req); return
	}

	// Chapter list page
	if isChapterList {
		var allChapters []models.Chapter
		database.DB.Where("novel_id = ?", novelID).Order("sort_order ASC").Find(&allChapters)

		// Group by volume
		type volGroup struct {
			Title    string
			Chapters []models.Chapter
		}
		var grouped []volGroup
		currentVol := volGroup{Title: "正文"}
		for _, ch := range allChapters {
			vol := strings.TrimSpace(ch.Volume)
			if vol != "" && currentVol.Title != vol {
				if len(currentVol.Chapters) > 0 {
					grouped = append(grouped, currentVol)
				}
				currentVol = volGroup{Title: vol}
			}
			currentVol.Chapters = append(currentVol.Chapters, ch)
		}
		grouped = append(grouped, currentVol)

		r.render(w, "chapter_list.html", map[string]interface{}{
			"Title":       novel.Title + " - 章节目录",
			"Novel":       novel,
			"AllChapters": allChapters,
			"Grouped":     grouped,
		})
		return
	}

	// Novel detail page

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
