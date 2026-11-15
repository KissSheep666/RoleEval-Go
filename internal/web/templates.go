package web

const css = `
:root{--bg:#0d1117;--panel:#161b22;--panel2:#1c2129;--text:#c9d1d9;--heading:#f0f6fc;--muted:#8b949e;--faint:#6e7681;--border:#30363d;--accent:#58a6ff;--green:#3fb950;--yellow:#d29922;--red:#f85149}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--text);font:15px/1.7 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif}
a{color:var(--accent);text-decoration:none}
a:hover{text-decoration:underline}
header.top{border-bottom:1px solid var(--border);background:var(--panel)}
.top-inner{max-width:1080px;margin:0 auto;padding:16px 24px;display:flex;align-items:baseline;gap:16px;flex-wrap:wrap}
.brand{font-size:1.15em;font-weight:700;color:var(--heading)}
.brand span{color:var(--accent)}
.sub{color:var(--muted);font-size:.85em}
main{max-width:1080px;margin:0 auto;padding:28px 24px 80px}
h1{font-size:1.6em;margin:0 0 .3em;color:var(--heading)}
h2{font-size:1.15em;margin:1.8em 0 .6em;color:var(--heading)}
.muted{color:var(--muted)}
.banner{background:rgba(63,185,80,.12);border:1px solid #238636;color:#7ee787;border-radius:8px;padding:9px 12px;margin:0 0 18px;font-size:.9em}
.cards{display:flex;gap:10px;flex-wrap:wrap;margin:14px 0 6px}
.card{flex:1 1 150px;background:var(--panel);border:1px solid var(--border);border-radius:10px;padding:12px 14px}
.card .k{font-size:.72em;color:var(--faint);letter-spacing:.05em}
.card .v{font-size:1.5em;font-weight:700;color:var(--heading);line-height:1.3}
table{width:100%;border-collapse:collapse;margin:12px 0;font-size:.92em}
th,td{border:1px solid var(--border);padding:9px 12px;text-align:left;vertical-align:middle}
th{background:var(--panel);color:var(--heading);font-weight:650}
tbody tr:nth-child(even){background:rgba(255,255,255,.015)}
tbody tr:hover{background:rgba(88,166,255,.06)}
.btn{display:inline-block;border:1px solid var(--border);background:var(--panel2);color:var(--text);border-radius:7px;padding:5px 11px;font-size:.85em;cursor:pointer;font-family:inherit}
.btn:hover{border-color:var(--accent);color:var(--heading);text-decoration:none}
.btn.primary{background:#1f6feb;border-color:#1f6feb;color:#fff}
.btn.primary:hover{background:#388bfd}
.pill{display:inline-block;font-size:.72em;padding:.1em .5em;border-radius:999px;border:1px solid var(--border);color:var(--muted);margin-right:6px}
.pill.on{color:var(--heading);border-color:var(--accent);background:rgba(88,166,255,.12)}
.turn{border:1px solid var(--border);border-radius:10px;padding:12px 14px;margin:10px 0;background:var(--panel)}
.turn.user{border-left:4px solid var(--accent)}
.turn.assistant{border-left:4px solid var(--green)}
.turn.bad{border-left-color:var(--red);background:linear-gradient(90deg,rgba(248,81,73,.07),var(--panel) 40%)}
.turn-head{font-size:.78em;color:var(--faint);margin-bottom:6px;font-family:Consolas,monospace}
.turn-text{white-space:pre-wrap;word-break:break-word}
.evidence{font-size:.84em;color:var(--muted);margin-top:8px;padding-left:9px;border-left:2px solid var(--border)}
.badge{display:inline-block;font-size:.72em;border-radius:5px;padding:.1em .45em;margin:.15em .3em .15em 0;border:1px solid var(--border);color:var(--muted);font-family:Consolas,monospace}
.badge.sev1{color:var(--yellow);border-color:#9e6a03;background:rgba(158,106,3,.12)}
.badge.sev2{color:#f0883e;border-color:#bd561d;background:rgba(189,86,29,.14)}
.badge.sev3{color:var(--red);border-color:#da3633;background:rgba(218,54,51,.14)}
.empty{color:var(--green)}
.upload{background:var(--panel);border:1px dashed var(--border);border-radius:10px;padding:14px 16px;margin:18px 0;display:flex;gap:12px;align-items:center;flex-wrap:wrap}
footer{margin-top:40px;color:var(--faint);font-size:.8em}
@media print{body{background:#fff;color:#000}.turn{break-inside:avoid}.btn,.upload,.top{display:none}}
`

const tplIndex = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title><style>{{.CSS}}</style></head>
<body>
<header class="top"><div class="top-inner">
  <div class="brand">Role<span>Eval</span></div>
  <div class="sub">Roleplay 对话离线评测 · {{.Source}}</div>
  <div class="sub" style="margin-left:auto">{{if .LLMOn}}LLM judge 已启用{{else}}仅规则评测{{end}}</div>
