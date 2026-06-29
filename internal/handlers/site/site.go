package site

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
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
		"seq": func(n int) []int { s := make([]int, n); for i := range s { s[i] = i + 1 }; return s },
		"or": func(a, b string) string { if a != "" { return a }; return b },
		"add": func(a, b int) int { return a + b },
		"gt": func(a, b int) bool { return a > b },
		"lt": func(a, b int) bool { return a < b },
		"eq": func(a, b interface{}) bool { return a == b },
		"truncate": func(s string, n int) string {
			r := []rune(s); if len(r) <= n { return s }; return string(r[:n]) + "..."
		},
		"statusLabel": func(s string) string {
			return map[string]string{"ongoing":"连载中","completed":"已完结","hiatus":"暂停更新"}[s]
		},
		"stripHTML": func(s string) string {
			s = strings.ReplaceAll(s, "<br>", "\n")
			s = strings.ReplaceAll(s, "<br/>", "\n")
			return strings.TrimSpace(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(s, ""))
		},
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

func (r *Router) home(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" { http.NotFound(w, req); return }
	ctx := req.Context()

	cats, _ := queryCategories(ctx)
	catRecs := queryCatRecs(ctx, cats)
	latestNovels, _ := queryLatestNovels(ctx, 30)
	listItems := buildLatestList(ctx, latestNovels)
	ranking, _ := queryRanking(ctx, 15)
	var total int64
	database.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels").Scan(&total)

	r.render(w, "home.html", map[string]interface{}{
		"Title":"归来小说CMS - 首页","CatRecs":catRecs,"LatestList":listItems,
		"Ranking":ranking,"Featured":safeSlice(ranking,5),"Categories":cats,"Total":total,
	})
}

func (r *Router) bookLibrary(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	page, _ := strconv.Atoi(req.URL.Query().Get("page"))
	if page < 1 { page = 1 }
	size := 30; var total int64
	where := " WHERE 1=1"; args := []interface{}{}; n := 1
	if c := req.URL.Query().Get("category"); c != "" {
		where += fmt.Sprintf(" AND n.id IN (SELECT novel_id FROM novel_categories WHERE category_id = $%d)", n); args = append(args, c); n++
	}
	if s := req.URL.Query().Get("status"); s != "" {
		where += fmt.Sprintf(" AND n.status = $%d", n); args = append(args, s); n++
	}
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels n"+where, args...).Scan(&total)
	args = append(args, size, (page-1)*size)
	rows, _ := pool.Query(ctx, fmt.Sprintf("SELECT n.id,n.title,n.author,n.description,n.cover_image_url,n.source_url,n.source_name,n.status,n.total_chapters,n.created_at,n.updated_at FROM novels n%s ORDER BY n.updated_at DESC LIMIT $%d OFFSET $%d", where, n, n+1), args...)
	novels, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.Novel])
	if rows != nil { rows.Close() }
	cats, _ := queryCategories(ctx)
	r.render(w, "home.html", map[string]interface{}{
		"Title":"归来小说CMS - 书库","Novels":novels,"Categories":cats,
		"Page":page,"Total":total,"Pages":pagesFromTotal(total,size),
		"Featured":safeSlice(novels,5),"Ranking":safeSlice(novels,15),
	})
}

func (r *Router) novelDetail(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	path := strings.TrimPrefix(req.URL.Path, "/novel/")
	if path == "" { http.NotFound(w, req); return }
	parts := strings.SplitN(path, "/", 2)
	novelID := parts[0]
	isChList := len(parts) == 2 && parts[1] == "chapters"

	n, err := services.GetNovel(ctx, novelID)
	if err != nil { http.NotFound(w, req); return }

	if isChList {
		rows, _ := pool.Query(ctx, "SELECT id,novel_id,title,content_file,volume,sort_order,word_count,source_url,is_published,created_at,updated_at FROM chapters WHERE novel_id=$1 ORDER BY sort_order ASC", novelID)
		all, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.Chapter])
		if rows != nil { rows.Close() }
		type vg struct{ Title string; Chapters []models.Chapter }
		var g []vg; cur := vg{Title:"正文"}
		for _, c := range all {
			v := strings.TrimSpace(c.Volume)
			if v != "" && cur.Title != v { if len(cur.Chapters)>0{g=append(g,cur)}; cur=vg{Title:v} }
			cur.Chapters = append(cur.Chapters, c)
		}
		g = append(g, cur)
		r.render(w, "chapter_list.html", map[string]interface{}{
			"Title":n.Title+" - 章节目录","Novel":n,"AllChapters":all,"Grouped":g,
		})
		return
	}

	rows, _ := pool.Query(ctx, "SELECT id,novel_id,title,content_file,volume,sort_order,word_count,source_url,is_published,created_at,updated_at FROM chapters WHERE novel_id=$1 ORDER BY sort_order DESC LIMIT 15", novelID)
	chs, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.Chapter])
	if rows != nil { rows.Close() }
	cats, _ := queryCategories(ctx)
	r.render(w, "novel.html", map[string]interface{}{
		"Title":n.Title+" - 归来小说CMS","Novel":n,"Chapters":chs,"Categories":cats,
	})
}

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
		"Title":ch.Title+" - "+n.Title,"Novel":n,"Chapter":ch,
		"PrevID":prev.ID,"PrevT":prev.Title,"NextID":next.ID,"NextT":next.Title,
	})
}

