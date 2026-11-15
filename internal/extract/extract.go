// Package extract 用规则从对话里抽取可判定的问题。
//
// 设计原则：
//   - 规则先跑，覆盖那些「机器一定能判」的问题；
//   - 每条 Finding 必须带 evidence（原文），否则报告不可读；
//   - 判不了的留给 judge 包的 LLM。
package extract

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"roleeval/internal/model"
)

// Fact 是从用户发言里抽出来、后面可以回查的事实。
type Fact struct {
	Turn  int    `json:"turn"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Run 跑完整的规则抽取，返回发现与事实表。
func Run(c model.Conversation) ([]model.Finding, []Fact) {
	var findings []model.Finding
	var facts []Fact

	for _, t := range c.Turns {
		if t.IsUser() {
			facts = append(facts, extractFacts(t)...)
			continue
		}
		findings = append(findings, checkFormat(t)...)
		findings = append(findings, checkPerson(t)...)
	}

	findings = append(findings, checkLengths(c)...)
	findings = append(findings, checkRepeats(c)...)
	findings = append(findings, checkAddress(c)...)

	return dedupe(findings), facts
}

/* ------------------------------ format_broken ------------------------------ */

var jsonLeak = regexp.MustCompile(`\{\s*"|\[\s*\{|":\s*"`)

func checkFormat(t model.Turn) []model.Finding {
	var out []model.Finding
	text := strings.TrimSpace(t.AssistantText())

	if text == "" {
		return []model.Finding{{
			Category: "format_broken", Sub: "empty_reply_or_refusal",
			Severity: 3, Turn: t.Seq, Evidence: "(空)", Reason: "角色轮次没有任何输出",
			Method: "rule", Confidence: 1,
		}}
	}

	for _, s := range t.Sentences {
		if s.Role == model.RoleImage || s.Kind == "image" || s.Content == "" {
			continue
		}
		if strings.ContainsRune(s.Content, '�') || hasControl(s.Content) {
			out = append(out, model.Finding{
				Category: "format_broken", Sub: "garbled_or_leaked",
				Severity: 2, Turn: t.Seq, SentenceID: s.ID, Evidence: clip(s.Content, 80),
				Reason: "包含替换字符或控制字符", Method: "rule", Confidence: 0.9,
			})
		}
		if jsonLeak.MatchString(s.Content) {
			out = append(out, model.Finding{
				Category: "format_broken", Sub: "garbled_or_leaked",
				Severity: 3, Turn: t.Seq, SentenceID: s.ID, Evidence: clip(s.Content, 80),
				Reason: "疑似 JSON 残留泄漏到正文", Method: "rule", Confidence: 0.95,
			})
		}
		if sentenceCutOff(s) {
			out = append(out, model.Finding{
				Category: "format_broken", Sub: "cut_off",
				Severity: 2, Turn: t.Seq, SentenceID: s.ID, Evidence: tail(s.Content, 40),
				Reason: "单句没有结束标记，疑似截断", Method: "rule", Confidence: 0.7,
			})
		}
	}

	return out
}

// sentenceCutOff 按句子判断截断：
//   - 旁白以 * 包裹，开头有 * 结尾没有 = 截断
//   - 台词以「」包裹，开头有「结尾没有」= 截断
//   - 其他：长度够长却没有结束标点
func sentenceCutOff(s model.Sentence) bool {
	c := strings.TrimSpace(s.Content)
	if c == "" {
		return false
	}
	if s.Kind == "narration" || strings.HasPrefix(c, "*") {
		return strings.HasPrefix(c, "*") && !strings.HasSuffix(c, "*")
	}
	if strings.HasPrefix(c, "「") {
		return !strings.HasSuffix(c, "」")
	}
	if utf8.RuneCountInString(c) < 40 {
		return false
	}
	last, _ := utf8.DecodeLastRuneInString(c)
	for _, ok := range terminals {
		if last == ok {
			return false
		}
	}
	return true
}

func hasControl(s string) bool {
	for _, r := range s {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

var terminals = []rune{'。', '！', '？', '…', '」', '』', '”', '"', ')', '）', '.', '!', '?', '*', '~', '—'}


/* ------------------------- took_over_character ------------------------- */

// 角色替用户写心理 / 做决定 / 说话，这是 Popia 反馈分类里的「took_over_character」。
var mentalForUser = regexp.MustCompile(`你(感到|感觉到|想|决定|意识到|明白|知道|记得|觉得|希望|愿意|担心|害怕|喜欢|讨厌|后悔|犹豫)`)
var speakForUser = regexp.MustCompile(`你说[：:]?\s*[「"“]`)

func checkPerson(t model.Turn) []model.Finding {
	var out []model.Finding
	narration := t.NarrationText()

	if m := mentalForUser.FindString(narration); m != "" {
		out = append(out, model.Finding{
			Category: "took_over_character", Sub: "wrote_my_thoughts",
			Severity: 2, Turn: t.Seq, Evidence: clip(sentenceAround(narration, m), 80),
			Reason: "旁白替用户写了心理活动", Method: "rule", Confidence: 0.75,
		})
	}

	if m := speakForUser.FindString(narration); m != "" {
		out = append(out, model.Finding{
			Category: "took_over_character", Sub: "acting_for_me",
			Severity: 3, Turn: t.Seq, Evidence: clip(sentenceAround(narration, m), 80),
			Reason: "旁白替用户说了话", Method: "rule", Confidence: 0.85,
		})
	}

	return out
}

/* ---------------------------- writing_quality ---------------------------- */

func checkLengths(c model.Conversation) []model.Finding {
	var lengths []int
	for _, t := range c.Turns {
		if t.IsUser() {
			continue
		}
		n := utf8.RuneCountInString(strings.TrimSpace(t.AssistantText()))
		if n > 0 {
			lengths = append(lengths, n)
		}
	}
	if len(lengths) == 0 {
		return nil
	}

	med := median(lengths)
	var out []model.Finding

	for _, t := range c.Turns {
		if t.IsUser() {
			continue
		}
		n := utf8.RuneCountInString(strings.TrimSpace(t.AssistantText()))
		switch {
		case n >= 800:
			out = append(out, model.Finding{
				Category: "writing_quality", Sub: "turn_too_long", Severity: 3,
				Turn: t.Seq, Evidence: fmt.Sprintf("本轮 %d 字", n),
				Reason: "单轮输出接近或超过 800 字", Method: "rule", Confidence: 0.9,
			})
		case n >= 500:
			out = append(out, model.Finding{
				Category: "writing_quality", Sub: "turn_too_long", Severity: 2,
				Turn: t.Seq, Evidence: fmt.Sprintf("本轮 %d 字", n),
				Reason: "单轮输出超过 500 字", Method: "rule", Confidence: 0.8,
			})
		case n > 0 && n <= 10:
			out = append(out, model.Finding{
				Category: "writing_quality", Sub: "turn_too_short", Severity: 1,
				Turn: t.Seq, Evidence: fmt.Sprintf("本轮 %d 字", n),
				Reason: "单轮输出过短", Method: "rule", Confidence: 0.6,
			})
		case med >= 60 && float64(n) > 2.5*med && n > 150:
			out = append(out, model.Finding{
				Category: "writing_quality", Sub: "turn_too_long", Severity: 1,
				Turn: t.Seq, Evidence: fmt.Sprintf("本轮 %d 字，中位数 %.0f", n, med),
				Reason: "明显长于本会话中位数", Method: "rule", Confidence: 0.6,
			})
		}
	}
	return out
}

/* ------------------------------- repetition ------------------------------- */

func checkRepeats(c model.Conversation) []model.Finding {
	var out []model.Finding
	var prevUser string
	type past struct {
		seq  int
		text string
	}
	var history []past

	for _, t := range c.Turns {
		if t.IsUser() {
			prevUser = t.UserText()
			continue
		}
		text := strings.TrimSpace(t.AssistantText())
		if text == "" {
			continue
		}

		// 1) 复读用户刚说的话
		if utf8.RuneCountInString(prevUser) >= 30 && gramOverlap(prevUser, text, 4) >= 0.6 {
			out = append(out, model.Finding{
				Category: "repetition", Sub: "parrots_user_input", Severity: 2,
				Turn: t.Seq, Evidence: clip(text, 80),
				Reason: "大量复述用户刚才的输入", Method: "rule", Confidence: 0.7,
			})
		}

		// 2) 与最近几轮角色输出重复（不止上一轮）
		if utf8.RuneCountInString(text) >= 30 {
			cur := strings.Trim(text, "* \n")
			for i := len(history) - 1; i >= 0 && i >= len(history)-5; i-- {
				p := history[i]
				if utf8.RuneCountInString(p.text) < 30 {
					continue
				}
				prev := strings.Trim(p.text, "* \n")
				ov := gramOverlap(prev, cur, 8)
				if ov < 0.35 {
					ov = gramOverlap(cur, prev, 8) // 短的那段几乎全被包含时，也要算重复
				}
				if strings.Contains(prev, cur) || strings.Contains(cur, prev) || ov >= 0.35 {
					out = append(out, model.Finding{
						Category: "repetition", Sub: "repeated_phrases", Severity: 2,
						Turn: t.Seq, Evidence: clip(text, 80),
						Reason: fmt.Sprintf("与第 %d 轮输出高度重复", p.seq), Method: "rule", Confidence: 0.7,
					})
					break
				}
			}
		}

		history = append(history, past{seq: t.Seq, text: text})
	}
	return out
}

/* ---------------------------- forgot_story（规则部分） ---------------------------- */

var callMeRe = regexp.MustCompile(`(?:别|不要|不准|不许)叫我\s*([^\s，。！？,.!?]{1,8})`)
var newNameRe = regexp.MustCompile(`(?:叫我|喊我|称呼我)\s*([^\s，。！？,.!?]{1,8})`)

// checkAddress 处理「用户让角色改口」这种最硬的一致性检查。
func checkAddress(c model.Conversation) []model.Finding {
	type rule struct {
		turn    int
		oldName string
		newName string
	}
	var rules []rule

	for _, t := range c.Turns {
		if !t.IsUser() {
			continue
		}
		text := t.UserText()
		old := ""
		if m := callMeRe.FindStringSubmatch(text); m != nil {
			old = m[1]
		}
		newName := ""
		if m := newNameRe.FindStringSubmatch(text); m != nil {
			newName = m[1]
		}
		if old != "" || newName != "" {
			rules = append(rules, rule{turn: t.Seq, oldName: old, newName: newName})
		}
	}
	if len(rules) == 0 {
		return nil
	}

	var out []model.Finding
	for _, r := range rules {
		if r.oldName == "" {
			continue
		}
		for _, t := range c.Turns {
			if t.Seq <= r.turn || t.IsUser() {
				continue
			}
			text := t.AssistantText()
			if strings.Contains(text, r.oldName) {
				out = append(out, model.Finding{
					Category: "forgot_story", Sub: "forgot_about_me", Severity: 2,
					Turn: t.Seq, Evidence: clip(sentenceAround(text, r.oldName), 80),
					Reason: fmt.Sprintf("第 %d 轮用户已要求不要叫「%s」，此处仍在用", r.turn, r.oldName),
					Method: "rule", Confidence: 0.8,
				})
				break
			}
		}
	}
	return out
}

/* -------------------------------- facts ---------------------------------- */

var factPatterns = []struct {
	key string
	re  *regexp.Regexp
}{
	{"name", regexp.MustCompile(`我叫\s*([^\s，。！？,.!?]{1,10})`)},
	{"residence", regexp.MustCompile(`(?:住在|我住)\s*([^\s，。！？,.!?]{1,12})`)},
	{"room", regexp.MustCompile(`([0-9一二三四五六七八九十百]+号房|[0-9]+号房)`)},
	{"tomorrow", regexp.MustCompile(`我明天(?:要|去|得)\s*([^\s，。！？,.!?]{1,20})`)},
}

func extractFacts(t model.Turn) []Fact {
	text := t.UserText()
	var out []Fact
	for _, p := range factPatterns {
		for _, m := range p.re.FindAllStringSubmatch(text, -1) {
			if len(m) < 2 || strings.TrimSpace(m[1]) == "" {
				continue
			}
			out = append(out, Fact{Turn: t.Seq, Key: p.key, Value: strings.TrimSpace(m[1])})
		}
	}
	return out
}

/* -------------------------------- helpers -------------------------------- */

func median(xs []int) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	n := len(s)
	if n%2 == 1 {
		return float64(s[n/2])
	}
	return float64(s[n/2-1]+s[n/2]) / 2
}

// gramOverlap 返回 a 的 n-gram 有多大比例出现在 b 里。
func gramOverlap(a, b string, n int) float64 {
	ga := grams(a, n)
	if len(ga) == 0 {
		return 0
	}
	gb := grams(b, n)
	hit := 0
	for g := range ga {
		if _, ok := gb[g]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(ga))
}

func grams(s string, n int) map[string]struct{} {
	r := []rune(s)
	out := make(map[string]struct{}, len(r))
	if len(r) < n {
		return out
	}
	for i := 0; i+n <= len(r); i++ {
		out[string(r[i:i+n])] = struct{}{}
	}
	return out
}

func sentenceAround(text, needle string) string {
	i := strings.Index(text, needle)
	if i < 0 {
		return text
	}
	start := strings.LastIndexAny(text[:i], "。！？\n")
	start++
	end := strings.IndexAny(text[i:], "。！？\n")
	if end < 0 {
		end = len(text) - i
	}
	return text[start : i+end]
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + "…"
}

func tail(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	return "…" + string(r[len(r)-n:])
}

func dedupe(fs []model.Finding) []model.Finding {
	seen := map[string]bool{}
	out := fs[:0]
	for _, f := range fs {
		k := fmt.Sprintf("%s|%s|%d|%s", f.Category, f.Sub, f.Turn, f.Evidence)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}
