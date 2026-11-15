// Package web 提供一个可以现场演示的本地 Web 界面：
// 上传抓包 → 选会话 → 跑评测 → 看时间线上的问题。
package web

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"sync"
	"time"

	"roleeval/internal/collect"
	"roleeval/internal/extract"
	"roleeval/internal/fuse"
	"roleeval/internal/judge"
	"roleeval/internal/model"
	"roleeval/internal/report"
)

// Server 是演示服务的状态。
type Server struct {
	mu      sync.Mutex
	convs   []model.Conversation
	results map[string]report.Item
	cfg     judge.Config
	useLLM  bool
	source  string
	message string
}

// New 创建服务。
func New(convs []model.Conversation, cfg judge.Config, useLLM bool, source string) *Server {
	s := &Server{
		convs:   convs,
		results: map[string]report.Item{},
		cfg:     cfg,
		useLLM:  useLLM,
		source:  source,
	}
	// 启动时先跑一遍规则，打开首页就有东西看
	for _, c := range convs {
		s.results[c.ID] = s.evaluate(c, false)
	}
	return s
}

// Routes 注册路由。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/conv", s.handleConv)
	mux.HandleFunc("/eval", s.handleEval)
	mux.HandleFunc("/import", s.handleImport)
	return mux
}

/* -------------------------------- pages --------------------------------- */

type convRow struct {
	ID          string
	CharacterID int64
	Turns       int
	Findings    int
	ErrorRate   float64
	Evaluated   bool
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	rows := make([]convRow, 0, len(s.convs))
	for _, c := range s.convs {
		row := convRow{ID: c.ID, CharacterID: c.CharacterID, Turns: len(c.Turns)}
		if it, ok := s.results[c.ID]; ok {
			row.Evaluated = true
			row.Findings = len(it.Findings)
			row.ErrorRate = it.Summary.ErrorRate
		}
		rows = append(rows, row)
	}
	// 问题多的排前面，演示时一眼看到重点
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Findings > rows[j].Findings })

	render(w, tplIndex, map[string]any{
		"Title":   "RoleEval · 会话列表",
		"CSS":     template.CSS(css),
		"Rows":    rows,
		"Message": s.message,
		"LLMOn":   s.useLLM && s.cfg.Ready(),
		"Source":  s.source,
	})
	s.message = ""
}

func (s *Server) handleConv(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := r.URL.Query().Get("id")
	item, ok := s.results[id]
	if !ok {
		http.NotFound(w, r)
		return
	}

	filter := r.URL.Query().Get("cat")
	byTurn := map[int][]model.Finding{}
	var shown int
	for _, f := range item.Findings {
		if filter != "" && f.Category != filter {
			continue
		}
		byTurn[f.Turn] = append(byTurn[f.Turn], f)
		shown++
	}

	render(w, tplDetail, map[string]any{
		"Title":  "RoleEval · " + id,
		"CSS":    template.CSS(css),
		"Item":   item,
		"ByTurn": byTurn,
		"Filter": filter,
		"Shown":  shown,
		"LLMOn":  s.useLLM && s.cfg.Ready(),
	})
}

/* ------------------------------- actions --------------------------------- */

func (s *Server) handleEval(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")

	s.mu.Lock()
	var target *model.Conversation
	for i := range s.convs {
		if s.convs[i].ID == id {
			target = &s.convs[i]
			break
		}
	}
	useLLM := s.useLLM && s.cfg.Ready()
	s.mu.Unlock()

	if target == nil {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.mu.Lock()
	item := s.evaluateWithLLM(ctx, *target, useLLM)
	s.results[target.ID] = item
	s.message = fmt.Sprintf("已重新评测 %s：%d 条发现", shortID(target.ID), len(item.Findings))
	s.mu.Unlock()

	http.Redirect(w, r, "/conv?id="+id, http.StatusSeeOther)
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "缺少文件字段 file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil {
		http.Error(w, "读取文件失败", http.StatusBadRequest)
		return
	}
	convs, err := collect.Parse(raw)
	if err != nil {
		http.Error(w, "解析失败: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	index := map[string]bool{}
	for _, c := range s.convs {
		index[c.ID] = true
	}
	added := 0
	for _, c := range convs {
		if c.ID != "" && index[c.ID] {
			continue
		}
		s.convs = append(s.convs, c)
		index[c.ID] = true
		s.results[c.ID] = s.evaluate(c, false)
		added++
	}
	s.message = fmt.Sprintf("导入 %d 个会话（去重后）", added)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

/* ------------------------------- helpers --------------------------------- */

func (s *Server) evaluate(c model.Conversation, useLLM bool) report.Item {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return s.evaluateWithLLM(ctx, c, useLLM)
}

func (s *Server) evaluateWithLLM(ctx context.Context, c model.Conversation, useLLM bool) report.Item {
	findings, facts := extract.Run(c)
	if useLLM {
		if out, err := s.cfg.JudgeStable(ctx, c); err == nil {
			findings = append(findings, out.Findings...)
		}
	}
	findings = fuse.Merge(findings)
	return report.NewItem(c, findings, facts)
}

func render(w http.ResponseWriter, tpl string, data map[string]any) {
	t, err := template.New("page").Funcs(template.FuncMap{
		"pct":      func(f float64) string { return fmt.Sprintf("%.1f%%", f*100) },
		"sevClass": func(s int) string { return fmt.Sprintf("sev%d", s) },
		"roleName": func(t model.Turn) string {
			if t.IsUser() {
				return "用户"
			}
			return "角色"
		},
		"roleClass": func(t model.Turn) string {
			if t.IsUser() {
				return "user"
			}
			return "assistant"
		},
		"catCount": func(m map[string]int, k string) int { return m[k] },
	}).Parse(tpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, data)
}

func shortID(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}