func (r *Router) search(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	q := req.URL.Query().Get("q"); var results []models.Novel; var total int64
	if q != "" {
		like := "%"+q+"%"
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels WHERE LOWER(title) LIKE LOWER($1) OR LOWER(author) LIKE LOWER($2)", like, like).Scan(&total)
		rows, _ := pool.Query(ctx, "SELECT id,title,author,description,cover_image_url,source_url,source_name,status,total_chapters,created_at,updated_at FROM novels WHERE LOWER(title) LIKE LOWER($1) OR LOWER(author) LIKE LOWER($2) ORDER BY updated_at DESC LIMIT 20", like, like)
		results, _ = pgx.CollectRows(rows, pgx.RowToStructByName[models.Novel])
		if rows != nil { rows.Close() }
	}
	cats, _ := queryCategories(ctx)
	r.render(w, "search.html", map[string]interface{}{
		"Title":"搜索: "+q+" - 归来小说CMS","Query":q,"Results":results,"Total":total,"Categories":cats,
	})
}

// ── query helpers ──────────────────────────────────────────────────────────

func queryCategories(ctx context.Context) ([]models.Category, error) {
	rows, err := database.Pool.Query(ctx, "SELECT id,name,slug,sort_order,created_at,updated_at FROM categories ORDER BY sort_order")
	if err != nil { return nil, err }
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Category])
}

func queryLatestNovels(ctx context.Context, limit int) ([]models.Novel, error) {
	rows, err := database.Pool.Query(ctx, "SELECT id,title,author,description,cover_image_url,source_url,source_name,status,total_chapters,created_at,updated_at FROM novels ORDER BY updated_at DESC LIMIT $1", limit)
	if err != nil { return nil, err }
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Novel])
}

func queryRanking(ctx context.Context, limit int) ([]models.Novel, error) {
	rows, err := database.Pool.Query(ctx, "SELECT id,title,author,description,cover_image_url,source_url,source_name,status,total_chapters,created_at,updated_at FROM novels ORDER BY total_chapters DESC LIMIT $1", limit)
	if err != nil { return nil, err }
	defer rows.Close()
	return pgx.CollectRows(rows, pgx.RowToStructByName[models.Novel])
}

type CatRecGroup struct{ Category models.Category; Novels []models.Novel }

func queryCatRecs(ctx context.Context, cats []models.Category) []CatRecGroup {
	recs := make([]CatRecGroup, 0)
	for i, cat := range cats {
		if i >= 6 { break }
		rows, _ := database.Pool.Query(ctx, `SELECT n.id,n.title,n.author,n.description,n.cover_image_url,n.source_url,n.source_name,n.status,n.total_chapters,n.created_at,n.updated_at FROM novels n JOIN novel_categories nc ON nc.novel_id=n.id WHERE nc.category_id=$1 ORDER BY n.total_chapters DESC LIMIT 4`, cat.ID)
		novels, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.Novel])
		if rows != nil { rows.Close() }
		if len(novels) > 0 { recs = append(recs, CatRecGroup{cat, novels}) }
	}
	return recs
}

func buildLatestList(ctx context.Context, novels []models.Novel) []map[string]interface{} {
	if len(novels) == 0 { return nil }
	ids := make([]string, len(novels))
	for i, n := range novels { ids[i] = n.ID }
	rows, _ := database.Pool.Query(ctx, `SELECT c.novel_id,c.id,c.title FROM chapters c INNER JOIN (SELECT novel_id,MAX(sort_order) m FROM chapters WHERE novel_id=ANY($1) GROUP BY novel_id) l ON c.novel_id=l.novel_id AND c.sort_order=l.m`, ids)
	type ci struct{ NID,ID,Title string }
	chMap := map[string]ci{}
	if rows != nil {
		for rows.Next() { var x ci; rows.Scan(&x.NID,&x.ID,&x.Title); chMap[x.NID]=x }
		rows.Close()
	}
	items := make([]map[string]interface{}, 0, len(novels))
	for _, n := range novels {
		catName := ""
		r2, _ := database.Pool.Query(ctx, "SELECT c.name FROM categories c JOIN novel_categories nc ON nc.category_id=c.id WHERE nc.novel_id=$1 LIMIT 1", n.ID)
		if r2 != nil && r2.Next() { r2.Scan(&catName); r2.Close() }
		ch := chMap[n.ID]
		items = append(items, map[string]interface{}{
			"Novel":n,"CategoryName":catName,"LatestChapter":ch.Title,"LatestChapterID":ch.ID,
			"UpdatedMMDD":n.UpdatedAt.Format("01-02"),
		})
	}
	return items
}

func (r *Router) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	if r.templates == nil { http.Error(w, "Template error", http.StatusInternalServerError); return }
	data["SiteName"]="归来小说CMS"; data["Lang"]="zh"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.templates.ExecuteTemplate(w, name, data)
}

func safeSlice(s []models.Novel, n int) []models.Novel { if len(s)<=n{return s}; return s[:n] }
func pagesFromTotal(total int64, size int) int { if total==0{return 0}; p:=int(total)/size; if int(total)%size>0{p++}; return p }
