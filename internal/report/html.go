// Package report 把评测结果渲染成可读的 HTML。
package report

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"roleeval/internal/drift"
	"roleeval/internal/extract"
	"roleeval/internal/metrics"
	"roleeval/internal/model"
)

// Item 是一次会话的评测结果。
type Item struct {
	Conv           model.Conversation
	Findings       []model.Finding
	Summary        metrics.Summary
	Facts          []extract.Fact
	Profile        *model.PersonaProfile
	Drift          *drift.Series
	FindingsByTurn map[int][]model.Finding
}

// Data 是整份报告。
type Data struct {
	Items     []Item
	Generated string
	Source    string
}

// NewItem 组装 Item（不带画像与漂移）。
func NewItem(conv model.Conversation, findings []model.Finding, facts []extract.Fact) Item {
	return NewItemWith(conv, findings, facts, nil, nil)
}

// NewItemWith 组装 Item，并建立按轮次的索引。
func NewItemWith(conv model.Conversation, findings []model.Finding, facts []extract.Fact,
	profile *model.PersonaProfile, series *drift.Series) Item {
	byTurn := map[int][]model.Finding{}
	for _, f := range findings {
		byTurn[f.Turn] = append(byTurn[f.Turn], f)
	}
	return Item{
		Conv:           conv,
		Findings:       findings,
		Summary:        metrics.Compute(conv, findings),
		Facts:          facts,
		Profile:        profile,
		Drift:          series,
		FindingsByTurn: byTurn,
	}
}