</div></header>
<main>
  {{if .Message}}<div class="banner">{{.Message}}</div>{{end}}
  <h1>会话列表</h1>
  <p class="muted">选一个会话看时间线；问题越多的排在越前面。</p>

  <form class="upload" method="post" action="/import" enctype="multipart/form-data">
    <strong>导入抓包</strong>
    <span class="muted" style="font-size:.85em">支持油猴脚本导出的 JSON，或 turn-list 原始响应</span>
    <input type="file" name="file" accept=".json,application/json" required>
    <button class="btn primary" type="submit">上传并解析</button>
  </form>

  <table>
    <thead><tr><th style="width:30%">会话</th><th>角色</th><th>轮数</th><th>发现</th><th>命中轮次</th><th style="width:24%">操作</th></tr></thead>
    <tbody>
    {{range .Rows}}
    <tr>
      <td><a href="/conv?id={{.ID}}"><code>{{.ID}}</code></a></td>
      <td>{{.CharacterID}}</td>
      <td>{{.Turns}}</td>
      <td>{{if .Evaluated}}{{.Findings}}{{else}}—{{end}}</td>
      <td>{{if .Evaluated}}{{pct .ErrorRate}}{{else}}—{{end}}</td>
      <td>
        <a class="btn" href="/conv?id={{.ID}}">查看</a>
        <form method="post" action="/eval?id={{.ID}}" style="display:inline">
          <button class="btn" type="submit">重新评测</button>
        </form>
      </td>
    </tr>
    {{else}}
    <tr><td colspan="6" class="muted">还没有会话，先上传一份抓包。</td></tr>
    {{end}}
    </tbody>
  </table>
  <footer>规则指标可复现；模型判定建议抽样人工复核。</footer>
</main></body></html>`

const tplDetail = `<!DOCTYPE html>
<html lang="zh-CN"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title><style>{{.CSS}}</style></head>
<body>
<header class="top"><div class="top-inner">
  <a class="brand" href="/">Role<span>Eval</span></a>
  <div class="sub">会话 <code>{{.Item.Conv.ID}}</code></div>
  <div class="sub" style="margin-left:auto">{{if .LLMOn}}LLM judge 已启用{{else}}仅规则评测{{end}}</div>
</div></header>
<main>
  <p><a href="/">← 返回列表</a></p>
  <h1>角色 {{.Item.Conv.CharacterID}} · {{.Item.Summary.TotalTurns}} 轮</h1>
  <p class="muted">{{.Item.Summary.TotalFindings}} 条发现，影响 {{.Item.Summary.AffectedTurns}} 轮（{{pct .Item.Summary.ErrorRate}}）</p>

  <form method="post" action="/eval?id={{.Item.Conv.ID}}" style="margin:10px 0">
    <button class="btn primary" type="submit">{{if .LLMOn}}用规则 + LLM 重新评测{{else}}重新评测{{end}}</button>
  </form>

  <div class="cards">
    <div class="card"><div class="k">命中轮次</div><div class="v">{{pct .Item.Summary.ErrorRate}}</div></div>
    <div class="card"><div class="k">受影响轮次</div><div class="v">{{.Item.Summary.AffectedTurns}} / {{.Item.Summary.TotalTurns}}</div></div>
    <div class="card"><div class="k">规则</div><div class="v">{{index .Item.Summary.ByMethod "rule"}}</div></div>
    <div class="card"><div class="k">模型</div><div class="v">{{index .Item.Summary.ByMethod "llm"}}</div></div>
  </div>

  <h2>按分类筛选</h2>
  <p>
    <a class="pill {{if eq .Filter ""}}on{{end}}" href="/conv?id={{.Item.Conv.ID}}">全部</a>
    {{range $k, $v := .Item.Summary.ByCategory}}
      <a class="pill {{if eq $.Filter $k}}on{{end}}" href="/conv?id={{$.Item.Conv.ID}}&cat={{$k}}">{{$k}} · {{$v}}</a>
    {{end}}
  </p>
  {{if .Filter}}<p class="muted">当前筛选：<code>{{.Filter}}</code>，{{.Shown}} 条</p>{{end}}

  {{if .Item.Facts}}
  <h2>事实表</h2>
  <table>
    <thead><tr><th style="width:12%">轮次</th><th style="width:24%">类型</th><th>值</th></tr></thead>
    <tbody>{{range .Item.Facts}}<tr><td>{{.Turn}}</td><td><code>{{.Key}}</code></td><td>{{.Value}}</td></tr>{{end}}</tbody>
  </table>
  {{end}}

  <h2>时间线</h2>
  {{range .Item.Conv.Turns}}
  {{$fs := index $.ByTurn .Seq}}
  <div class="turn {{roleClass .}}{{if $fs}} bad{{end}}">
    <div class="turn-head">第 {{.Seq}} 轮 · {{roleName .}}{{if .ModelName}} · {{.ModelName}}{{end}}{{if .Kind}} · {{.Kind}}{{end}}</div>
    {{if .IsUser}}<div class="turn-text">{{.UserText}}</div>{{else}}<div class="turn-text">{{.AssistantText}}</div>{{end}}
    {{range $fs}}
    <div class="evidence">
      <span class="badge {{sevClass .Severity}}">{{.Category}}{{if .Sub}} / {{.Sub}}{{end}} · {{.Method}}{{if .Votes}} · {{.Votes}}/{{.Samples}}{{end}}</span>
      {{if .Reason}}{{.Reason}}{{end}}
      {{if .Evidence}}<br>「{{.Evidence}}」{{end}}
    </div>
    {{end}}
  </div>
  {{end}}

  <footer>由 roleeval 生成 · {{.Item.Summary.TotalFindings}} 条发现</footer>
</main></body></html>`
