package judge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"roleeval/internal/model"
)

// Config 是 LLM judge 的配置。换模型只改这里，评测逻辑不动。
type Config struct {
	Provider string // openai | anthropic
	BaseURL  string // 如 https://api.deepseek.com/v1 或 https://api.anthropic.com
	APIKey   string
	Model    string
	MaxTurns int           // 送进 judge 的最大轮数
	Timeout  time.Duration // 单次请求超时
	Samples  int           // 多次采样取多数，默认 3

	// Profile 非空时，judge 会把它当成人设基线来判 OOC。
	Profile *model.PersonaProfile
}

// FromEnv 从环境变量读取配置。
func FromEnv() Config {
	c := Config{
		Provider: os.Getenv("ROLEEVAL_PROVIDER"),
		BaseURL:  os.Getenv("ROLEEVAL_BASE_URL"),
		APIKey:   os.Getenv("ROLEEVAL_API_KEY"),
		Model:    os.Getenv("ROLEEVAL_MODEL"),
		MaxTurns: 12,
		Timeout:  90 * time.Second,
		Samples:  3,
	}
	if v := os.Getenv("ROLEEVAL_JUDGE_SAMPLES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 9 {
			c.Samples = n
		}
	}
	if c.Provider == "" {
		c.Provider = "openai"
	}
	if c.BaseURL == "" {
		if c.Provider == "anthropic" {
			c.BaseURL = "https://api.anthropic.com"
		} else {
			c.BaseURL = "https://api.openai.com/v1"
		}
	}
	if c.Model == "" {
		if c.Provider == "anthropic" {
			c.Model = "claude-sonnet-5"
		} else {
			c.Model = "gpt-4o-mini"
		}
	}
	return c
}

// Ready 判断配置是否足以发起请求。
func (c Config) Ready() bool { return c.APIKey != "" && c.Model != "" }

// judgeOnce 跑一次采样。
func (c Config) judgeOnce(ctx context.Context, conv model.Conversation) ([]model.Finding, error) {
	system := buildSystemPrompt(c.Profile)
	user := buildTranscript(conv, c.MaxTurns)

	text, err := c.chat(ctx, system, user)
	if err != nil {
		return nil, err
	}

	return parseFindings(text)
}

/* -------------------------------- prompt -------------------------------- */

func buildSystemPrompt(p *model.PersonaProfile) string {
	var b strings.Builder
	b.WriteString("你是 AI 角色扮演对话的质量评测员。\n")
	b.WriteString("只报告有明确证据的问题；没有问题就返回空数组。不要臆测。\n\n")
	b.WriteString("输出必须是 JSON，格式：\n")
	b.WriteString(`{"findings":[{"category":"分类key","sub":"子类key","severity":1,"turn":3,"evidence":"原文片段","reason":"简短原因"}]}`)
	b.WriteString("\n\nseverity：1 轻微、2 明显、3 破坏体验。\n")
	b.WriteString("turn 是出问题的轮次编号；evidence 必须逐字引用原文，不得改写。\n\n")
	b.WriteString("可选分类：\n")
	for _, cat := range LLMCategories {
		b.WriteString(fmt.Sprintf("- %s：%s\n  子类：%s\n", cat.Key, cat.Definition, strings.Join(cat.Subs, ", ")))
	}
	b.WriteString("\n注意：不要报告已经由规则覆盖的简单问题（长度、乱码、复读），只判语义层面的问题。\n")
	return b.String()
}

func buildTranscript(conv model.Conversation, maxTurns int) string {
	turns := conv.Turns
	if maxTurns > 0 && len(turns) > maxTurns {
		// 保留开场（角色设定往往在这里）+ 最近的若干轮
		head := turns[:1]
		tail := turns[len(turns)-(maxTurns-1):]
		turns = append(append([]model.Turn{}, head...), tail...)
	}

	var b strings.Builder
	b.WriteString("以下是待评测的对话（按时间顺序）：\n\n")
	for _, t := range turns {
		who := "角色"
		text := t.AssistantText()
		if t.IsUser() {
			who = "用户"
			text = t.UserText()
		}
		text = truncateRunes(strings.TrimSpace(text), 600)
		if text == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("### 第 %d 轮 · %s\n%s\n\n", t.Seq, who, text))
	}
	b.WriteString("请输出 JSON。")
	return b.String()
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

/* -------------------------------- HTTP ---------------------------------- */

func (c Config) chat(ctx context.Context, system, user string) (string, error) {
	if c.Provider == "anthropic" {
		return c.callAnthropic(ctx, system, user)
	}
	return c.callOpenAI(ctx, system, user)
}

func (c Config) httpClient() *http.Client {
	to := c.Timeout
	if to <= 0 {
		to = 90 * time.Second
	}
	return &http.Client{Timeout: to}
}

