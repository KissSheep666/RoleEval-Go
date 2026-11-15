// Package probe 实现「半自动记忆探针」：
// 系统生成一份可以直接照着输入的脚本，人执行后把抓包数据拿回来评分。
package probe

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"roleeval/internal/model"
)

// Seed 是要注入给角色的事实。
type Seed struct {
	Turn int    `json:"turn"`
	User string `json:"user"`
}

// Spec 是一份探针定义。
type Spec struct {
	CharacterID int64    `json:"character_id"`
	Seeds       []Seed   `json:"seeds"`
	DelayTurns  int      `json:"delay_turns"`
	Questions   []string `json:"questions"`
	Expected    []string `json:"expected"`
}

// Load 读取探针定义。
func Load(path string) (Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Spec{}, err
	}
	var s Spec
	if err := json.Unmarshal(raw, &s); err != nil {
		return Spec{}, fmt.Errorf("探针文件不是合法 JSON: %w", err)
	}
	return s, nil
}

// Gen 生成一份可以照着操作的 Markdown 脚本。
func Gen(s Spec) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# 记忆探针 · 角色 %d\n\n", s.CharacterID))
	b.WriteString("按顺序执行。每一步都直接输入引号里的内容。\n\n")

	turn := 1
	for _, seed := range s.Seeds {
		if seed.Turn > turn {
			b.WriteString(fmt.Sprintf("## 第 %d 轮\n\n%s\n\n", turn, seed.User))
			turn = seed.Turn
		}
		b.WriteString(fmt.Sprintf("## 第 %d 轮 · 注入事实\n\n> %s\n\n", turn, seed.User))
		turn++
	}

	if s.DelayTurns > 0 {
		b.WriteString(fmt.Sprintf("## 第 %d–%d 轮 · 自由聊天\n\n", turn, turn+s.DelayTurns-1))
		b.WriteString("随便聊点别的（天气、训练、吃饭都行），目的是把注入的事实挤到上下文之外。\n\n")
		turn += s.DelayTurns
	}

	for i, q := range s.Questions {
		b.WriteString(fmt.Sprintf("## 第 %d 轮 · 提问 %d\n\n> %s\n\n", turn, i+1, q))
		turn++
	}

	b.WriteString("## 结束后\n\n")
	b.WriteString("点「导出 JSON」，把文件交给 roleeval 的 `probe score` 命令评分。\n\n")
	b.WriteString("期望召回：\n")
	for _, e := range s.Expected {
		b.WriteString(fmt.Sprintf("- %s\n", e))
	}
	return b.String()
}

var roomRe = regexp.MustCompile(`[0-9一二三四五六七八九十百]+号房`)

// Score 用抓包结果给探针打分。
func Score(s Spec, conv model.Conversation) []model.Finding {
	if len(s.Questions) == 0 || len(s.Expected) == 0 {
		return nil
	}

	q := s.Questions[0]
	qi := -1
	for _, t := range conv.Turns {
		if t.IsUser() && strings.Contains(t.UserText(), q) {
			qi = t.Seq
		}
	}
	if qi < 0 {
		return []model.Finding{{
			Category: "forgot_story", Sub: "probe_not_found", Severity: 1,
			Evidence: q, Reason: "抓包结果里没有找到探针问题，可能没有按脚本执行",
			Method: "probe", Confidence: 0.5,
		}}
	}

	var b strings.Builder
	for _, t := range conv.Turns {
		if t.Seq > qi && !t.IsUser() {
			b.WriteString(t.AssistantText())
			b.WriteString("\n")
		}
	}
	answer := b.String()

	var out []model.Finding
	for _, want := range s.Expected {
		if strings.Contains(answer, want) {
			continue
		}
		// 房间号这类可枚举事实：答成别的号，比“忘了”更严重
		if strings.Contains(want, "号房") {
			if got := roomRe.FindString(answer); got != "" && got != want {
				out = append(out, model.Finding{
					Category: "forgot_story", Sub: "facts_logic_wrong", Severity: 3,
					Evidence: clipStr(answer, 100),
					Reason:   fmt.Sprintf("期望「%s」，回答里出现「%s」——属于编造", want, got),
					Method:   "probe", Confidence: 0.9,
				})
				continue
			}
		}
		out = append(out, model.Finding{
			Category: "forgot_story", Sub: "forgot_our_history", Severity: 2,
			Evidence: clipStr(answer, 100),
			Reason:   fmt.Sprintf("未召回注入的事实「%s」", want),
			Method:   "probe", Confidence: 0.75,
		})
	}
	return out
}

func clipStr(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
