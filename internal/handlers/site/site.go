package site

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"os"
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

var reStripHTML = regexp.MustCompile(`<[^>]*>`)

func stripHTMLFn(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	return strings.TrimSpace(reStripHTML.ReplaceAllString(s, ""))
}

func NewRouter(cfg *config.Config) *Router {
	r := &Router{cfg: cfg}
	tplName := "default"; if v := os.Getenv("TEMPLATE"); v != "" { tplName = strings.TrimSpace(strings.ToLower(v)) }; tplDir := filepath.Join("web", "templates", tplName)
	pattern := filepath.Join(tplDir, "pages", "*.html")
	tmpl, err := template.New("").Funcs(template.FuncMap{
		"or": func(a, b string) string { if a != "" { return a }; return b },
		"add": func(a, b int) int { return a + b },
		"gt":  func(a, b int) bool { return a > b },
		"lt":  func(a, b int) bool { return a < b },
		"eq":  func(a, b interface{}) bool { return a == b },
		"seq": func(n int) []int { s := make([]int, n); for i := range s { s[i] = i + 1 }; return s },
		"truncate": func(s string, n int) string { r := []rune(s); if len(r) <= n { return s }; return string(r[:n]) + "..." },
		"statusLabel": func(s string) string {
			return map[string]string{"ongoing":"连载中","completed":"已完结","hiatus":"暂停更新"}[s]
		},
		"stripHTML": stripHTMLFn,
		"paginate": paginateFn,
		"splitParagraphs": func(s string) []string {
			parts := strings.Split(s, "\n"); result := make([]string, 0, len(parts))
			for _, p := range parts { if p = strings.TrimSpace(p); p != "" { result = append(result, p) } }
			if len(result) == 0 && s != "" { result = append(result, s) }; return result
		},
	}).ParseGlob(pattern)
	if err != nil { log.Fatalf("Template loading failed: %v", err) }
	r.templates = tmpl
	return r
}

func (r *Router) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", r.home)
	mux.HandleFunc("/novels", r.bookLibrary)
	mux.HandleFunc("/novel/", r.novelDetail)
	mux.HandleFunc("/chapter/", r.chapterRead)
	mux.HandleFunc("/search", r.search)
}

// ── Home ─────────────────────────────────────────────────────────────────
func (r *Router) home(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" { http.NotFound(w, req); return }
	ctx := req.Context()

	cats := mustCategories(ctx)
	catRecs := buildCatRecs(ctx, cats)
	latest, _ := services.ListNovels(ctx, services.NovelListParams{Page:1, Size:30, SortBy:"updated_at", SortDir:"desc"})
	ranking, _ := services.ListNovels(ctx, services.NovelListParams{Page:1, Size:15, SortBy:"total_chapters", SortDir:"desc"})
	listItems := buildLatest(ctx, latest.Items)
	var total int64
	database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels").Scan(&total)

	r.render(w, "home.html", map[string]interface{}{
		"Title":"归来小说CMS - 首页","CatRecs":catRecs,"LatestList":listItems,
		"Ranking":ranking.Items,"Featured":safeSlice(ranking.Items,5),"Categories":cats,"Total":total,
	})
}

type catGroup struct{ Category models.Category; Novels []models.Novel }

func buildCatRecs(ctx context.Context, cats []models.Category) []catGroup {
	var recs []catGroup
	for i, cat := range cats {
		if i >= 6 { break }
		r, _ := services.ListNovels(ctx, services.NovelListParams{Page:1, Size:4, CategoryID:&cat.ID, SortBy:"total_chapters", SortDir:"desc"})
		if len(r.Items) > 0 { recs = append(recs, catGroup{cat, r.Items}) }
	}
	return recs
}

func mustCategories(ctx context.Context) []models.Category {
	rows, err := database.Pool.Query(ctx, "SELECT id,name,slug,sort_order,created_at,updated_at FROM categories ORDER BY sort_order")
	if err != nil { return nil }
	defer rows.Close()
	var cats []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID,&c.Name,&c.Slug,&c.SortOrder,&c.CreatedAt,&c.UpdatedAt); err != nil { continue }
		cats = append(cats, c)
	}
	return cats
}

func buildLatest(ctx context.Context, novels []models.Novel) []map[string]interface{} {
	if len(novels) == 0 { return nil }
	ids := make([]string, len(novels))
	for i, n := range novels { ids[i] = n.ID }
	rows, err := database.Pool.Query(ctx, `SELECT c.novel_id,c.id,c.title FROM chapters c INNER JOIN (SELECT novel_id,MAX(sort_order) m FROM chapters WHERE novel_id=ANY($1) GROUP BY novel_id) l ON c.novel_id=l.novel_id AND c.sort_order=l.m`, ids)
	if err != nil || rows == nil { return nil }
	defer rows.Close()
	type ci struct{ NID,ID,Title string }
	chMap := map[string]ci{}
	for rows.Next() { var x ci; rows.Scan(&x.NID,&x.ID,&x.Title); chMap[x.NID]=x }
	items := make([]map[string]interface{}, 0, len(novels))
	for _, n := range novels {
		catName := ""
		r2, _ := database.Pool.Query(ctx, "SELECT c.name FROM categories c JOIN novel_categories nc ON nc.category_id=c.id WHERE nc.novel_id=$1 LIMIT 1", n.ID)
		if r2 != nil { if r2.Next() { r2.Scan(&catName) }; r2.Close() }
		ch := chMap[n.ID]
		items = append(items, map[string]interface{}{
			"Novel":n,"CategoryName":catName,"LatestChapter":ch.Title,"LatestChapterID":ch.ID,
			"UpdatedMMDD":n.UpdatedAt.Format("01-02"),
		})
	}
	return items
}

