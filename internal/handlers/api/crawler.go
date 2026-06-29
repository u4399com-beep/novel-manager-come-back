package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"github.com/jackc/pgx/v5"
	"github.com/u4399com-beep/novel-manager-come-back/internal/database"
	"github.com/u4399com-beep/novel-manager-come-back/internal/models"
)

func (r *Router) handleSources(w http.ResponseWriter, req *http.Request) {
	writeOK(w, []map[string]string{{"source_name":"23qb","base_url":"https://www.23qb.net","description":"铅笔小说"}})
}

func (r *Router) handleCrawlTrigger(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost { writeError(w, 405, "POST required"); return }
	var b struct{ NovelID, SourceName, RuleName, Mode string }
	if err := json.NewDecoder(req.Body).Decode(&b); err != nil { writeError(w, 400, "invalid JSON"); return }
	if b.NovelID == "" { writeError(w, 400, "novel_id required"); return }
	if b.Mode == "" { b.Mode = "direct" }
	if b.RuleName == "" { b.RuleName = b.SourceName }
	if b.RuleName == "" { b.RuleName = "23qb" }
	ctx := req.Context(); pool := database.Pool
	var exists bool
	pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM novels WHERE id=$1)", b.NovelID).Scan(&exists)
	if !exists { writeError(w, 404, "novel not found"); return }
	var t models.CrawlerTask
	pool.QueryRow(ctx, "INSERT INTO crawler_tasks (novel_id,status,rule_name) VALUES ($1,'pending',$2) RETURNING id,novel_id,status,rule_name,chapters_found,chapters_added,created_at,updated_at", b.NovelID, b.RuleName).
		Scan(&t.ID,&t.NovelID,&t.Status,&t.RuleName,&t.ChaptersFound,&t.ChaptersAdded,&t.CreatedAt,&t.UpdatedAt)
	writeJSON(w, 202, t)
}

func (r *Router) handleCrawlTasks(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	q := req.URL.Query(); page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 { page = 1 }; size, _ := strconv.Atoi(q.Get("size"))
	if size < 1 || size > 100 { size = 20 }
	where := " WHERE 1=1"; args := []interface{}{}; n := 1
	if nid := q.Get("novel_id"); nid != "" { where += " AND novel_id=$"+strconv.Itoa(n); args = append(args, nid); n++ }
	if st := q.Get("status"); st != "" { where += " AND status=$"+strconv.Itoa(n); args = append(args, st); n++ }
	var total int64
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM crawler_tasks"+where, args...).Scan(&total)
	args = append(args, size, (page-1)*size)
	rows, _ := pool.Query(ctx, "SELECT id,novel_id,status,rule_name,chapters_found,chapters_added,error_message,started_at,finished_at,created_at,updated_at FROM crawler_tasks"+where+" ORDER BY created_at DESC LIMIT $"+strconv.Itoa(n)+" OFFSET $"+strconv.Itoa(n+1), args...)
	tasks, _ := pgx.CollectRows(rows, pgx.RowToStructByName[models.CrawlerTask])
	if rows != nil { rows.Close() }
	writeOK(w, map[string]interface{}{"items":tasks,"total":total,"page":page,"size":size,"pages":calcPages(total,int64(size))})
}

func (r *Router) handleCrawlTaskByID(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	path := strings.TrimPrefix(req.URL.Path, r.cfg.APIPrefix+"/crawler/tasks/")
	parts := strings.SplitN(path, "/", 2); tid := parts[0]
	action := ""; if len(parts) > 1 { action = parts[1] }
	var t models.CrawlerTask
	err := pool.QueryRow(ctx, "SELECT id,novel_id,status,chapters_found,chapters_added,error_message,started_at,finished_at,created_at,updated_at FROM crawler_tasks WHERE id=$1", tid).
		Scan(&t.ID,&t.NovelID,&t.Status,&t.ChaptersFound,&t.ChaptersAdded,&t.ErrorMessage,&t.StartedAt,&t.FinishedAt,&t.CreatedAt,&t.UpdatedAt)
	if err != nil { writeError(w, 404, "not found"); return }
	switch req.Method {
	case http.MethodGet: writeOK(w, t)
	case http.MethodDelete:
		if t.Status == "running" { writeError(w, 409, "cannot delete running"); return }
		pool.Exec(ctx, "DELETE FROM crawler_tasks WHERE id=$1", tid); w.WriteHeader(204)
	case http.MethodPost:
		switch action {
		case "start": pool.Exec(ctx, "UPDATE crawler_tasks SET status='running' WHERE id=$1 AND status='pending'", tid)
		case "stop": pool.Exec(ctx, "UPDATE crawler_tasks SET status='failed',error_message='manually stopped' WHERE id=$1", tid)
		case "retry": pool.Exec(ctx, "UPDATE crawler_tasks SET status='pending',error_message=NULL WHERE id=$1", tid)
		default: writeError(w, 404, "unknown action"); return
		}
		pool.QueryRow(ctx, "SELECT id,novel_id,status,chapters_found,chapters_added,error_message,started_at,finished_at,created_at,updated_at FROM crawler_tasks WHERE id=$1", tid).
			Scan(&t.ID,&t.NovelID,&t.Status,&t.ChaptersFound,&t.ChaptersAdded,&t.ErrorMessage,&t.StartedAt,&t.FinishedAt,&t.CreatedAt,&t.UpdatedAt)
		writeJSON(w, 202, t)
	default: writeError(w, 405, "GET/DELETE/POST required")
	}
}

func (r *Router) handleCrawlStats(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context(); pool := database.Pool
	var nv, ch, tt, pd int64
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM novels").Scan(&nv)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM chapters").Scan(&ch)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM crawler_tasks").Scan(&tt)
	pool.QueryRow(ctx, "SELECT COUNT(*) FROM crawler_tasks WHERE status='pending'").Scan(&pd)
	writeOK(w, map[string]interface{}{"novels":nv,"chapters":ch,"tasks_total":tt,"tasks_pending":pd})
}

func calcPages(total, size int64) int { if total==0{return 0}; p:=int(total/size); if total%size>0{p++}; return p }