func (c Config) callOpenAI(ctx context.Context, system, user string) (string, error) {
	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"

	body := map[string]any{
		"model":       c.Model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	text, status, err := c.postJSON(ctx, url, map[string]string{
		"Authorization": "Bearer " + c.APIKey,
	}, body)
	if err != nil && status == http.StatusBadRequest && strings.Contains(text, "response_format") {
		// 有些兼容接口不支持 response_format，去掉重试
		delete(body, "response_format")
		text, _, err = c.postJSON(ctx, url, map[string]string{
			"Authorization": "Bearer " + c.APIKey,
		}, body)
	}
	if err != nil {
		return "", err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w; body=%s", err, clipStr(text, 300))
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("响应没有 choices: %s", clipStr(text, 300))
	}
	return resp.Choices[0].Message.Content, nil
}

func (c Config) callAnthropic(ctx context.Context, system, user string) (string, error) {
	url := strings.TrimRight(c.BaseURL, "/") + "/v1/messages"

	body := map[string]any{
		"model":       c.Model,
		"max_tokens":  2000,
		"temperature": 0,
		"system":      system,
		"messages": []map[string]string{
			{"role": "user", "content": user},
		},
	}
	text, _, err := c.postJSON(ctx, url, map[string]string{
		"x-api-key":         c.APIKey,
		"anthropic-version": "2023-06-01",
	}, body)
	if err != nil {
		return "", err
	}

	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w; body=%s", err, clipStr(text, 300))
	}
	var b strings.Builder
	for _, c := range resp.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}
	return b.String(), nil
}

func (c Config) postJSON(ctx context.Context, url string, headers map[string]string, body map[string]any) (string, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return "", 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return string(data), resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, clipStr(string(data), 300))
	}
	return string(data), resp.StatusCode, nil
}

/* ------------------------------- parsing -------------------------------- */

var fenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")

type rawFinding struct {
	Category string  `json:"category"`
	Sub      string  `json:"sub"`
	Severity int     `json:"severity"`
	Turn     int     `json:"turn"`
	Evidence string  `json:"evidence"`
	Reason   string  `json:"reason"`
	Score    float64 `json:"confidence"`
}

func parseFindings(text string) ([]model.Finding, error) {
	text = strings.TrimSpace(text)
	if m := fenceRe.FindStringSubmatch(text); m != nil {
		text = strings.TrimSpace(m[1])
	}
	// 容错：取第一个 { 到最后一个 }
	if i := strings.Index(text, "{"); i >= 0 {
		if j := strings.LastIndex(text, "}"); j > i {
			text = text[i : j+1]
		}
	}

	var payload struct {
		Findings []rawFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil, fmt.Errorf("judge 输出不是合法 JSON: %w; 原文=%s", err, clipStr(text, 300))
	}

	var out []model.Finding
	for _, f := range payload.Findings {
		if _, ok := Find(f.Category); !ok {
			continue // 模型编了不存在的分类，直接丢
		}
		sev := f.Severity
		if sev < 1 {
			sev = 1
		}
		if sev > 3 {
			sev = 3
		}
		conf := f.Score
		if conf == 0 {
			conf = 0.7
		}
		out = append(out, model.Finding{
			Category:   f.Category,
			Sub:        f.Sub,
			Severity:   sev,
			Turn:       f.Turn,
			Evidence:   strings.TrimSpace(f.Evidence),
			Reason:     strings.TrimSpace(f.Reason),
			Method:     "llm",
			Confidence: conf,
		})
	}
	return out, nil
}

func clipStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

/* ---------------------------- stable judging ----------------------------- */

// Outcome 是多次采样后的稳定判定。
type Outcome struct {
	Findings  []model.Finding
	Samples   int     // 实际成功的采样次数
	Agreement float64 // 保留判定的平均命中率
	Errors    int     // 失败的采样次数
}

// JudgeStable 跑 N 次采样，取多数。
// 单次失败不会让整体失败；全部失败才报错。
func (c Config) JudgeStable(ctx context.Context, conv model.Conversation) (Outcome, error) {
	n := c.Samples
	if n <= 0 {
		n = 1
	}

	var runs [][]model.Finding
	errors := 0
	for i := 0; i < n; i++ {
		fs, err := c.judgeOnce(ctx, conv)
		if err != nil {
			errors++
			continue
		}
		runs = append(runs, fs)
	}
	if len(runs) == 0 {
		return Outcome{}, fmt.Errorf("%d 次采样全部失败", n)
	}

	out := aggregate(runs)
	out.Errors = errors
	return out, nil
}

// aggregate 按 (category, sub, turn) 做多数投票。
// 阈值：3 次采样需要 ≥2 次命中；2 次需要 2 次；1 次需要 1 次。
func aggregate(runs [][]model.Finding) Outcome {
	type bucket struct {
		f     model.Finding
		votes int
	}
	index := map[string]int{}
	var buckets []bucket

	for _, fs := range runs {
		seen := map[string]bool{}
		for _, f := range fs {
			k := f.Category + "|" + f.Sub + "|" + strconv.Itoa(f.Turn)
			if seen[k] {
				continue
			}
			seen[k] = true

			if i, ok := index[k]; ok {
				buckets[i].votes++
				if f.Severity > buckets[i].f.Severity {
					buckets[i].f.Severity = f.Severity
				}
			} else {
				index[k] = len(buckets)
				buckets = append(buckets, bucket{f: f, votes: 1})
			}
		}
	}

	need := (len(runs) + 2) / 2 // 3→2, 2→2, 1→1
	out := Outcome{Samples: len(runs)}

	var sum float64
	for _, b := range buckets {
		if b.votes < need {
			continue
		}
		f := b.f
		f.Votes = b.votes
		f.Samples = len(runs)
		f.Confidence = float64(b.votes) / float64(len(runs))
		f.Method = "llm"
		out.Findings = append(out.Findings, f)
		sum += f.Confidence
	}

	if len(out.Findings) == 0 {
		out.Agreement = 1 // 所有采样都认为没问题
	} else {
		out.Agreement = sum / float64(len(out.Findings))
	}
	return out
}