// Write 输出 HTML 文件。
func Write(path string, d Data) error {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"join":     strings.Join,
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
	}).Parse(htmlTemplate)
	if err != nil {
		return err
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return tmpl.Execute(f, d)
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>RoleEval 评测报告</title>
<style>
:root{--bg:#0d1117;--panel:#161b22;--text:#c9d1d9;--heading:#f0f6fc;--muted:#8b949e;--faint:#6e7681;--border:#30363d;--accent:#58a6ff;--green:#3fb950;--yellow:#d29922;--red:#f85149}
*{box-sizing:border-box}
html{background:var(--bg)}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif;-webkit-font-smoothing:antialiased}
article{max-width:1000px;margin:0 auto;padding:44px 24px 90px}
h1{font-size:1.9em;margin:0 0 .4em;color:var(--heading);font-weight:650}
h2{font-size:1.35em;margin:2.2em 0 .7em;padding-bottom:.3em;border-bottom:1px solid var(--border);color:var(--heading);font-weight:650}
h3{font-size:1.05em;color:var(--muted);margin:1.6em 0 .5em;font-weight:650}
p{margin:.6em 0}
code{font-family:"Cascadia Code",Consolas,monospace;font-size:.88em;background:var(--panel);padding:.12em .35em;border-radius:4px}
.meta{color:var(--muted);font-size:.9em}
.cards{display:flex;flex-wrap:wrap;gap:10px;margin:1.2em 0}
.card{flex:1 1 150px;background:var(--panel);border:1px solid var(--border);border-radius:10px;padding:12px 14px}
.card .k{font-size:.75em;color:var(--faint);letter-spacing:.05em}
.card .v{font-size:1.5em;font-weight:650;color:var(--heading);line-height:1.3}
table{width:100%;border-collapse:collapse;margin:.8em 0;font-size:.92em}
th,td{border:1px solid var(--border);padding:7px 10px;text-align:left;vertical-align:top}
th{background:var(--panel);font-weight:650;color:var(--heading)}
tbody tr:nth-child(even){background:rgba(255,255,255,.015)}
.turn{border:1px solid var(--border);border-radius:10px;padding:12px 14px;margin:10px 0;background:var(--panel)}
.turn.user{border-left:4px solid var(--accent)}
.turn.assistant{border-left:4px solid var(--green)}
.turn.bad{border-left-color:var(--red)}
.turn-head{font-size:.8em;color:var(--faint);margin-bottom:6px;font-family:Consolas,monospace}
.turn-text{white-space:pre-wrap;word-break:break-word}
.badge{display:inline-block;font-size:.72em;border-radius:5px;padding:.1em .45em;margin:.15em .3em .15em 0;border:1px solid var(--border);color:var(--muted);font-family:Consolas,monospace}
.badge.sev1{color:var(--yellow);border-color:#9e6a03;background:rgba(158,106,3,.12)}
.badge.sev2{color:#f0883e;border-color:#bd561d;background:rgba(189,86,29,.14)}
.badge.sev3{color:var(--red);border-color:#da3633;background:rgba(218,54,51,.14)}
.evidence{font-size:.82em;color:var(--muted);margin-top:6px;padding-left:8px;border-left:2px solid var(--border)}
.empty{color:var(--green)}
footer{margin-top:3em;color:var(--faint);font-size:.82em}
@media print{body{background:#fff;color:#000}.turn{break-inside:avoid}}
</style>
</head>
<body>
<article>
<h1>RoleEval 评测报告</h1>
<p class="meta">生成时间：{{.Generated}} · 来源：{{.Source}} · 会话数：{{len .Items}}</p>

{{range .Items}}
{{$conv := .}}
<h2>会话 {{.Conv.ID}}</h2>
<p class="meta">角色 {{.Conv.CharacterID}} · {{.Summary.TotalTurns}} 轮 · {{.Summary.TotalFindings}} 条发现</p>

<div class="cards">
  <div class="card"><div class="k">命中轮次占比</div><div class="v">{{pct .Summary.ErrorRate}}</div></div>
  <div class="card"><div class="k">受影响轮次</div><div class="v">{{.Summary.AffectedTurns}} / {{.Summary.TotalTurns}}</div></div>
  <div class="card"><div class="k">问题条数</div><div class="v">{{.Summary.TotalFindings}}</div></div>
  <div class="card"><div class="k">规则 / 模型</div><div class="v">{{index .Summary.ByMethod "rule"}} / {{index .Summary.ByMethod "llm"}}</div></div>
</div>

<h3>按分类</h3>
{{if .Summary.ByCategory}}
<table>
<thead><tr><th style="width:52%">分类</th><th style="width:14%">条数</th><th style="width:34%">最严重的子类</th></tr></thead>
<tbody>
{{range $k, $v := .Summary.ByCategory}}
<tr><td><code>{{$k}}</code></td><td>{{$v}}</td><td class="meta">{{index $conv.Summary.BySub $k}}</td></tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">没有命中任何规则问题。</p>
{{end}}

<h3>按严重度</h3>
<table>
<thead><tr><th>严重度</th><th>条数</th><th>含义</th></tr></thead>
<tbody>
<tr><td>3</td><td>{{index .Summary.BySeverity "3"}}</td><td>破坏体验</td></tr>
<tr><td>2</td><td>{{index .Summary.BySeverity "2"}}</td><td>明显问题</td></tr>
<tr><td>1</td><td>{{index .Summary.BySeverity "1"}}</td><td>轻微</td></tr>
</tbody>
</table>

{{if .Facts}}
<h3>事实表（用于记忆对账）</h3>
<table>
<thead><tr><th style="width:12%">轮次</th><th style="width:22%">类型</th><th>值</th></tr></thead>
<tbody>
{{range .Facts}}<tr><td>{{.Turn}}</td><td><code>{{.Key}}</code></td><td>{{.Value}}</td></tr>{{end}}
</tbody>
</table>
{{end}}

{{if .Profile}}
<h3>人物画像（基线：前 {{.Profile.BaselineTurns}} 条，来源 {{.Profile.Source}}）</h3>
<table>
<tbody>
<tr><th style="width:22%">自称</th><td>{{.Profile.Self}}</td></tr>
<tr><th>对用户称呼</th><td>{{.Profile.AddressToUser}}</td></tr>
<tr><th>口癖</th><td>{{join .Profile.Catchphrases "、"}}</td></tr>
<tr><th>语气</th><td>{{.Profile.Tone}}</td></tr>
<tr><th>价值观</th><td>{{join .Profile.Values "、"}}</td></tr>
<tr><th>禁忌</th><td>{{join .Profile.Taboos "、"}}</td></tr>
<tr><th>平均句长</th><td>{{.Profile.AvgLen}}</td></tr>
</tbody>
</table>
{{end}}

{{if .Drift}}
<h3>风格漂移</h3>
<table>
<thead><tr><th>轮次</th><th>字数</th><th>Z</th><th>中文占比</th><th>元话语</th><th>口癖</th><th>标记</th></tr></thead>
<tbody>
{{range .Drift.Points}}
<tr><td>{{.Turn}}</td><td>{{.Len}}</td><td>{{.LenZ}}</td><td>{{.CJK}}</td><td>{{.Meta}}</td><td>{{.Catch}}</td><td>{{join .Flags " / "}}</td></tr>
{{end}}
</tbody>
</table>
{{end}}

<h3>逐轮时间线</h3>
{{range .Conv.Turns}}
{{$fs := index $conv.FindingsByTurn .Seq}}
<div class="turn {{roleClass .}}{{if $fs}} bad{{end}}">
  <div class="turn-head">第 {{.Seq}} 轮 · {{roleName .}}{{if .ModelName}} · {{.ModelName}}{{end}}{{if .Kind}} · {{.Kind}}{{end}}</div>
  {{if .IsUser}}
  <div class="turn-text">{{.UserText}}</div>
  {{else}}
  <div class="turn-text">{{.AssistantText}}</div>
  {{end}}
  {{range $fs}}
  <div class="evidence">
    <span class="badge {{sevClass .Severity}}">{{.Category}}{{if .Sub}} / {{.Sub}}{{end}} · {{.Method}}{{if .Votes}} · {{.Votes}}/{{.Samples}}{{end}}</span>
    {{if .Reason}}{{.Reason}}{{end}}
    {{if .Evidence}}<br>「{{.Evidence}}」{{end}}
  </div>
  {{end}}
</div>
{{end}}

{{end}}

<footer>由 roleeval 生成 · 规则指标可复现，模型判定需抽样人工复核</footer>
</article>
</body>
</html>`