// ── Book Library ────────────────────────────────────────────────────────
func (r *Router) bookLibrary(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 { page = 1 }; size := 48
	params := services.NovelListParams{Page:page, Size:size, SortBy:"updated_at", SortDir:"desc"}
	if c := req.URL.Query().Get("category"); c != "" { id, _ := strconv.Atoi(c); params.CategoryID = &id }
	if s := req.URL.Query().Get("status"); s != "" { params.Status = s }
	result, _ := services.ListNovels(ctx, params)
	cats := mustCategories(ctx)
	gridCards, libList := splitNovels(result.Items, 8)
	// Category-specific ranking
	rankParams := services.NovelListParams{Page:1, Size:10, SortBy:"total_chapters", SortDir:"desc"}
	if params.CategoryID != nil { rankParams.CategoryID = params.CategoryID }
	catRanking, _ := services.ListNovels(ctx, rankParams)
	r.render(w, "library.html", map[string]interface{}{
		"Title":"归来小说CMS - 书库","GridCards":gridCards,"LibraryList":libList,"Categories":cats,
		"Page":page,"Total":result.Total,"Pages":pagesFrom(result.Total, size),
		"CategoryID":params.CategoryID,"Novels":result.Items,"CatRanking":catRanking.Items,
	})
}

// ── Novel Detail ────────────────────────────────────────────────────────
func (r *Router) novelDetail(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	path := strings.TrimPrefix(req.URL.Path, "/novel/")
	if path == "" { http.NotFound(w, req); return }
	parts := strings.SplitN(path, "/", 2)
	novelID := parts[0]; isChList := len(parts) == 2 && parts[1] == "chapters"

	n, err := services.GetNovel(ctx, novelID)
	if err != nil { http.NotFound(w, req); return }

	if isChList {
		rows, err := pool.Query(ctx, "SELECT id,novel_id,title,content_file,volume,sort_order,word_count,source_url,is_published,created_at,updated_at FROM chapters WHERE novel_id=$1 ORDER BY sort_order ASC", novelID)
		if err != nil || rows == nil { http.NotFound(w, req); return }
		defer rows.Close()
		var all []models.Chapter
		for rows.Next() { var c models.Chapter; rows.Scan(&c.ID,&c.NovelID,&c.Title,&c.ContentFile,&c.Volume,&c.SortOrder,&c.WordCount,&c.SourceURL,&c.IsPublished,&c.CreatedAt,&c.UpdatedAt); all = append(all, c) }
		type vg struct{ Title string; Chapters []models.Chapter }
		var g []vg; cur := vg{Title:"正文"}
		for _, c := range all {
			v := strings.TrimSpace(c.Volume)
			if v != "" && cur.Title != v { if len(cur.Chapters)>0{g=append(g,cur)}; cur=vg{Title:v} }
			cur.Chapters = append(cur.Chapters, c)
		}
		g = append(g, cur)
		r.render(w, "chapter_list.html", map[string]interface{}{"Title":n.Title+" - 章节目录","Novel":n,"AllChapters":all,"Grouped":g})
		return
	}

	rows, err := pool.Query(ctx, "SELECT id,novel_id,title,content_file,volume,sort_order,word_count,source_url,is_published,created_at,updated_at FROM chapters WHERE novel_id=$1 ORDER BY sort_order DESC LIMIT 15", novelID)
	chs := []models.Chapter{}
	if err == nil && rows != nil { defer rows.Close(); for rows.Next() { var c models.Chapter; rows.Scan(&c.ID,&c.NovelID,&c.Title,&c.ContentFile,&c.Volume,&c.SortOrder,&c.WordCount,&c.SourceURL,&c.IsPublished,&c.CreatedAt,&c.UpdatedAt); chs = append(chs, c) } }
	cats := mustCategories(ctx)
	r.render(w, "novel.html", map[string]interface{}{"Title":n.Title+" - 归来小说CMS","Novel":n,"Chapters":chs,"Categories":cats})
}

// ── Chapter Reader ──────────────────────────────────────────────────────
func (r *Router) chapterRead(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	cid := strings.TrimPrefix(req.URL.Path, "/chapter/")
	if cid == "" { http.NotFound(w, req); return }
	var ch models.Chapter
	err := pool.QueryRow(ctx, "SELECT id,novel_id,title,content,content_file,volume,sort_order,word_count,source_url,is_published,created_at,updated_at FROM chapters WHERE id=$1", cid).
		Scan(&ch.ID,&ch.NovelID,&ch.Title,&ch.Content,&ch.ContentFile,&ch.Volume,&ch.SortOrder,&ch.WordCount,&ch.SourceURL,&ch.IsPublished,&ch.CreatedAt,&ch.UpdatedAt)
	if err != nil { http.NotFound(w, req); return }
	var n models.Novel
	pool.QueryRow(ctx, "SELECT id,title,author,description,cover_image_url,source_url,source_name,status,total_chapters,created_at,updated_at FROM novels WHERE id=$1", ch.NovelID).
		Scan(&n.ID,&n.Title,&n.Author,&n.Description,&n.CoverImageURL,&n.SourceURL,&n.SourceName,&n.Status,&n.TotalChapters,&n.CreatedAt,&n.UpdatedAt)
	var prev, next models.Chapter
	pool.QueryRow(ctx, "SELECT id,title FROM chapters WHERE novel_id=$1 AND sort_order<$2 ORDER BY sort_order DESC LIMIT 1", ch.NovelID, ch.SortOrder).Scan(&prev.ID,&prev.Title)
	pool.QueryRow(ctx, "SELECT id,title FROM chapters WHERE novel_id=$1 AND sort_order>$2 ORDER BY sort_order ASC LIMIT 1", ch.NovelID, ch.SortOrder).Scan(&next.ID,&next.Title)
	if c, _ := services.GetChapterContent(&ch); c != "" { ch.Content = c }
	r.render(w, "chapter.html", map[string]interface{}{
		"Title":ch.Title+" - "+n.Title,"Novel":n,"Chapter":ch,"PrevID":prev.ID,"PrevT":prev.Title,"NextID":next.ID,"NextT":next.Title,
	})
}

// ── Search ──────────────────────────────────────────────────────────────
func (r *Router) search(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	q := req.URL.Query().Get("q"); var results []models.Novel; var total int64
	if q != "" {
		like := "%"+q+"%"
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels WHERE LOWER(title) LIKE LOWER($1) OR LOWER(author) LIKE LOWER($2)", like, like).Scan(&total)
		rows, err := pool.Query(ctx, "SELECT id,title,author,description,cover_image_url,source_url,source_name,status,total_chapters,created_at,updated_at FROM novels WHERE LOWER(title) LIKE LOWER($1) OR LOWER(author) LIKE LOWER($2) ORDER BY updated_at DESC LIMIT 20", like, like)
		if err == nil && rows != nil { defer rows.Close(); for rows.Next() { var n models.Novel; rows.Scan(&n.ID,&n.Title,&n.Author,&n.Description,&n.CoverImageURL,&n.SourceURL,&n.SourceName,&n.Status,&n.TotalChapters,&n.CreatedAt,&n.UpdatedAt); results = append(results, n) } }
	}
	cats := mustCategories(ctx)
	r.render(w, "search.html", map[string]interface{}{"Title":"搜索: "+q+" - 归来小说CMS","Query":q,"Results":results,"Total":total,"Categories":cats})
}

// ── Helpers ─────────────────────────────────────────────────────────────
func (r *Router) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	if r.templates == nil { http.Error(w, "Template error", http.StatusInternalServerError); return }
	data["SiteName"]="归来小说CMS"; data["Lang"]="zh"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.templates.ExecuteTemplate(w, name, data)
}
func safeSlice(s []models.Novel, n int) []models.Novel { if len(s)<=n{return s}; return s[:n] }
type pageItem struct{ Page int; Label string; Active bool }

func paginateFn(current, total int) []pageItem {
	if total <= 9 {
		items := make([]pageItem, total)
		for i := 0; i < total; i++ { items[i] = pageItem{i+1, fmt.Sprintf("%d", i+1), i+1 == current} }
		return items
	}
	items := []pageItem{{1, "1", current == 1}}
	if current > 3 { items = append(items, pageItem{0, "...", false}) }
	start := max(2, current-1); end := min(total-1, current+1)
	if current <= 3 { start = 2; end = 4 }
	if current >= total-2 { start = total-3; end = total-1 }
	for i := start; i <= end; i++ { items = append(items, pageItem{i, fmt.Sprintf("%d", i), i == current}) }
	if current < total-2 { items = append(items, pageItem{0, "...", false}) }
	items = append(items, pageItem{total, fmt.Sprintf("%d", total), current == total})
	return items
}

func splitNovels(novels []models.Novel, n int) ([]models.Novel, []models.Novel) {
	if len(novels) <= n { return novels, nil }
	return novels[:n], novels[n:]
}
func pagesFrom(total int64, size int) int {
	if total==0{return 0}; p:=int(total)/size; if int(total)%size>0{p++}; return p
}
